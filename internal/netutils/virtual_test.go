package netutils

import (
	"context"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

const virtualWait = 5 * time.Second

type connResult struct {
	conn net.Conn
	err  error
}

type listenResult struct {
	l   net.Listener
	err error
}

func await[T any](t *testing.T, ch <-chan T, what string) T {
	t.Helper()
	select {
	case v := <-ch:
		return v
	case <-time.After(virtualWait):
		t.Fatalf("%s did not complete within %v", what, virtualWait)
		var zero T
		return zero
	}
}

func dialAsync(ctx context.Context, v *virtualNetworkManager, address string) <-chan connResult {
	ch := make(chan connResult, 1)
	go func() {
		c, err := v.DialContext(ctx, "tcp", address)
		ch <- connResult{c, err}
	}()
	return ch
}

func acceptAsync(l net.Listener) <-chan connResult {
	ch := make(chan connResult, 1)
	go func() {
		c, err := l.Accept()
		ch <- connResult{c, err}
	}()
	return ch
}

func listenAsync(v *virtualNetworkManager, address string) <-chan listenResult {
	ch := make(chan listenResult, 1)
	go func() {
		l, err := v.Listen(context.Background(), "tcp", address)
		ch <- listenResult{l, err}
	}()
	return ch
}

func connectPair(t *testing.T, v *virtualNetworkManager, l net.Listener, address string) (client, server net.Conn) {
	t.Helper()
	acc := acceptAsync(l)
	d := await(t, dialAsync(context.Background(), v, address), "dial "+address)
	if d.err != nil {
		t.Fatalf("dial %s: %v", address, d.err)
	}
	a := await(t, acc, "accept "+address)
	if a.err != nil {
		t.Fatalf("accept %s: %v", address, a.err)
	}
	return d.conn, a.conn
}

// handoffCtx closes reached the first time Done is evaluated, which Conn only does at its hand-off select.
type handoffCtx struct {
	context.Context
	once    sync.Once
	reached chan struct{}
}

func newHandoffCtx(parent context.Context) *handoffCtx {
	return &handoffCtx{Context: parent, reached: make(chan struct{})}
}

func (c *handoffCtx) Done() <-chan struct{} {
	c.once.Do(func() { close(c.reached) })
	return c.Context.Done()
}

func (c *handoffCtx) wait(t *testing.T) {
	t.Helper()
	await(t, c.reached, "dial reaching hand-off select")
}

func TestVirtualListenReplace(t *testing.T) {
	v := newVirtualNetworkManager()
	l1, err := v.Listen(context.Background(), "tcp", "a")
	if err != nil {
		t.Fatal(err)
	}
	r := await(t, listenAsync(v, "a"), "duplicate Listen")
	if r.err != nil {
		t.Fatal(r.err)
	}
	l2 := r.l
	if a := await(t, acceptAsync(l1), "Accept on replaced listener"); a.conn != nil || a.err != ErrClosedConn {
		t.Fatalf("replaced listener Accept = %v, %v; want nil, ErrClosedConn", a.conn, a.err)
	}
	c, s := connectPair(t, v, l2, "a")
	c.Close()
	s.Close()
	if err := l1.Close(); err != nil {
		t.Fatal(err)
	}
	c, s = connectPair(t, v, l2, "a")
	c.Close()
	s.Close()
	pending := acceptAsync(l2)
	if err := l2.Close(); err != nil {
		t.Fatal(err)
	}
	if a := await(t, pending, "pending Accept unblocked by Close"); a.conn != nil || a.err != ErrClosedConn {
		t.Fatalf("pending Accept after Close = %v, %v; want nil, ErrClosedConn", a.conn, a.err)
	}
	if a := await(t, acceptAsync(l2), "Accept on closed listener"); a.conn != nil || a.err != ErrClosedConn {
		t.Fatalf("closed listener Accept = %v, %v; want nil, ErrClosedConn", a.conn, a.err)
	}
	d := await(t, dialAsync(context.Background(), v, "a"), "dial after close")
	if d.conn != nil || d.err == nil || !strings.Contains(d.err.Error(), "couldn't connect to virtual server") {
		t.Fatalf("dial after close = %v, %v; want nil, couldn't connect error", d.conn, d.err)
	}
}

func TestVirtualDialCancel(t *testing.T) {
	v := newVirtualNetworkManager()
	l, err := v.Listen(context.Background(), "tcp", "a")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hc := newHandoffCtx(ctx)
	blocked := dialAsync(hc, v, "a")
	hc.wait(t)
	// Manager write lock must stay available while a dial waits for Accept.
	r := await(t, listenAsync(v, "b"), "Listen b during blocked dial")
	if r.err != nil {
		t.Fatal(r.err)
	}
	r.l.Close()
	cancel()
	if d := await(t, blocked, "canceled dial"); d.conn != nil || d.err != context.Canceled {
		t.Fatalf("canceled dial = %v, %v; want nil, context.Canceled", d.conn, d.err)
	}

	// An already-canceled ctx must fail before the hand-off select, not hand a conn to a ready acceptor.
	acc := acceptAsync(l)
	pre := newHandoffCtx(ctx)
	if d := await(t, dialAsync(pre, v, "a"), "pre-canceled dial"); d.conn != nil || d.err != context.Canceled {
		t.Fatalf("pre-canceled dial = %v, %v; want nil, context.Canceled", d.conn, d.err)
	}
	select {
	case <-pre.reached:
		t.Fatal("pre-canceled dial reached the hand-off select")
	default:
	}
	d := await(t, dialAsync(context.Background(), v, "a"), "dial")
	if d.err != nil {
		t.Fatal(d.err)
	}
	a := await(t, acc, "accept")
	if a.err != nil {
		t.Fatal(a.err)
	}
	assertExchange(t, d.conn, a.conn, "ping")
	d.conn.Close()
	a.conn.Close()
}

func TestVirtualCloseRace(t *testing.T) {
	v := newVirtualNetworkManager()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	for i := 0; i < 200; i++ {
		l, err := v.Listen(context.Background(), "tcp", "a")
		if err != nil {
			t.Fatal(err)
		}
		hc := newHandoffCtx(ctx)
		res := make(chan connResult, 1)
		go func() {
			defer func() {
				if p := recover(); p != nil {
					res <- connResult{err: fmt.Errorf("panic: %v", p)}
				}
			}()
			c, err := v.DialContext(hc, "tcp", "a")
			res <- connResult{c, err}
		}()
		hc.wait(t)
		l.Close()
		d := await(t, res, "dial closed at hand-off")
		if d.conn != nil || d.err != ErrClosedConn {
			t.Fatalf("iteration %d: dial closed at hand-off = %v, %v; want nil, ErrClosedConn", i, d.conn, d.err)
		}
	}
}

func TestVirtualConnAddrsAndData(t *testing.T) {
	v := newVirtualNetworkManager()
	l, err := v.Listen(context.Background(), "tcp", "a")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	if got := l.Addr(); got.Network() != "virtual" || got.String() != "a" {
		t.Fatalf("listener Addr = %s://%s; want virtual://a", got.Network(), got.String())
	}
	c, s := connectPair(t, v, l, "a")
	defer c.Close()
	defer s.Close()
	// Established conns must outlive their listener.
	if err := l.Close(); err != nil {
		t.Fatal(err)
	}
	for name, addr := range map[string]net.Addr{
		"client local": c.LocalAddr(), "client remote": c.RemoteAddr(),
		"server local": s.LocalAddr(), "server remote": s.RemoteAddr(),
	} {
		if addr.Network() != "virtual" || addr.String() != "a" {
			t.Errorf("%s addr = %s://%s; want virtual://a", name, addr.Network(), addr.String())
		}
	}
	assertExchange(t, c, s, "hello")
	assertExchange(t, s, c, "world")
}

func assertExchange(t *testing.T, w, r net.Conn, msg string) {
	t.Helper()
	werr := make(chan error, 1)
	go func() {
		_, err := w.Write([]byte(msg))
		werr <- err
	}()
	buf := make([]byte, len(msg))
	r.SetReadDeadline(time.Now().Add(virtualWait))
	if _, err := io.ReadFull(r, buf); err != nil || string(buf) != msg {
		t.Fatalf("read = %q, %v; want %q, nil", buf, err, msg)
	}
	if err := await(t, werr, "write"); err != nil {
		t.Fatal(err)
	}
}
