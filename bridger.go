package bridge

import (
	"context"
	"net"
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
	DialContext(ctx context.Context, network, address string) (net.Conn, error)
}

// DialFunc type is an adapter for Dialer.
type DialFunc func(ctx context.Context, network, address string) (net.Conn, error)

// DialContext calls d(ctx, network, address)
func (d DialFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
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

// CommandDialer contains options for connecting to an address with command.
type CommandDialer interface {
	CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error)
}

// CommandDialFunc type is an adapter for Dialer  with command.
type CommandDialFunc func(ctx context.Context, name string, args ...string) (net.Conn, error)

// CommandDialFunc calls d(ctx, name, args...)
func (d CommandDialFunc) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	return d(ctx, name, args...)
}
