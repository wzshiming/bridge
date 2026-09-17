package netutils

import (
	"context"
	"net"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/cmux"
)

func ConnWithCloser(conn net.Conn, closer func() error) net.Conn {
	return &connCloser{Conn: conn, closer: closer}
}

type connCloser struct {
	net.Conn
	closer func() error
}

func (w *connCloser) Close() error {
	return w.closer()
}

func ConnWithAddr(conn net.Conn, localAddr, remoteAddr net.Addr) net.Conn {
	return &connAddr{Conn: conn, localAddr: localAddr, remoteAddr: remoteAddr}
}

type connAddr struct {
	net.Conn
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (w *connAddr) LocalAddr() net.Addr {
	if w.localAddr == nil {
		return w.Conn.LocalAddr()
	}
	return w.localAddr
}

func (w *connAddr) RemoteAddr() net.Addr {
	if w.remoteAddr == nil {
		return w.Conn.RemoteAddr()
	}
	return w.remoteAddr
}

func NewNetAddr(network, address string) net.Addr {
	return &addr{network: network, address: address}
}

type addr struct {
	network string
	address string
}

func (a *addr) Network() string {
	return a.network
}
func (a *addr) String() string {
	return a.address
}

func NewCommandDialContext(ctx context.Context, commandDialer bridge.CommandDialer, localAddr, remoteAddr net.Addr, proxy []string) (net.Conn, error) {
	conn, err := commandDialer.CommandDialContext(ctx, proxy[0], proxy[1:]...)
	if err != nil {
		return nil, err
	}

	conn = ConnWithAddr(conn, localAddr, remoteAddr)
	return conn, nil
}

func NewCommandListener(ctx context.Context, commandDialer bridge.CommandDialer, localAddr net.Addr, remoteAddr net.Addr, proxy []string) (net.Listener, error) {
	return &listener{
		ctx:           ctx,
		commandDialer: commandDialer,
		localAddr:     localAddr,
		remoteAddr:    remoteAddr,
		proxy:         proxy,
	}, nil
}

type listener struct {
	ctx           context.Context
	commandDialer bridge.CommandDialer
	proxy         []string
	localAddr     net.Addr
	remoteAddr    net.Addr
	mux           sync.Mutex
	state         sync.Mutex
	closed        bool
	cancel        context.CancelFunc
}

func (l *listener) Accept() (net.Conn, error) {
	l.mux.Lock()
	defer l.mux.Unlock()

	l.state.Lock()
	if l.closed {
		l.state.Unlock()
		return nil, ErrClosedConn
	}
	// Canceled by Close only while pending, so accepted commands outlive the listener.
	ctx, cancel := context.WithCancel(l.ctx)
	l.cancel = cancel
	l.state.Unlock()

	n, err := l.probe(ctx)

	l.state.Lock()
	l.cancel = nil
	l.state.Unlock()

	if ctx.Err() != nil {
		if n != nil {
			n.Close()
		}
		cancel()
		return nil, ErrClosedConn
	}
	if err != nil {
		cancel()
		return nil, err
	}
	return ConnWithCloser(n, func() error {
		err := n.Close()
		cancel()
		return err
	}), nil
}

// probe dials one command and waits for its first byte, closing the conn if ctx ends first.
func (l *listener) probe(ctx context.Context) (net.Conn, error) {
	n, err := NewCommandDialContext(ctx, l.commandDialer, l.localAddr, l.remoteAddr, l.proxy)
	if err != nil {
		return nil, err
	}

	// Because there is no way to tell if there is a connection coming in from the command line,
	// the next listen can only be performed if the data is read or closed
	stop := context.AfterFunc(ctx, func() { n.Close() })
	var tmp [1]byte
	_, err = n.Read(tmp[:])
	if !stop() {
		return nil, ctx.Err()
	}
	if err != nil {
		n.Close()
		return nil, err
	}
	return cmux.UnreadConn(n, tmp[:]), nil
}

func (l *listener) Close() error {
	l.state.Lock()
	defer l.state.Unlock()
	l.closed = true
	if l.cancel != nil {
		l.cancel()
	}
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.localAddr
}
