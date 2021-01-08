package command

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"sync"
	"sync/atomic"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/warp"
	"github.com/wzshiming/bridge/local"
	"github.com/wzshiming/cmux"
	"github.com/wzshiming/commandproxy"
)

var (
	ErrNotSupported = fmt.Errorf("is not supported 'cmd'")
	ErrConnClosed   = errors.New("use of closed network connection")
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
		localAddr:    warp.NewNetAddr(uri.Scheme, uri.Opaque),
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

	remoteAddr := warp.NewNetAddr(network, address)
	return &listener{
		ctx:           ctx,
		commandDialer: c.CommandDialer,
		localAddr:     c.pd.localAddr,
		remoteAddr:    remoteAddr,
		proxy:         proxy,
	}, nil
}

func (c *command) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	proxy := c.pd.proxyCommand.Format(network, address)
	remoteAddr := warp.NewNetAddr(network, address)
	return commandConnContext(ctx, c.CommandDialer, c.pd.localAddr, remoteAddr, proxy)
}

type listener struct {
	ctx           context.Context
	commandDialer bridge.CommandDialer
	proxy         []string
	localAddr     net.Addr
	remoteAddr    net.Addr
	isClose       uint32
	mux           sync.Mutex
}

func (l *listener) Accept() (net.Conn, error) {
	l.mux.Lock()
	defer l.mux.Unlock()
	if atomic.LoadUint32(&l.isClose) == 1 {
		return nil, ErrConnClosed
	}

	n, err := commandConnContext(l.ctx, l.commandDialer, l.localAddr, l.remoteAddr, l.proxy)
	if err != nil {
		return nil, err
	}

	// Because there is no way to tell if there is a connection coming in from the command line,
	// the next listen can only be performed if the data is read or closed
	var tmp [1]byte
	_, err = n.Read(tmp[:])
	if err != nil {
		return nil, err
	}
	n = cmux.UnreadConn(n, tmp[:])
	return n, nil
}

func (l *listener) Close() error {
	atomic.StoreUint32(&l.isClose, 1)
	return nil
}

func (l *listener) Addr() net.Addr {
	return l.localAddr
}

func commandConnContext(ctx context.Context, commandDialer bridge.CommandDialer, localAddr, remoteAddr net.Addr, proxy []string) (net.Conn, error) {
	conn, err := commandDialer.CommandDialContext(ctx, proxy[0], proxy[1:]...)
	if err != nil {
		return nil, err
	}

	conn = warp.ConnWithAddr(conn, localAddr, remoteAddr)
	return conn, nil
}
