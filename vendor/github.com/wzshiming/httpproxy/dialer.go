package httpproxy

import (
	"bufio"
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/url"
)

// NewDialer is create a new HTTP CONNECT connection
func NewDialer(addr string) (*Dialer, error) {
	d := &Dialer{}

	proxy, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	d.userinfo = proxy.User
	switch proxy.Scheme {
	default:
		return nil, fmt.Errorf("unsupported protocol '%s'", proxy.Scheme)
	case "https":
		port := proxy.Port()
		if port == "" {
			port = "443"
		}
		hostname := proxy.Hostname()
		d.proxy = hostname + ":" + port
		d.TLSClientConfig = &tls.Config{
			ServerName: hostname,
		}
	case "http":
		port := proxy.Port()
		if port == "" {
			port = "80"
		}
		hostname := proxy.Hostname()
		d.proxy = hostname + ":" + port
	}
	return d, nil
}

// Dialer holds HTTP CONNECT options.
type Dialer struct {
	// ProxyDial specifies the optional dial function for
	// establishing the transport connection.
	ProxyDial func(context.Context, string, string) (net.Conn, error)

	// TLSClientConfig specifies the TLS configuration to use with
	// tls.Client.
	// If nil, the TLS is not used.
	// If non-nil, HTTP/2 support may not be enabled by default.
	TLSClientConfig *tls.Config

	proxy    string
	userinfo *url.Userinfo
}

func (d *Dialer) proxyDial(ctx context.Context, network string, address string) (net.Conn, error) {
	if d.ProxyDial == nil {
		return net.Dial(network, address)
	}

	rawConn, err := d.ProxyDial(ctx, network, address)
	if err != nil {
		return nil, err
	}

	config := d.TLSClientConfig
	if config == nil {
		return rawConn, nil
	}

	conn := tls.Client(rawConn, config)

	if err != nil {
		rawConn.Close()
		return nil, err
	}
	return conn, nil
}

// DialContext connects to the provided address on the provided network.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != "tcp" {
		return nil, fmt.Errorf("network '%v' unsupported", network)
	}

	conn, err := d.proxyDial(ctx, "tcp", d.proxy)
	if err != nil {
		return nil, err
	}
	req := &http.Request{
		Method: "CONNECT",
		URL:    &url.URL{Opaque: address},
		Host:   address,
		Header: http.Header{},
	}
	if d.userinfo != nil {
		req.Header.Set("Proxy-Authorization", "Basic "+d.userinfo.String())
	}

	err = req.Write(conn)
	if err != nil {
		return nil, err
	}

	br := bufio.NewReader(conn)
	resp, err := http.ReadResponse(br, req)
	if err != nil {
		conn.Close()
		return nil, err
	}

	if resp.StatusCode != 200 {
		conn.Close()
		return nil, fmt.Errorf("failed proxying %d: %s", resp.StatusCode, resp.Status)
	}
	return conn, nil
}

// Dial connects to the provided address on the provided network.
func (d *Dialer) Dial(network string, address string) (net.Conn, error) {
	return d.DialContext(context.Background(), network, address)
}
