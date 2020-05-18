package local

import (
	"context"
	"net"
)

var LOCAL Local

type Local struct{}

func (Local) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	var dialer net.Dialer
	return dialer.DialContext(ctx, network, address)
}

func (Local) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	var listenConfig net.ListenConfig
	return listenConfig.Listen(ctx, network, address)
}
