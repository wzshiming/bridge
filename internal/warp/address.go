package warp

import (
	"net"
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
