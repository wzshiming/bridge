package bridge

import (
	"context"
	"net"

	"golang.org/x/net/proxy"
)

// ListenConfig contains options for listening to an address.
type ListenConfig interface {
	Listen(ctx context.Context, network, address string) (net.Listener, error)
}

// ListenConfigFunc type is an adapter for ListenConfig.
type ListenConfigFunc func(ctx context.Context, network, address string) (net.Listener, error)

// Listen calls b(ctx, network, address)
func (l ListenConfigFunc) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return l(ctx, network, address)
}

// Dialer contains options for connecting to an address.
type Dialer interface {
	proxy.ContextDialer
}

// DialFunc type is an adapter for Dialer.
type DialFunc func(ctx context.Context, network, address string) (c net.Conn, err error)

// DialContext calls d(ctx, network, address)
func (d DialFunc) DialContext(ctx context.Context, network, address string) (c net.Conn, err error) {
	return d(ctx, network, address)
}

// Bridger contains options for crossing a bridge address.
type Bridger interface {
	Bridge(dialer Dialer, address string) (Dialer, error)
}

// BridgeFunc type is an adapter for Bridger.
type BridgeFunc func(dialer Dialer, address string) (Dialer, error)

// Bridge calls b(dialer, address)
func (b BridgeFunc) Bridge(dialer Dialer, address string) (Dialer, error) {
	return b(dialer, address)
}
