package local

import (
	"context"
	"net"
	"os"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/wrapping"
	"github.com/wzshiming/commandproxy"
)

var LOCAL = &Local{
	Dialer:       &net.Dialer{},
	ListenConfig: &net.ListenConfig{},
	LocalAddr:    wrapping.NewNetAddr("local", "local"),
}

type Local struct {
	Dialer       bridge.Dialer
	ListenConfig bridge.ListenConfig
	LocalAddr    net.Addr
}

func (l *Local) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return l.Dialer.DialContext(ctx, network, address)
}

func (l *Local) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	return l.ListenConfig.Listen(ctx, network, address)
}

func (l *Local) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	proxy := commandproxy.ProxyCommand(ctx, name, args...)
	proxy.Stderr = os.Stderr
	conn, err := proxy.Stdio()
	if err != nil {
		return nil, err
	}
	remoteAddr := wrapping.NewNetAddr("cmd", strings.Join(append([]string{name}, args...), " "))
	conn = wrapping.ConnWithAddr(conn, l.LocalAddr, remoteAddr)
	return conn, nil
}

func (l *Local) CommandListen(ctx context.Context, name string, args ...string) (net.Listener, error) {
	proxy := append([]string{name}, args...)
	remoteAddr := wrapping.NewNetAddr("cmd", strings.Join(proxy, " "))
	return wrapping.NewCommandListener(ctx, l, l.LocalAddr, remoteAddr, proxy)
}
