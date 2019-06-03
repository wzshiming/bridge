package bridge

import (
	"context"
	"net"
)

type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (c net.Conn, err error)
}

type Bridger interface {
	Bridge(dialer Dialer, addr string) (Dialer, error)
}

type DialFunc func(ctx context.Context, network, addr string) (c net.Conn, err error)

func (b DialFunc) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	return b(ctx, network, addr)
}

type BridgeFunc func(dialer Dialer, addr string) (Dialer, error)

func (b BridgeFunc) Bridge(dialer Dialer, addr string) (Dialer, error) {
	return b(dialer, addr)
}
