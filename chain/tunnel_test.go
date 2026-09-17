package chain

import (
	"context"
	"errors"
	"io"
	"net"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/dump"
)

func TestStepCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client, raw := net.Pipe()
	conn, target := net.Pipe()
	for _, endpoint := range []net.Conn{client, raw, conn, target} {
		endpoint.SetDeadline(time.Now().Add(5 * time.Second))
		t.Cleanup(func() { endpoint.Close() })
	}
	done := make(chan error, 1)
	dialer := bridge.DialFunc(func(got context.Context, network, address string) (net.Conn, error) {
		if got != ctx {
			t.Error("dialer did not receive caller context")
		}
		return conn, nil
	})
	go func() { done <- step(ctx, dialer, raw, []string{"tcp://target:80"}) }()
	t.Cleanup(func() {
		raw.Close()
		conn.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("step did not finish after cleanup")
		}
	})
	written := make(chan error, 1)
	go func() {
		_, err := client.Write([]byte("x"))
		written <- err
	}()
	var payload [1]byte
	if _, err := io.ReadFull(target, payload[:]); err != nil {
		t.Fatal(err)
	}
	if err := <-written; err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		done <- err
	case <-time.After(time.Second):
		t.Fatal("step ignored caller cancellation after dialing")
	}
}

func TestStepStdioRemoteEOF(t *testing.T) {
	testStepStdio(t, false)
}

func TestStepStdioCancellation(t *testing.T) {
	testStepStdio(t, true)
}

func testStepStdio(t *testing.T, cancelContext bool) {
	t.Helper()
	for _, wrapped := range []bool{false, true} {
		name := "plain"
		if wrapped {
			name = "dump"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			input, inputWriter := io.Pipe()
			defer inputWriter.Close()
			reader := &tunnelReader{ReadCloser: io.NopCloser(input), started: make(chan struct{}), done: make(chan struct{})}
			var raw io.ReadWriteCloser = struct {
				io.ReadCloser
				io.Writer
			}{
				ReadCloser: reader,
				Writer:     io.Discard,
			}
			if wrapped {
				raw = dump.NewDumpReadWriteCloser(raw, true, "STDIO", "target")
			}
			conn, target := net.Pipe()
			defer target.Close()
			defer conn.Close()
			conn.SetDeadline(time.Now().Add(5 * time.Second))
			dialed := make(chan struct{})
			dialer := bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
				close(dialed)
				return conn, nil
			})
			done := make(chan error, 1)
			go func() { done <- step(ctx, dialer, raw, []string{"tcp://target:80"}) }()
			t.Cleanup(func() {
				input.Close()
				conn.Close()
				select {
				case <-done:
				case <-time.After(time.Second):
					t.Error("step did not finish after releasing stdin")
				}
				select {
				case <-reader.done:
				case <-time.After(time.Second):
					t.Error("stdin read did not finish after cleanup")
				}
			})
			select {
			case <-dialed:
			case <-time.After(time.Second):
				t.Fatal("step did not dial")
			}
			select {
			case <-reader.started:
			case <-time.After(time.Second):
				t.Fatal("stdin read did not start")
			}
			if cancelContext {
				cancel()
			} else {
				target.Close()
			}
			select {
			case err := <-done:
				done <- err
				var want error
				if cancelContext {
					want = context.Canceled
				}
				if !errors.Is(err, want) {
					t.Fatalf("step error = %v, want %v", err, want)
				}
			case <-time.After(time.Second):
				t.Fatal("step hung on stdin: io.NopCloser.Close cannot interrupt Read")
			}
		})
	}
}

type tunnelReader struct {
	io.ReadCloser
	started chan struct{}
	done    chan struct{}
}

func (reader *tunnelReader) Read(buffer []byte) (int, error) {
	close(reader.started)
	defer close(reader.done)
	return reader.ReadCloser.Read(buffer)
}

func tunnelTCPPair(t *testing.T) (*net.TCPConn, *net.TCPConn) {
	t.Helper()
	listener, err := net.ListenTCP("tcp4", &net.TCPAddr{IP: net.IPv4(127, 0, 0, 1)})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	deadline := time.Now().Add(5 * time.Second)
	listener.SetDeadline(deadline)
	client, err := net.DialTimeout("tcp4", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { client.Close() })
	server, err := listener.AcceptTCP()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { server.Close() })
	client.SetDeadline(deadline)
	server.SetDeadline(deadline)
	return client.(*net.TCPConn), server
}

func startTunnel(t *testing.T, first, second io.ReadWriteCloser, run func() error) <-chan struct{} {
	t.Helper()
	done := make(chan struct{})
	go func() {
		defer close(done)
		if err := run(); err != nil {
			t.Errorf("tunnel: %v", err)
		}
	}()
	t.Cleanup(func() {
		first.Close()
		second.Close()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Error("tunnel did not finish after cleanup")
		}
	})
	return done
}

