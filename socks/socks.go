package socks

import (
	"context"
	"net"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/socks"
)

// SOCKS socks5://[username:password@]{address}
func SOCKS(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	d, err := socks.NewDialer(addr)
	if err != nil {
		return nil, err
	}

	if dialer != nil {
		d.ProxyDial = func(ctx context.Context, network string, address string) (net.Conn, error) {
			return dialer.Dial(network, address)
		}
	}
	return d, nil
}
