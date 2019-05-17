package bridge

import (
	"net"
)

type Dialer interface {
	Dial(network, addr string) (c net.Conn, err error)
}

type Bridger interface {
	Bridge(dialer Dialer, addr string) (Dialer, error)
}

type DialFunc func(network, addr string) (c net.Conn, err error)

func (b DialFunc) Dial(network, addr string) (c net.Conn, err error) {
	return b(network, addr)
}

type BridgeFunc func(dialer Dialer, addr string) (Dialer, error)

func (b BridgeFunc) Bridge(dialer Dialer, addr string) (Dialer, error) {
	return b(dialer, addr)
}
