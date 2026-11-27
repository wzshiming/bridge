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

// ListenPacketConfig contains options for listening to a packet address.
type ListenPacketConfig interface {
	ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error)
}

// ListenPacketConfigFunc type is an adapter for ListenPacketConfig.
type ListenPacketConfigFunc func(ctx context.Context, network, address string) (net.PacketConn, error)

// ListenPacket calls l(ctx, network, address)
func (l ListenPacketConfigFunc) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
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
	Bridge(ctx context.Context, dialer Dialer, address string) (Dialer, error)
}

// BridgeFunc type is an adapter for Bridger.
type BridgeFunc func(ctx context.Context, dialer Dialer, address string) (Dialer, error)

// Bridge calls b(dialer, address)
func (b BridgeFunc) Bridge(ctx context.Context, dialer Dialer, address string) (Dialer, error) {
	return b(ctx, dialer, address)
}
