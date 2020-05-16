package socks

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/socks"
)

// SOCKS socks4://[username:password@]{address}
func SOCKS(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	d, err := socks.NewDialer(addr)
	if err != nil {
		return nil, err
	}

	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil
}
