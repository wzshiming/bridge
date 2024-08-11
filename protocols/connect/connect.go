package connect

import (
	"context"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/httpproxy"
)

// CONNECT https?://[username:password@]{address}
func CONNECT(ctx context.Context, dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	d, err := httpproxy.NewDialer(address)
	if err != nil {
		return nil, err
	}
	if dialer != nil {
		d.ProxyDial = dialer.DialContext
	}
	return d, nil
}
