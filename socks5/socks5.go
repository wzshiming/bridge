package socks5

import (
	"net/url"

	"github.com/wzshiming/bridge"
	"golang.org/x/net/proxy"
)

// SOCKS5 socks5://[username:password@]{address}
func SOCKS5(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	ur, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}

	var auth *proxy.Auth
	if ur.User != nil {
		user := ur.User.Username()
		pwd, _ := ur.User.Password()
		auth = &proxy.Auth{
			User:     user,
			Password: pwd,
		}

	}
	host := ur.Host

	return proxy.SOCKS5("tcp", host, auth, dialer)
}
