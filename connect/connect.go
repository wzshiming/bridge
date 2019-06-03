package connect

import (
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/httpproxy"
)

// CONNECT https?://[username:password@]{address}
func CONNECT(dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	d, err := httpproxy.NewDialer(address)
	if err != nil {
		return nil, err
	}
	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil
}
