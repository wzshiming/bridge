package ws

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/local"
	"github.com/wzshiming/commandproxy"
	"golang.org/x/net/websocket"
)

// WS ws: 'ws://[username:password@]{domain}[{path}[?[query]]]' 'Origin: http://domain'
func WS(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	config := &websocket.Config{
		Version: websocket.ProtocolVersionHybi13,
		Header:  http.Header{},
	}
	addr = strings.TrimPrefix(addr, "ws: ")
	addr = strings.TrimSpace(addr)

	scmd, err := commandproxy.SplitCommand(addr)
	if err != nil {
		return nil, err
	}

	addr = scmd[0]
	for _, header := range scmd {
		i := strings.IndexByte(header, ':')
		if i == -1 {
			continue
		}
		key := header[:i]
		config.Header.Add(key, strings.TrimSpace(header[i+1:]))
	}
	location, err := url.ParseRequestURI(addr)
	if err != nil {
		return nil, err
	}
	config.Location = location

	const Origin = "Origin"
	if origin := config.Header.Get(Origin); origin == "" {
		u, err := location.Parse("/")
		if err != nil {
			return nil, err
		}
		u.User = nil
		if location.Scheme == "wss" {
			u.Scheme = "https"
		} else {
			u.Scheme = "http"
		}
		config.Header.Set(Origin, u.String())
	}
	origin, err := url.ParseRequestURI(config.Header.Get(Origin))
	if err != nil {
		return nil, err
	}
	config.Origin = origin

	return bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
		conn, err := dialWithDialer(ctx, dialer, config)
		if err != nil {
			return nil, err
		}
		return websocket.NewClient(config, conn)
	}), nil
}

func dialWithDialer(ctx context.Context, dialer bridge.Dialer, config *websocket.Config) (net.Conn, error) {
	addr := parseAuthority(config.Location)
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	if config.Location.Scheme == "wss" {
		tlsConn := tls.Client(conn, config.TlsConfig)
		err = tlsConn.Handshake()
		if err != nil {
			return nil, err
		}
		conn = tlsConn
	}
	return conn, nil
}

var portMap = map[string]string{
	"ws":  "80",
	"wss": "443",
}

func parseAuthority(location *url.URL) string {
	if _, ok := portMap[location.Scheme]; ok {
		if _, _, err := net.SplitHostPort(location.Host); err != nil {
			return net.JoinHostPort(location.Host, portMap[location.Scheme])
		}
	}
	return location.Host
}
