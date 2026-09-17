package tls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
)

// newServer serves the httptest self-signed certificate and reports each SNI it receives.
func newServer(t *testing.T) (*httptest.Server, <-chan string) {
	sni := make(chan string, 16)
	srv := httptest.NewUnstartedServer(http.NotFoundHandler())
	srv.TLS = &tls.Config{GetCertificate: func(h *tls.ClientHelloInfo) (*tls.Certificate, error) {
		sni <- h.ServerName
		return nil, nil
	}}
	srv.StartTLS()
	t.Cleanup(srv.Close)
	return srv, sni
}

// toServer ignores the logical target and connects to srv.
func toServer(srv *httptest.Server) bridge.Dialer {
	return bridge.DialFunc(func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, srv.Listener.Addr().String())
	})
}

func dial(t *testing.T, dialer bridge.Dialer, uri, target string) (net.Conn, error) {
	t.Helper()
	d, err := TLS(t.Context(), dialer, uri)
	if err != nil {
		t.Fatal(err)
	}
	return d.DialContext(t.Context(), "tcp", target)
}

func recvSNI(t *testing.T, sni <-chan string) string {
	t.Helper()
	select {
	case s := <-sni:
		return s
	case <-time.After(2 * time.Second):
		t.Fatal("server received no SNI")
		return ""
	}
}

type countingConn struct {
	net.Conn
	closes *atomic.Int32
}

func (c countingConn) Close() error {
	c.closes.Add(1)
	return c.Conn.Close()
}

func counting(dialer bridge.Dialer, closes *atomic.Int32) bridge.Dialer {
	return bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		c, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		return countingConn{c, closes}, nil
	})
}

func TestTLS_RejectsUntrustedByDefault(t *testing.T) {
	srv, _ := newServer(t)
	for _, uri := range []string{"tls:", "tls://localhost", "tls:127.0.0.1", "tls://127.0.0.1"} {
		t.Run(uri, func(t *testing.T) {
			c, err := dial(t, toServer(srv), uri, "127.0.0.1:443")
			if err == nil {
				c.Close()
				t.Fatal("handshake with untrusted certificate succeeded")
			}
			var verr *tls.CertificateVerificationError
			if !errors.As(err, &verr) {
				t.Fatalf("got %v, want certificate verification error", err)
			}
		})
	}
}

func TestTLS_ClosesRawConnOnHandshakeFailure(t *testing.T) {
	srv, _ := newServer(t)
	var closes atomic.Int32
	if _, err := dial(t, counting(toServer(srv), &closes), "tls:example.com", "127.0.0.1:443"); err == nil {
		t.Fatal("handshake with untrusted certificate succeeded")
	}
	if closes.Load() == 0 {
		t.Fatal("raw connection left open after failed handshake")
	}
}

func TestTLS_HandshakeHonorsContext(t *testing.T) {
	client, server := net.Pipe()
	defer server.Close()
	var closes atomic.Int32
	stalled := counting(bridge.DialFunc(func(context.Context, string, string) (net.Conn, error) {
		return client, nil
	}), &closes)
	d, err := TLS(t.Context(), stalled, "tls:")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 100*time.Millisecond)
	defer cancel()
	res := make(chan error, 1)
	go func() {
		_, err := d.DialContext(ctx, "tcp", "example.com:443")
		res <- err
	}()
	select {
	case err := <-res:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("got %v, want %v", err, context.DeadlineExceeded)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("DialContext did not return after the context deadline")
	}
	if closes.Load() == 0 {
		t.Fatal("raw connection left open after canceled handshake")
	}
}

func TestTLS_RejectsMalformedInsecure(t *testing.T) {
	for _, uri := range []string{"tls:?insecure=yes", "tls://localhost?insecure=", "tls:example.com?insecure", "tls:example.com?insecure=%zz", "tls:example.com?insecure=true;other=false"} {
		if _, err := TLS(t.Context(), nil, uri); err == nil {
			t.Errorf("%s: accepted malformed insecure value", uri)
		}
	}
}

func TestTLS_ServerName(t *testing.T) {
	tests := []struct{ uri, target, want string }{
		{"tls:?insecure=true", "target.test:443", "target.test"},
		{"tls:opaque.test?insecure=true", "target.test:443", "opaque.test"},
		{"tls://host.test:8443?insecure=1", "target.test:443", "host.test"},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			srv, sni := newServer(t)
			c, err := dial(t, toServer(srv), tt.uri, tt.target)
			if err != nil {
				t.Fatal(err)
			}
			c.Close()
			if got := recvSNI(t, sni); got != tt.want {
				t.Fatalf("SNI = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTLS_TrustedRootVerifies(t *testing.T) {
	if os.Getenv("BRIDGE_TLS_TRUSTED_ROOT") == "" {
		cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestTLS_TrustedRootVerifies$", "-test.v")
		cmd.Env = append(os.Environ(), "BRIDGE_TLS_TRUSTED_ROOT=1", "GODEBUG=x509usefallbackroots=1")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v\n%s", err, out)
		}
		return
	}
	srv, _ := newServer(t)
	pool := x509.NewCertPool()
	pool.AddCert(srv.Certificate())
	x509.SetFallbackRoots(pool)
	for _, uri := range []string{"tls:", "tls:example.com", "tls://127.0.0.1"} {
		c, err := dial(t, toServer(srv), uri, "127.0.0.1:443")
		if err != nil {
			t.Fatalf("%s: %v", uri, err)
		}
		c.Close()
	}
}

func TestTLS_ConcurrentDials(t *testing.T) {
	srv, sni := newServer(t)
	d, err := TLS(t.Context(), toServer(srv), "tls:?insecure=true")
	if err != nil {
		t.Fatal(err)
	}
	names := []string{"a.test", "b.test", "c.test", "d.test"}
	var wg sync.WaitGroup
	for _, name := range names {
		wg.Go(func() {
			c, err := d.DialContext(t.Context(), "tcp", name+":443")
			if err != nil {
				t.Error(err)
				return
			}
			c.Close()
		})
	}
	wg.Wait()
	if t.Failed() {
		return
	}
	seen := map[string]bool{}
	for range names {
		seen[recvSNI(t, sni)] = true
	}
	for _, name := range names {
		if !seen[name] {
			t.Errorf("server never saw SNI %q", name)
		}
	}
}
