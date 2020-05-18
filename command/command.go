package command

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/utils"
	"github.com/wzshiming/commandproxy"
)

// COMMAND cmd:shell
func COMMAND(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {

	uri, err := url.Parse(cmd)
	if err != nil {
		return nil, err
	}

	scmd, err := utils.SplitCommand(uri.Opaque)
	if err != nil {
		return nil, err
	}

	var commandDialer bridge.CommandDialer = bridge.CommandDialFunc(func(ctx context.Context, name string, args ...string) (c net.Conn, err error) {
		return commandproxy.ProxyCommand(ctx, name, args...).Stdio()
	})
	if dialer != nil {
		cd, ok := dialer.(bridge.CommandDialer)
		if !ok || commandDialer == nil {
			return nil, fmt.Errorf("cmd must be the first agent or after the agent that can execute cmd, such as ssh")
		}
		commandDialer = cd
	}

	return bridge.DialFunc(func(ctx context.Context, network, addr string) (c net.Conn, err error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		cc := make([]string, len(scmd))
		copy(cc, scmd)
		for i := 0; i != len(cc); i++ {
			cc[i] = strings.ReplaceAll(cc[i], "%h", host)
			cc[i] = strings.ReplaceAll(cc[i], "%p", port)
		}
		return commandDialer.CommandDialContext(ctx, cc[0], cc[1:]...)
	}), nil
}
