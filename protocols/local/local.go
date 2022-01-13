package local

import (
	"context"
	"net"
	"os"
	"strconv"
	"strings"
	"syscall"

	"github.com/go-logr/logr"
	"github.com/gogf/greuse"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/wrapping"
	"github.com/wzshiming/commandproxy"
)

var LOCAL = &Local{
	Dialer: &net.Dialer{},
	ListenConfig: &net.ListenConfig{
		Control: getControl(),
	},
	LocalAddr: wrapping.NewNetAddr("local", "local"),
}

func getControl() func(network, address string, c syscall.RawConn) error {
	enable, _ := strconv.ParseBool(os.Getenv("ADDRESS_REUSE"))
	if enable {
		return greuse.Control
	}
	return nil
}

type Local struct {
	Dialer       bridge.Dialer
	ListenConfig bridge.ListenConfig
	LocalAddr    net.Addr
}

func (l *Local) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	logr.FromContextOrDiscard(ctx).V(1).Info("Dial", "network", network, "address", address)
	return l.Dialer.DialContext(ctx, network, address)
}

func (l *Local) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	logr.FromContextOrDiscard(ctx).V(1).Info("Listen", "network", network, "address", address)
	return l.ListenConfig.Listen(ctx, network, address)
}

func (l *Local) CommandDialContext(ctx context.Context, name string, args ...string) (net.Conn, error) {
	logr.FromContextOrDiscard(ctx).V(1).Info("CommandDial", "name", name, "args", args)
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
	logr.FromContextOrDiscard(ctx).V(1).Info("CommandListen", "name", name, "args", args)
	proxy := append([]string{name}, args...)
	remoteAddr := wrapping.NewNetAddr("cmd", strings.Join(proxy, " "))
	return wrapping.NewCommandListener(ctx, l, l.LocalAddr, remoteAddr, proxy)
}
