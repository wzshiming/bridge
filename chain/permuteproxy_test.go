package chain

import (
	"context"
	"crypto/tls"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/wzshiming/permuteproxy"
	_ "github.com/wzshiming/permuteproxy/protocols/all"

	"github.com/wzshiming/bridge"
)

// recorder notes each dial and fails it, so a hop is seen asking for its endpoint without any I/O.
type recorder struct {
	mu    sync.Mutex
	calls []string
}

var errRecorded = errors.New("recorded")

func (r *recorder) DialContext(_ context.Context, network, address string) (net.Conn, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.calls = append(r.calls, network+" "+address)
	return nil, errRecorded
}

func TestDefaultPort(t *testing.T) {
	for uri, want := range map[string]string{
		"http://proxy.test":               "tcp proxy.test:80",
		"https://proxy.test":              "tcp proxy.test:443",
		"socks4://proxy.test":             "tcp proxy.test:1080",
		"socks5://u:p@proxy.test":         "tcp proxy.test:1080",
		"socks5h://[::1]":                 "tcp [::1]:1080",
		"ssh://u@proxy.test?allow_exec=1": "tcp proxy.test:22",
		"ss://aes-256-cfb:p@proxy.test":   "tcp proxy.test:8379",
		"socks5://proxy.test:1":           "tcp proxy.test:1",
		"socks5+snappy://proxy.test":      "tcp proxy.test:1080",
	} {
		t.Run(uri, func(t *testing.T) {
			base := &recorder{}
			d, err := permuteProxy(t.Context(), base, uri)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.DialContext(t.Context(), "tcp", "192.0.2.1:80"); !errors.Is(err, errRecorded) {
				t.Fatalf("DialContext = %v, want the base's %v", err, errRecorded)
			}
			if want := []string{want}; !slices.Equal(base.calls, want) {
				t.Fatalf("base calls = %q, want %q", base.calls, want)
			}
		})
	}
}

func TestRejectsHopsThatAreNoClient(t *testing.T) {
	for _, uri := range []string{"unknown://proxy.test:1", "unknown:", "socks5:", "socks5://proxy.test:1?unknown=1", "tls+socks5://proxy.test:1", "tls:", "snappy:", "tls://proxy.test:443", "tls:proxy.test?insecure=true"} {
		if _, err := permuteProxy(t.Context(), &recorder{}, uri); err == nil {
			t.Errorf("%s: accepted", uri)
		}
	}
}

// TestServerNameDefaultsToTheProxyHost checks the tls layer of a hop verifies the host the hop names.
func TestServerNameDefaultsToTheProxyHost(t *testing.T) {
	for uri, want := range map[string]string{
		"https://proxy.test?tls_skip_verify=true":                            "proxy.test",
		"socks5+tls://proxy.test:1080?tls_skip_verify=true":                  "proxy.test",
		"https://proxy.test?tls_skip_verify=true&tls_server_name=other.test": "other.test",
	} {
		t.Run(uri, func(t *testing.T) {
			sni := make(chan string, 1)
			srv := httptest.NewUnstartedServer(http.NotFoundHandler())
			srv.TLS = &tls.Config{GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
				sni <- h.ServerName
				return nil, nil
			}}
			srv.StartTLS()
			defer srv.Close()
			toServer := bridge.DialFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
			})
			d, err := permuteProxy(t.Context(), toServer, uri)
			if err != nil {
				t.Fatal(err)
			}
			// The dial ends with the test: the server speaks no proxy protocol, so only its handshake matters.
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			go func() {
				if conn, err := d.DialContext(ctx, "tcp", "192.0.2.1:80"); err == nil {
					conn.Close()
				}
			}()
			select {
			case got := <-sni:
				if got != want {
					t.Fatalf("SNI = %q, want %q", got, want)
				}
			case <-time.After(5 * time.Second):
				t.Fatal("server received no SNI")
			}
		})
	}
}

// commands is a base whose cmd network records each command line and fails it.
type commands struct {
	recorder
}

func (c *commands) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	_, err := c.DialContext(ctx, network, address)
	return nil, err
}

