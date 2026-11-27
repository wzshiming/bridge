package chain

import (
	"context"
	"errors"
	"net"

	"github.com/wzshiming/permuteproxy"
	"github.com/wzshiming/permuteproxy/protocols/local"
)

// Local is the network of this machine, what a chain's first hop is crossed over.
var Local permuteproxy.Network = system{}

// system is the system's networking and commands, and beside them each network permuteproxy has registered under a name, as pipe on Windows.
type system struct{}

// named is a network the system does not know, built from its registration.
func named(network, address string) (permuteproxy.Transport, error) {
	return permuteproxy.TransportEngine{}.Transport(permuteproxy.Chain{Endpoint: permuteproxy.Endpoint{Network: network, Address: address}})
}

func (system) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	conn, err := local.Network.DialContext(ctx, network, address)
	if !errors.Is(err, permuteproxy.ErrUnsupported) {
		return conn, err
	}
	t, err := named(network, address)
	if err != nil {
		return nil, err
	}
	return t.Dial(ctx, t.Planes())
}

func (system) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	ln, err := local.Network.Listen(ctx, network, address)
	if !errors.Is(err, permuteproxy.ErrUnsupported) {
		return ln, err
	}
	t, err := named(network, address)
	if err != nil {
		return nil, err
	}
	return t.Listen(ctx)
}

func (system) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	pc, err := local.Network.ListenPacket(ctx, network, address)
	if !errors.Is(err, permuteproxy.ErrUnsupported) {
		return pc, err
	}
	t, err := named(network, address)
	if err != nil {
		return nil, err
	}
	return t.ListenPacket(ctx)
}
