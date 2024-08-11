package permuteproxy

import (
	"context"

	"github.com/wzshiming/permuteproxy"
	"github.com/wzshiming/permuteproxy/protocols/local"

	_ "github.com/wzshiming/permuteproxy/protocols/httpproxy"
	_ "github.com/wzshiming/permuteproxy/protocols/local"
	_ "github.com/wzshiming/permuteproxy/protocols/shadowsocks"
	_ "github.com/wzshiming/permuteproxy/protocols/snappy"
	_ "github.com/wzshiming/permuteproxy/protocols/socks4"
	_ "github.com/wzshiming/permuteproxy/protocols/socks5"
	_ "github.com/wzshiming/permuteproxy/protocols/sshproxy"
	_ "github.com/wzshiming/permuteproxy/protocols/tls"

	"github.com/wzshiming/bridge"
)

func PermuteProxy(ctx context.Context, dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	l := &permuteproxy.Proxy{
		Dialer: local.LOCAL,
	}
	if dialer != nil {
		l.Dialer = dialer
	}
	return l.NewDialer(addr)
}
