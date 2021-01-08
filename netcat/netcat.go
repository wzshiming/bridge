package netcat

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/command"
	"github.com/wzshiming/bridge/local"
)

// NetCat nc: [prefix]
func NetCat(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	var prefix string
	u, err := url.Parse(cmd)
	if err == nil {
		prefix = u.Opaque
	}
	return &netCat{
		prefix:  prefix,
		dialer:  dialer,
		command: command.COMMAND,
	}, nil
}

type netCat struct {
	prefix       string
	dialer       bridge.Dialer
	tcpDialer    bridge.Dialer
	unixDialer   bridge.Dialer
	tcpListener  bridge.ListenConfig
	unixListener bridge.ListenConfig
	command      func(dialer bridge.Dialer, cmd string) (bridge.Dialer, error)
}

func (n *netCat) exec(cmd string) (bridge.Dialer, error) {
	return n.command(n.dialer, strings.Join([]string{"cmd:", n.prefix, cmd}, " "))
}

func (n *netCat) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network == "unix" {
		if n.unixDialer == nil {
			d, err := n.exec("nc -U %h")
			if err != nil {
				return nil, err
			}
			n.unixDialer = d
		}
		return n.unixDialer.DialContext(ctx, network, address)
	}
	if n.tcpDialer == nil {
		cmd := "nc %h %p"
		switch network {
		case "tcp4":
			cmd = "nc -4 %h %p"
		case "tcp6":
			cmd = "nc -6 %h %p"
		}
		d, err := n.exec(cmd)
		if err != nil {
			return nil, err
		}
		n.tcpDialer = d
	}
	return n.tcpDialer.DialContext(ctx, network, address)
}

func (n *netCat) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if network == "unix" {
		if n.unixListener == nil {
			d, err := n.exec("nc -Ul %h")
			if err != nil {
				return nil, err
			}
			n.unixListener = d.(bridge.ListenConfig)
		}
		return n.unixListener.Listen(ctx, network, address)
	}
	if n.tcpListener == nil {
		cmd := "nc -l %h %p"
		switch network {
		case "tcp4":
			cmd = "nc -4l %h %p"
		case "tcp6":
			cmd = "nc -6l %h %p"
		}
		d, err := n.exec(cmd)
		if err != nil {
			return nil, err
		}
		n.tcpListener = d.(bridge.ListenConfig)
	}
	return n.tcpListener.Listen(ctx, network, address)
}
