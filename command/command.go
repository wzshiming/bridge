package command

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"net"
	"net/url"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/commandproxy"
)

// COMMAND cmd:shell
func COMMAND(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {

	uri, err := url.Parse(cmd)
	if err != nil {
		return nil, err
	}

	if dialer != nil {
		cd, ok := dialer.(CommandDialer)
		if !ok {
			return nil, fmt.Errorf("cmd must be the first agent or after the agent that can execute cmd, such as ssh")
		}

		return bridge.DialFunc(func(ctx context.Context, network, addr string) (c net.Conn, err error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, err
			}
			cmd := uri.Opaque
			cmd = strings.ReplaceAll(cmd, "%h", host)
			cmd = strings.ReplaceAll(cmd, "%p", port)
			return cd.CommandDialContext(ctx, cmd)
		}), nil
	}

	scmd, err := parseCmd(uri.Opaque)
	if err != nil {
		return nil, err
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
		return commandproxy.ProxyCommand(ctx, cc[0], cc[1:]...).Stdio()
	}), nil
}

func parseCmd(cmd string) ([]string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil, fmt.Errorf("cmd is empty")
	}
	r := csv.NewReader(bytes.NewBufferString(cmd))
	r.Comma = ' '
	r.TrimLeadingSpace = true
	line, err := r.Read()
	if err != nil {
		return nil, err
	}
	return line, nil
}

type CommandDialer interface {
	CommandDialContext(ctx context.Context, cmd string) (net.Conn, error)
}