func TestCommandHops(t *testing.T) {
	for _, tt := range []struct{ uri, network, address, dial, listen string }{
		{"cmd:ssh example nc %h %p", "tcp", "host:1", "ssh example nc host 1", "ssh example nc host 1"},
		{"command:nc -X %n %h %p", "tcp4", "host:1", "nc -X tcp4 host 1", "nc -X tcp4 host 1"},
		{"cmd:sh -c 'exec nc %h %p' 100%%", "tcp", "host:1", "sh -c 'exec nc host 1' 100%", "sh -c 'exec nc host 1' 100%"},
		{"nc:", "tcp", "host:1", "nc host 1", "nc -l host 1"},
		{"netcat:", "tcp4", "host:4", "nc -4 host 4", "nc -4 -l host 4"},
		{"nc:ssh example", "tcp6", "host:6", "ssh example nc -6 host 6", "ssh example nc -6 -l host 6"},
		{"nc:ssh example", "unix", "/run/nc.sock", "ssh example nc -U /run/nc.sock", "ssh example nc -Ul /run/nc.sock"},
	} {
		t.Run(tt.uri+" "+tt.network, func(t *testing.T) {
			base := &commands{}
			d, err := permuteProxy(t.Context(), base, tt.uri)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := d.DialContext(t.Context(), tt.network, tt.address); !errors.Is(err, errRecorded) {
				t.Fatalf("DialContext = %v, want the base's %v", err, errRecorded)
			}
			if _, err := d.(bridge.ListenConfig).Listen(t.Context(), tt.network, tt.address); !errors.Is(err, errRecorded) {
				t.Fatalf("Listen = %v, want the base's %v", err, errRecorded)
			}
			if want := []string{"cmd " + tt.dial, "cmd " + tt.listen}; !slices.Equal(base.calls, want) {
				t.Fatalf("base calls = %q, want %q", base.calls, want)
			}
		})
	}
}

func TestCommandHopRejectsBadCommands(t *testing.T) {
	for _, uri := range []string{"cmd:", "cmd:nc %x"} {
		if _, err := permuteProxy(t.Context(), &commands{}, uri); err == nil {
			t.Errorf("%s: accepted", uri)
		}
	}
	for _, uri := range []string{"cmd:nc %h %p", "nc:"} {
		d, err := permuteProxy(t.Context(), &recorder{}, uri)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := d.DialContext(t.Context(), "tcp", "a b:1"); err == nil || errors.Is(err, errRecorded) {
			t.Errorf("%s: dial of a host no command line carries = %v, want it refused before the command runs", uri, err)
		}
		if _, err := d.(bridge.ListenConfig).Listen(t.Context(), "tcp", "host:1"); !errors.Is(err, permuteproxy.ErrUnsupported) {
			t.Errorf("%s: Listen over a base that cannot listen = %v, want %v", uri, err, permuteproxy.ErrUnsupported)
		}
	}
}

func TestLocalRunsCommands(t *testing.T) {
	conn, err := Local.DialContext(t.Context(), "cmd", "echo ping")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	buf := make([]byte, 5)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "ping\n" {
		t.Fatalf("read %q, %v; want the command's output", buf, err)
	}
	if _, err := Local.DialContext(t.Context(), "unknown", "x"); err == nil {
		t.Error("dialed an unknown network")
	}
}

// TestCommandHopOverSSH runs a cmd: hop on the far side of an ssh hop, which executes it.
func TestCommandHopOverSSH(t *testing.T) {
	if _, err := exec.LookPath("nc"); err != nil {
		t.Skipf("nc unavailable: %v", err)
	}
	free, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := free.Addr().String()
	free.Close()
	srv, err := permuteproxy.NewServer("ssh://u:p@" + address + "?allow_exec=true")
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Close()
	go srv.Run(t.Context())

	echo, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		conn, err := echo.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		io.Copy(conn, conn)
	}()

	ssh, err := permuteProxy(t.Context(), nil, "ssh://u:p@"+address)
	if err != nil {
		t.Fatal(err)
	}
	d, err := permuteProxy(t.Context(), ssh, "cmd:nc %h %p")
	if err != nil {
		t.Fatal(err)
	}
	var conn net.Conn
	for range 50 {
		if conn, err = d.DialContext(t.Context(), "tcp", echo.Addr().String()); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 4)
	if _, err := io.ReadFull(conn, buf); err != nil || string(buf) != "ping" {
		t.Fatalf("read %q, %v; want %q echoed", buf, err, "ping")
	}
}
