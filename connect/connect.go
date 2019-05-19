package connect

import (
	"context"
	"net"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/httpconnect"
)

// CONNECT https?://[username:password@]{address}
func CONNECT(dialer bridge.Dialer, address string) (bridge.Dialer, error) {
	d, err := httpconnect.NewDialer(address)
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
