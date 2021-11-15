package command

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/wrapping"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
)

var (
	ErrNotSupported = fmt.Errorf("is not supported 'cmd'")
)

// COMMAND cmd:shell
func COMMAND(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	cd, ok := dialer.(bridge.CommandDialer)
	if !ok {
		return nil, ErrNotSupported
	}
	pd, err := newProxyDialer(cmd)
	if err != nil {
		return nil, err
	}
	return &command{
		pd:            pd,
		CommandDialer: cd,
	}, nil
}

func newProxyDialer(cmd string) (*proxyDialer, error) {
	uri, err := url.Parse(cmd)
	if err != nil {
		return nil, err
	}

	scmd, err := commandproxy.SplitCommand(uri.Opaque)
	if err != nil {
		return nil, err
	}
	return &proxyDialer{
		proxyCommand: scmd,
		localAddr:    wrapping.NewNetAddr(uri.Scheme, uri.Opaque),
	}, nil
}

type proxyDialer struct {
	proxyCommand commandproxy.DialProxyCommand
	localAddr    net.Addr
}

type command struct {
	pd *proxyDialer
	bridge.CommandDialer
}

func (c *command) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	proxy := c.pd.proxyCommand.Format(network, address)
	remoteAddr := wrapping.NewNetAddr(network, address)
	return wrapping.NewCommandListener(ctx, c.CommandDialer, c.pd.localAddr, remoteAddr, proxy)
}

func (c *command) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	proxy := c.pd.proxyCommand.Format(network, address)
	remoteAddr := wrapping.NewNetAddr(network, address)
	return wrapping.NewCommandDialContext(ctx, c.CommandDialer, c.pd.localAddr, remoteAddr, proxy)
}
