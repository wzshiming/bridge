package socks

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"time"
)

type Proto uint8

const (
	SOCKS4 Proto = iota
	SOCKS4A
	SOCKS5
)

// Dialer holds socks options.
type Dialer struct {
	ProxyDial func(context.Context, string, string) (net.Conn, error)
	Proto     Proto
	Host      string
	Auth      Auth
	Timeout   time.Duration
}

type Auth struct {
	Username string
	Password string
}

// NewDialer is create a new socks connection
func NewDialer(addr string) (*Dialer, error) {
	uri, err := url.Parse(addr)
	if err != nil {
		return nil, err
	}
	d := &Dialer{}
	switch uri.Scheme {
	case "socks4":
		d.Proto = SOCKS4
	case "socks4a":
		d.Proto = SOCKS4A
	case "socks5":
		d.Proto = SOCKS5
	default:
		return nil, fmt.Errorf("unknown SOCKS protocol %s", uri.Scheme)
	}
	d.Host = uri.Host
	if uri.User != nil {
		d.Auth.Username = uri.User.Username()
		d.Auth.Password, _ = uri.User.Password()
	}
	query := uri.Query()
	timeout := query.Get("timeout")
	if timeout != "" {
		var err error
		d.Timeout, err = time.ParseDuration(timeout)
		if err != nil {
			return nil, err
		}
	}
	return d, nil
}

// DialContext connects to the provided address on the provided network.
func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	switch d.Proto {
	case SOCKS5:
		return d.dialSocks5(ctx, network, address)
	case SOCKS4, SOCKS4A:
		return d.dialSocks4(ctx, network, address)
	}
	return nil, fmt.Errorf("unknown SOCKS protocol %v", d.Proto)
}

// Dial connects to the provided address on the provided network.
func (d *Dialer) Dial(network string, address string) (net.Conn, error) {
	return d.proxyDial(context.Background(), network, address)
}

func (d *Dialer) proxyDial(ctx context.Context, network string, address string) (net.Conn, error) {
	proxyDial := d.ProxyDial
	if proxyDial == nil {
		var dialer net.Dialer
		proxyDial = dialer.DialContext
	}

	return proxyDial(ctx, network, address)
}

func (d *Dialer) dialSocks5(ctx context.Context, network string, address string) (conn net.Conn, err error) {
	proxy := d.Host

	// dial TCP
	conn, err = d.proxyDial(ctx, network, proxy)
	if err != nil {
		return
	}

	// version identifier/method selection request
	req := []byte{
		5, // version number
		1, // number of methods
		0, // method 0: no authentication (only anonymous access supported for now)
	}
	resp, err := d.sendReceive(conn, req)
	if err != nil {
		return
	} else if len(resp) != 2 {
		err = errors.New("Server does not respond properly")
		return
	} else if resp[0] != 5 {
		err = errors.New("Server does not support Socks 5")
		return
	} else if resp[1] != 0 { // no auth
		err = errors.New("socks method negotiation failed")
		return
	}

	// detail request
	host, port, err := splitHostPort(address)
	if err != nil {
		return nil, err
	}
	req = []byte{
		5,               // version number
		1,               // connect command
		0,               // reserved, must be zero
		3,               // address type, 3 means domain name
		byte(len(host)), // address length
	}
	req = append(req, []byte(host)...)
	req = append(req, []byte{
		byte(port >> 8), // higher byte of destination port
		byte(port),      // lower byte of destination port (big endian)
	}...)
	resp, err = d.sendReceive(conn, req)
	if err != nil {
		return
	} else if len(resp) != 10 {
		err = errors.New("Server does not respond properly")
	} else if resp[1] != 0 {
		err = errors.New("Can't complete SOCKS5 connection")
	}

	return
}

func (d *Dialer) dialSocks4(ctx context.Context, network string, address string) (conn net.Conn, err error) {
	socksType := d.Proto
	proxy := d.Host

	// dial TCP
	conn, err = d.proxyDial(ctx, network, proxy)
	if err != nil {
		return
	}

	// connection request
	host, port, err := splitHostPort(address)
	if err != nil {
		return
	}
	ip := net.IPv4(0, 0, 0, 1).To4()
	if socksType == SOCKS4 {
		ip, err = lookupIP(host)
		if err != nil {
			return
		}
	}
	req := []byte{
		4,                          // version number
		1,                          // command CONNECT
		byte(port >> 8),            // higher byte of destination port
		byte(port),                 // lower byte of destination port (big endian)
		ip[0], ip[1], ip[2], ip[3], // special invalid IP address to indicate the host name is provided
		0, // user id is empty, anonymous proxy only
	}
	if socksType == SOCKS4A {
		req = append(req, []byte(host+"\x00")...)
	}

	resp, err := d.sendReceive(conn, req)
	if err != nil {
		return
	} else if len(resp) != 8 {
		err = errors.New("Server does not respond properly")
		return
	}
	switch resp[1] {
	case 90:
		// request granted
	case 91:
		err = errors.New("Socks connection request rejected or failed")
	case 92:
		err = errors.New("Socks connection request rejected becasue SOCKS server cannot connect to identd on the client")
	case 93:
		err = errors.New("Socks connection request rejected because the client program and identd report different user-id")
	default:
		err = errors.New("Socks connection request failed, unknown error")
	}
	// clear the deadline before returning
	if err := conn.SetDeadline(time.Time{}); err != nil {
		return nil, err
	}
	return
}

func (d *Dialer) sendReceive(conn net.Conn, req []byte) (resp []byte, err error) {
	if d.Timeout > 0 {
		if err := conn.SetWriteDeadline(time.Now().Add(d.Timeout)); err != nil {
			return nil, err
		}
	}
	_, err = conn.Write(req)
	if err != nil {
		return
	}
	resp, err = d.readAll(conn)
	return
}

func (d *Dialer) readAll(conn net.Conn) (resp []byte, err error) {
	resp = make([]byte, 1024)
	if d.Timeout > 0 {
		if err := conn.SetReadDeadline(time.Now().Add(d.Timeout)); err != nil {
			return nil, err
		}
	}
	n, err := conn.Read(resp)
	resp = resp[:n]
	return
}

func lookupIP(host string) (ip net.IP, err error) {
	ips, err := net.LookupIP(host)
	if err != nil {
		return
	}
	if len(ips) == 0 {
		err = fmt.Errorf("Cannot resolve host: %s", host)
		return
	}
	ip = ips[0].To4()
	if len(ip) != net.IPv4len {
		err = errors.New("IPv6 is not supported by SOCKS4")
		return
	}
	return
}

func splitHostPort(addr string) (host string, port uint16, err error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		return "", 0, err
	}
	portInt, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil {
		return "", 0, err
	}
	port = uint16(portInt)
	return
}
