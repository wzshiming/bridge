package netutils

import (
	"context"
	"io"
	"net"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/commandproxy"
)

// fakeDial records one CommandDialContext call; conn and peer are nil while the dial is held.
type fakeDial struct {
	ctx  context.Context
	conn *probeConn
	peer net.Conn
}

// probeConn is the dialed end of a pipe; it signals its first Read and counts Closes.
type probeConn struct {
	net.Conn
	reading chan struct{}
	once    sync.Once
	closes  atomic.Int32
}

func (c *probeConn) Read(p []byte) (int, error) {
	c.once.Do(func() { close(c.reading) })
	return c.Conn.Read(p)
}

func (c *probeConn) Close() error {
	c.closes.Add(1)
	return c.Conn.Close()
}

// fakeCommand serves one net.Pipe per dial; with hold set, dials block until ctx ends.
type fakeCommand struct {
	hold  bool
	dials chan *fakeDial
}

func newFakeCommand(hold bool) *fakeCommand {
	return &fakeCommand{hold: hold, dials: make(chan *fakeDial, 8)}
}

func (f *fakeCommand) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	d := &fakeDial{ctx: ctx}
	if f.hold {
		f.dials <- d
		<-ctx.Done()
		return nil, ctx.Err()
	}
	c, s := net.Pipe()
	d.peer = c
	d.conn = &probeConn{Conn: s, reading: make(chan struct{})}
	f.dials <- d
	return d.conn, nil
}

func (f *fakeCommand) assertNoDial(t *testing.T) {
	t.Helper()
	select {
	case <-f.dials:
		t.Fatal("unexpected dial")
	default:
	}
}

func newCommandListener(t *testing.T, ctx context.Context, f *fakeCommand) net.Listener {
	t.Helper()
	l, err := NewCommandListener(ctx, f, NewNetAddr("local", "local"), NewNetAddr("cmd", "fake"), []string{"fake"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	return l
}

func TestCommandListenerProbeErrorClosesConn(t *testing.T) {
	f := newFakeCommand(false)
	l := newCommandListener(t, context.Background(), f)
	acc := acceptAsync(l)
	d := await(t, f.dials, "dial")
	d.peer.Close()
	if r := await(t, acc, "Accept after peer close"); r.conn != nil || r.err != io.EOF {
		t.Fatalf("Accept = %v, %v; want nil, io.EOF", r.conn, r.err)
	}
	if n := d.conn.closes.Load(); n != 1 {
		t.Fatalf("probe conn closes = %d; want 1", n)
	}
}

func TestCommandListenerCloseDuringDial(t *testing.T) {
	f := newFakeCommand(true)
	l := newCommandListener(t, context.Background(), f)
	acc := acceptAsync(l)
	d := await(t, f.dials, "dial")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if r := await(t, acc, "Accept closed during dial"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
	if d.ctx.Err() == nil {
		t.Fatal("dial ctx not canceled by Close")
	}
	if r := await(t, acceptAsync(l), "Accept on closed listener"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("closed listener Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
	f.assertNoDial(t)
}

func TestCommandListenerCloseDuringProbe(t *testing.T) {
	f := newFakeCommand(false)
	l := newCommandListener(t, context.Background(), f)
	acc := acceptAsync(l)
	d := await(t, f.dials, "dial")
	await(t, d.conn.reading, "probe read")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if r := await(t, acc, "Accept closed during probe"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
	assertPeerEOF(t, d.peer)
	if d.ctx.Err() == nil {
		t.Fatal("dial ctx not canceled by Close")
	}
}

func TestCommandListenerParentCancelDuringProbe(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f := newFakeCommand(false)
	l := newCommandListener(t, ctx, f)
	acc := acceptAsync(l)
	d := await(t, f.dials, "dial")
	await(t, d.conn.reading, "probe read")
	cancel()
	if r := await(t, acc, "Accept after parent cancel"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
	assertPeerEOF(t, d.peer)
	if r := await(t, acceptAsync(l), "Accept on canceled listener"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("canceled listener Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
}

func TestCommandListenerHandoffSurvivesClose(t *testing.T) {
	f := newFakeCommand(false)
	l := newCommandListener(t, context.Background(), f)
	acc := acceptAsync(l)
	d := await(t, f.dials, "dial")
	werr := make(chan error, 1)
	go func() {
		_, err := d.peer.Write([]byte("hi"))
		werr <- err
	}()
	r := await(t, acc, "Accept")
	if r.err != nil {
		t.Fatal(r.err)
	}
	buf := make([]byte, 2)
	if _, err := io.ReadFull(r.conn, buf); err != nil || string(buf) != "hi" {
		t.Fatalf("read = %q, %v; want %q, nil", buf, err, "hi")
	}
	if err := await(t, werr, "write"); err != nil {
		t.Fatal(err)
	}
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	// Established conns and their command ctx must outlive the listener.
	if d.ctx.Err() != nil || d.conn.closes.Load() != 0 {
		t.Fatalf("listener Close touched accepted conn: ctx err %v, closes %d", d.ctx.Err(), d.conn.closes.Load())
	}
	assertExchange(t, r.conn, d.peer, "pong")
	assertExchange(t, d.peer, r.conn, "ping")
	if err := r.conn.Close(); err != nil {
		t.Fatal(err)
	}
	if d.ctx.Err() == nil || d.conn.closes.Load() != 1 {
		t.Fatalf("conn Close: ctx err %v, closes %d; want canceled, 1", d.ctx.Err(), d.conn.closes.Load())
	}
	assertPeerEOF(t, d.peer)
	f.assertNoDial(t)
}

func TestCommandListenerConcurrentAcceptClose(t *testing.T) {
	f := newFakeCommand(true)
	l := newCommandListener(t, context.Background(), f)
	accs := []<-chan connResult{acceptAsync(l), acceptAsync(l), acceptAsync(l)}
	await(t, f.dials, "first dial")
	f.assertNoDial(t)
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	for i, acc := range accs {
		if r := await(t, acc, "concurrent Accept"); r.conn != nil || r.err != ErrClosedConn {
			t.Fatalf("Accept %d = %v, %v; want nil, ErrClosedConn", i, r.conn, r.err)
		}
	}
	f.assertNoDial(t)
}

func TestCommandListenerLocalCommandClose(t *testing.T) {
	if _, err := exec.LookPath("sleep"); err != nil {
		t.Skip(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	started := make(chan struct{}, 1)
	dial := bridge.CommandDialFunc(func(ctx context.Context, name string, args ...string) (net.Conn, error) {
		c, err := commandproxy.ProxyCommand(ctx, name, args...).Stdio()
		started <- struct{}{}
		return c, err
	})
	l, err := NewCommandListener(ctx, dial, NewNetAddr("local", "local"), NewNetAddr("cmd", "sleep"), []string{"sleep", "30"})
	if err != nil {
		t.Fatal(err)
	}
	acc := acceptAsync(l)
	await(t, started, "command start")
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	if r := await(t, acc, "Accept closed during command"); r.conn != nil || r.err != ErrClosedConn {
		t.Fatalf("Accept = %v, %v; want nil, ErrClosedConn", r.conn, r.err)
	}
}

func assertPeerEOF(t *testing.T, c net.Conn) {
	t.Helper()
	c.SetReadDeadline(time.Now().Add(virtualWait))
	if n, err := c.Read(make([]byte, 1)); n != 0 || err != io.EOF {
		t.Fatalf("peer read = %d, %v; want 0, io.EOF", n, err)
	}
}
