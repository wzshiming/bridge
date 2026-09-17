package netcat

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/command"
	"github.com/wzshiming/bridge/protocols/local"
)

// NetCat nc: [prefix]
func NetCat(ctx context.Context, dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {
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
	prefix  string
	dialer  bridge.Dialer
	command func(ctx context.Context, dialer bridge.Dialer, cmd string) (bridge.Dialer, error)
}

func (n *netCat) exec(ctx context.Context, cmd string) (bridge.Dialer, error) {
	return n.command(ctx, n.dialer, strings.Join([]string{"cmd:", n.prefix, cmd}, " "))
}

func (n *netCat) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	cmd := "nc %h %p"
	switch network {
	case "tcp4":
		cmd = "nc -4 %h %p"
	case "tcp6":
		cmd = "nc -6 %h %p"
	case "unix":
		cmd = "nc -U %h"
	}
	d, err := n.exec(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return d.DialContext(ctx, network, address)
}

func (n *netCat) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	cmd := "nc -l %h %p"
	switch network {
	case "tcp4":
		cmd = "nc -4l %h %p"
	case "tcp6":
		cmd = "nc -6l %h %p"
	case "unix":
		cmd = "nc -Ul %h"
	}
	d, err := n.exec(ctx, cmd)
	if err != nil {
		return nil, err
	}
	return d.(bridge.ListenConfig).Listen(ctx, network, address)
}
