package socks5

import (
	"context"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/socks5"
)

// SOCKS5 socks5://[username:password@]{address}
func SOCKS5(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	d, err := socks5.NewDialer(addr)
	if err != nil {
		return nil, err
	}
	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil
}