func TestTunnelHalfClose(t *testing.T) {
	for _, mode := range []string{"first", "second", "step"} {
		t.Run(mode, func(t *testing.T) {
			client, first := tunnelTCPPair(t)
			second, target := tunnelTCPPair(t)
			run := func() error { return Tunnel(context.Background(), first, second) }
			if mode == "second" {
				client, target = target, client
			} else if mode == "step" {
				dialer := bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
					return second, nil
				})
				run = func() error { return step(context.Background(), dialer, first, []string{"tcp://target:80"}) }
			}
			done := startTunnel(t, first, second, run)
			if _, err := io.WriteString(client, "request"); err != nil {
				t.Fatal(err)
			}
			if err := client.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			request, err := io.ReadAll(target)
			if err != nil || string(request) != "request" {
				t.Fatalf("request = %q, error = %v", request, err)
			}
			select {
			case <-done:
				t.Fatal("tunnel returned before reverse response")
			default:
			}
			if _, err := io.WriteString(target, "response after EOF"); err != nil {
				t.Fatal(err)
			}
			if err := target.CloseWrite(); err != nil {
				t.Fatal(err)
			}
			response, err := io.ReadAll(client)
			if err != nil || string(response) != "response after EOF" {
				t.Fatalf("response = %q, error = %v", response, err)
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("tunnel did not finish after both EOFs")
			}
			for _, endpoint := range []*net.TCPConn{first, second} {
				if err := endpoint.SetDeadline(time.Now()); !errors.Is(err, net.ErrClosed) {
					t.Errorf("endpoint not closed: %v", err)
				}
			}
		})
	}
}

func TestTunnelPipeClosure(t *testing.T) {
	for _, cancelContext := range []bool{false, true} {
		name := "EOF"
		if cancelContext {
			name = "cancel"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			client, first := net.Pipe()
			second, target := net.Pipe()
			t.Cleanup(func() { client.Close(); target.Close() })
			done := startTunnel(t, first, second, func() error {
				_ = Tunnel(ctx, first, second)
				return nil
			})
			if cancelContext {
				cancel()
			} else {
				client.Close()
			}
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("tunnel did not unblock both copies")
			}
			for _, endpoint := range []net.Conn{first, second} {
				if _, err := endpoint.Write([]byte("x")); !errors.Is(err, io.ErrClosedPipe) {
					t.Errorf("endpoint not closed: %v", err)
				}
			}
		})
	}
}

type tunnelErrorStream struct {
	io.Reader
	readErr  error
	writeErr error
	closed   bool
}

func (stream *tunnelErrorStream) Read(buffer []byte) (int, error) {
	if stream.readErr != nil {
		return 0, stream.readErr
	}
	return stream.Reader.Read(buffer)
}

func (stream *tunnelErrorStream) Write(buffer []byte) (int, error) {
	return 0, stream.writeErr
}

func (stream *tunnelErrorStream) Close() error {
	stream.closed = true
	return nil
}

func TestTunnelCopyError(t *testing.T) {
	want := errors.New("copy failed")
	for _, mode := range []string{"read", "write"} {
		t.Run(mode, func(t *testing.T) {
			first := &tunnelErrorStream{Reader: strings.NewReader("")}
			second := &tunnelErrorStream{Reader: strings.NewReader("")}
			if mode == "read" {
				first.readErr = want
			} else {
				first.Reader = strings.NewReader("payload")
				second.writeErr = want
			}
			if err := Tunnel(context.Background(), first, second); !errors.Is(err, want) {
				t.Fatalf("error = %v, want %v", err, want)
			}
			if !first.closed || !second.closed {
				t.Fatal("tunnel did not close both endpoints")
			}
		})
	}
}

type tunnelDelayedConn struct {
	net.Conn
	release  <-chan struct{}
	readDone chan struct{}
}

func (conn *tunnelDelayedConn) Read(buffer []byte) (int, error) {
	count, err := conn.Conn.Read(buffer)
	<-conn.release
	close(conn.readDone)
	return count, err
}

func TestTunnelWaitsForCopies(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		client, first := net.Pipe()
		second, target := net.Pipe()
		defer target.Close()
		defer client.Close()
		release := make(chan struct{})
		delayed := &tunnelDelayedConn{Conn: second, release: release, readDone: make(chan struct{})}
		done := startTunnel(t, first, delayed, func() error {
			_ = Tunnel(context.Background(), first, delayed)
			return nil
		})
		defer close(release)
		synctest.Wait()
		client.Close()
		synctest.Wait()
		select {
		case <-done:
			t.Fatal("tunnel returned while a copy was still running")
		default:
		}
	})
}
