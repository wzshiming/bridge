package bridge

import (
	"context"
	"net"
)

type ListenConfig interface {
	Listen(ctx context.Context, network, addr string) (net.Listener, error)
}

type ListenConfigFunc func(ctx context.Context, network, addr string) (net.Listener, error)

func (b ListenConfigFunc) Listen(ctx context.Context, network, addr string) (net.Listener, error) {
	return b(ctx, network, addr)
}

type Dialer interface {
	DialContext(ctx context.Context, network, addr string) (c net.Conn, err error)
}

type Bridger interface {
	Bridge(dialer Dialer, addr string) (Dialer, ListenConfig, error)
}

type DialFunc func(ctx context.Context, network, addr string) (c net.Conn, err error)

func (b DialFunc) DialContext(ctx context.Context, network, addr string) (c net.Conn, err error) {
	return b(ctx, network, addr)
}

type BridgeFunc func(dialer Dialer, addr string) (Dialer, ListenConfig, error)

func (b BridgeFunc) Bridge(dialer Dialer, addr string) (Dialer, ListenConfig, error) {
	return b(dialer, addr)
}
