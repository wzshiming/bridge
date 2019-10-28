package socks

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/socks"
)

// SOCKS socks5://[username:password@]{address}
func SOCKS(dialer bridge.Dialer, addr string) (bridge.Dialer, bridge.ListenConfig, error) {
	d, err := socks.NewDialer(addr)
	if err != nil {
		return nil, nil, err
	}

	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil, nil
}
