package tls

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
)

// TLS tls:[opaque]
func TLS(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	uri, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	return bridge.DialFunc(func(ctx context.Context, network, addr string) (c net.Conn, err error) {
		c, err = dialer.DialContext(ctx, network, addr)
		if err != nil {
			return nil, err
		}

		conf := &tls.Config{}
		if uri.Opaque == "" || net.ParseIP(uri.Opaque) != nil {
			conf.InsecureSkipVerify = true
		} else {
			conf.ServerName = uri.Opaque
		}

		tc := tls.Client(c, conf)
		err = tc.Handshake()
		if err != nil {
			return nil, err
		}
		return tc, nil
	}), nil
}
