package proxy

import (
	"bufio"
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
	// this layer of packaging is to dump the output without interruption
	conn = &connBuffReader{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
	}

	var p [1]byte
	n, err := conn.Read(p[:])
	if err != nil {
		conn.Close()
		return
	}
	conn = UnreadConn(conn, p[:n])
	switch p[0] {
	case 0x04:
		s.socks4Proxy.ServeConn(conn)
	case 0x05:
		s.socks5Proxy.ServeConn(conn)
	default:
		s.httpProxy.Serve(NewSingleConnListener(conn))
	}
}

type connBuffReader struct {
	net.Conn
	*bufio.Reader
}

func (c *connBuffReader) Read(p []byte) (n int, err error) {
	return c.Reader.Read(p)
}
