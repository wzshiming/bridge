package socks5

import (
	"context"
	"net"
	"net/url"

	"github.com/wzshiming/bridge"
	"golang.org/x/net/proxy"
)

// SOCKS5 socks5://[username:password@]{address}
func SOCKS5(dialer bridge.Dialer, addr string) (bridge.Dialer, bridge.ListenConfig, error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return nil, nil, err
	}

	var auth *proxy.Auth
	var pd proxy.Dialer
	if dialer != nil {
		pd = newDialerWarp(dialer)
	}
	if ur.User != nil {
		auth = &proxy.Auth{}
		auth.User = ur.User.Username()
		auth.Password, _ = ur.User.Password()
	}
	d, err := proxy.SOCKS5("tcp", ur.Host, auth, pd)
	if err != nil {
		return nil, nil, err
	}
	return d.(proxy.ContextDialer), nil, nil
}

type dialerWrap struct {
	proxy.ContextDialer
}

func newDialerWarp(d proxy.ContextDialer) proxy.Dialer {
	if p, ok := d.(proxy.Dialer); ok {
		return p
	}
	return dialerWrap{d}
}

// Dial connects to the given address via the proxy.
func (d dialerWrap) Dial(network, addr string) (c net.Conn, err error) {
	return d.DialContext(context.Background(), network, addr)
}
