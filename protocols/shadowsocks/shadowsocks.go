package shadowsocks

import (
	"context"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/shadowsocks"
	_ "github.com/wzshiming/shadowsocks/init"
)

// ShadowSocks ss://{cipher}:{password}@{address}
func ShadowSocks(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	d, err := shadowsocks.NewDialer(addr)
	if err != nil {
		return nil, err
	}
	d.ProxyDial = dialer.DialContext
	return d, nil
}
