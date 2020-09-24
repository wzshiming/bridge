package proxy

import (
	"net"
	"net/http"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/httpproxy"
	"github.com/wzshiming/socks4"
	"github.com/wzshiming/socks5"
)

type proxy struct {
	socks4Proxy socks4.Server
	socks5Proxy socks5.Server
	httpProxy   http.Server
}

func NewProxy(dial bridge.Dialer) *proxy {
	return &proxy{
		socks4Proxy: socks4.Server{
			ProxyDial: dial.DialContext,
		},
		socks5Proxy: socks5.Server{
			ProxyDial: dial.DialContext,
		},
		httpProxy: http.Server{
			Handler: &httpproxy.ProxyHandler{
				ProxyDial: dial.DialContext,
			},
		},
	}
}

// ServeConn is used to serve a single connection.
func (s *proxy) ServeConn(conn net.Conn) {
	var p [1]byte
	_, err := conn.Read(p[:])
	if err != nil {
		conn.Close()
		return
	}
	conn = UnreadConn(conn, []byte{p[0]})
	switch p[0] {
	case 0x04:
		s.socks4Proxy.ServeConn(conn)
	case 0x05:
		s.socks5Proxy.ServeConn(conn)
	default:
		s.httpProxy.Serve(NewSingleConnListener(conn))
	}
}
