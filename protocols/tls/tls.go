package tls

import (
	"context"
	"crypto/tls"
	"net"
	"net/url"
	"strconv"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
)

// TLS tls:[name][?insecure=true] or tls://[name][?insecure=true]
func TLS(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	uri, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	query, err := url.ParseQuery(uri.RawQuery)
	if err != nil {
		return nil, err
	}
	insecure := false
	if values, ok := query["insecure"]; ok {
		insecure, err = strconv.ParseBool(values[0])
		if err != nil {
			return nil, err
		}
	}
	name := uri.Opaque
	if name == "" {
		name = uri.Hostname()
	}
	return bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		c, err := dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}

		conf := &tls.Config{ServerName: name, InsecureSkipVerify: insecure}
		if conf.ServerName == "" {
			conf.ServerName, _, _ = net.SplitHostPort(address)
		}

		tc := tls.Client(c, conf)
		if err := tc.HandshakeContext(ctx); err != nil {
			c.Close()
			return nil, err
		}
		return tc, nil
	}), nil
}
