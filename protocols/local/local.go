package local

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/netutils"
	"github.com/wzshiming/commandproxy"
)

var LOCAL = &Local{
	Dialer:       &net.Dialer{},
	ListenConfig: &net.ListenConfig{},
	LocalAddr:    netutils.NewNetAddr("local", "local"),
}

type Local struct {
	Dialer       bridge.Dialer
	ListenConfig bridge.ListenConfig
	LocalAddr    net.Addr
}

func (l *Local) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	slog.Debug("Dial", "network", network, "address", address)
	return l.Dialer.DialContext(ctx, network, address)
}

func (l *Local) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	slog.Debug("Listen", "network", network, "address", address)
	return l.ListenConfig.Listen(ctx, network, address)
}

func (l *Local) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	slog.Debug("CommandDial", "name", name, "args", args)
	proxy := commandproxy.ProxyCommand(ctx, name, args...)
	proxy.Stderr = os.Stderr
	conn, err := proxy.Stdio()
	if err != nil {
		return nil, err
	}
	remoteAddr := netutils.NewNetAddr("cmd", strings.Join(append([]string{name}, args...), " "))
	conn = netutils.ConnWithAddr(conn, l.LocalAddr, remoteAddr)
	return conn, nil
}

func (l *Local) CommandListen(ctx context.Context, name string, args ...string) (net.Listener, error) {
	slog.Debug("CommandListen", "name", name, "args", args)
	proxy := append([]string{name}, args...)
	remoteAddr := netutils.NewNetAddr("cmd", strings.Join(proxy, " "))
	return netutils.NewCommandListener(ctx, l, l.LocalAddr, remoteAddr, proxy)
}
