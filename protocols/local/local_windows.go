package local

import (
	"context"
	"net"

	winio "github.com/Microsoft/go-winio"
	"github.com/wzshiming/bridge"
)

const (
	pipeName = "pipe"
)

func init() {
	LOCAL.Dialer = &pipeDialer{LOCAL.Dialer}
	LOCAL.ListenConfig = &pipeListenConfig{LOCAL.ListenConfig}
}

type pipeListenConfig struct {
	bridge.ListenConfig
}

func (p *pipeListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if network != pipeName {
		return p.ListenConfig.Listen(ctx, network, address)
	}
	var conf winio.PipeConfig
	return winio.ListenPipe(address, &conf)
}

type pipeDialer struct {
	bridge.Dialer
}

func (p *pipeDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if network != pipeName {
		return p.Dialer.DialContext(ctx, network, address)
	}
	return winio.DialPipeContext(ctx, address)
}
