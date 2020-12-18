package command

import (
	"context"
	"fmt"
	"net"
	"net/url"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/commandproxy"
)

// COMMAND cmd:shell
func COMMAND(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {
	uri, err := url.Parse(cmd)
	if err != nil {
		return nil, err
	}

	scmd, err := commandproxy.SplitCommand(uri.Opaque)
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

	dp := commandproxy.DialProxyCommand(scmd)
	return bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
		proxy, err := dp.Format(network, address)
		if err != nil {
			return nil, err
		}
		c, err := commandDialer.CommandDialContext(ctx, proxy[0], proxy[1:]...)
		if err != nil {
			return nil, err
		}

		wc := &warpConnAddress{
			Conn: c,
			localAddr: &net.TCPAddr{
				IP: localIP,
			},
		}
		remoteAddr, err := net.ResolveTCPAddr(network, address)
		if err == nil {
			wc.remoteAddr = remoteAddr
		} else {
			wc.remoteAddr = &net.TCPAddr{}
		}
		return wc, nil
	}), nil
}

var localIP = net.ParseIP("127.0.0.1")

type warpConnAddress struct {
	net.Conn
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (w *warpConnAddress) LocalAddr() net.Addr {
	return w.localAddr
}

func (w *warpConnAddress) RemoteAddr() net.Addr {
	return w.remoteAddr
}
