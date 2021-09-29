package common

import (
	"context"
	"fmt"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/commandproxy"
)

// IsClosedConnError reports whether err is an error from use of a closed
// network connection.
func IsClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	str := err.Error()
	if strings.Contains(str, "use of closed network connection") {
		return true
	}

	if runtime.GOOS == "windows" {
		if oe, ok := err.(*net.OpError); ok && oe.Op == "read" {
			if se, ok := oe.Err.(*os.SyscallError); ok && se.Syscall == "wsarecv" {
				const WSAECONNABORTED = 10053
				const WSAECONNRESET = 10054
				if n := errno(se.Err); n == WSAECONNRESET || n == WSAECONNABORTED {
					return true
				}
			}
		}
	}
	return false
}

func errno(v error) uintptr {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Uintptr {
		return uintptr(rv.Uint())
	}
	return 0
}

func Dial(ctx context.Context, dialer bridge.Dialer, network, address string) (net.Conn, error) {
	if network == "cmd" || network == "command" {
		d, ok := dialer.(bridge.CommandDialer)
		if !ok {
			return nil, fmt.Errorf("protocol %q unsupported cmd %q", network, address)
		}
		cmd, err := commandproxy.SplitCommand(address)
		if err != nil {
			return nil, err
		}
		return d.CommandDialContext(ctx, cmd[0], cmd[1:]...)
	}
	return dialer.DialContext(ctx, network, address)
}

func Listen(ctx context.Context, listener bridge.ListenConfig, network, address string) (net.Listener, error) {
	if network == "cmd" || network == "command" {
		l, ok := listener.(bridge.CommandListenConfig)
		if !ok {
			return nil, fmt.Errorf("protocol %q unsupported cmd %q", network, address)
		}
		cmd, err := commandproxy.SplitCommand(address)
		if err != nil {
			return nil, err
		}
		return l.CommandListen(ctx, cmd[0], cmd[1:]...)
	}
	return listener.Listen(ctx, network, address)
}
