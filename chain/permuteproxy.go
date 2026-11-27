package chain

import (
	"context"
	"fmt"
	"net"

	"github.com/wzshiming/permuteproxy"

	"github.com/wzshiming/bridge"
)

// permuteProxy crosses one hop over dialer. The hop is a permuteproxy URI that is a client:
//
//	socks5://[username:password@]{address}  a proxy, optionally over layers as in socks5+tls://
//	cmd:{command}                           a command run per connection, %h and %p filled with its host and port
//	nc:[prefix]                             netcat run per connection, behind prefix as in nc:ssh host
func permuteProxy(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = Local
	}
	engine := permuteproxy.ClientEngine{Base: base{dialer}}
	// A dialer that cannot bind datagram sockets leaves a client's own to the system.
	if _, ok := dialer.(bridge.ListenPacketConfig); ok {
		engine.Local = engine.Base
	}
	c, err := permuteproxy.ParseURI(addr)
	if err != nil {
		return nil, err
	}
	return engine.Client(c)
}

// base offers a bridge dialer as the network a chain is built over.
type base struct {
	dialer bridge.Dialer
}

func (b base) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return b.dialer.DialContext(ctx, network, address)
}

func (b base) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	l, ok := b.dialer.(bridge.ListenConfig)
	if !ok {
		return nil, fmt.Errorf("%T could not listen: %w", b.dialer, permuteproxy.ErrUnsupported)
	}
	return l.Listen(ctx, network, address)
}

func (b base) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	l, ok := b.dialer.(bridge.ListenPacketConfig)
	if !ok {
		return nil, fmt.Errorf("%T could not listen packet: %w", b.dialer, permuteproxy.ErrUnsupported)
	}
	return l.ListenPacket(ctx, network, address)
}
