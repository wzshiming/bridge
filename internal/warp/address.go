package warp

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/cmux"
)

var (
	ErrConnClosed = fmt.Errorf("use of closed network connection")
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
	isClose       uint32
	mux           sync.Mutex
}

func (l *listener) Accept() (net.Conn, error) {
	l.mux.Lock()
	defer l.mux.Unlock()
	if atomic.LoadUint32(&l.isClose) == 1 {
		return nil, ErrConnClosed
	}

	n, err := NewCommandDialContext(l.ctx, l.commandDialer, l.localAddr, l.remoteAddr, l.proxy)
	if err != nil {
		return nil, err
	}

	// Because there is no way to tell if there is a connection coming in from the command line,
	// the next listen can only be performed if the data is read or closed
	var tmp [1]byte
	_, err = n.Read(tmp[:])
	if err != nil {
		return nil, err
	}
	n = cmux.UnreadConn(n, tmp[:])
	return n, nil
}

func (l *listener) Close() error {
	atomic.StoreUint32(&l.isClose, 1)
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.localAddr
}
