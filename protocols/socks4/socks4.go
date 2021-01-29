package socks4

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/socks4"
)

// SOCKS4 socks4://[username@]{address}
func SOCKS4(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	d, err := socks4.NewDialer(addr)
	if err != nil {
		return nil, err
	}
	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil
}
