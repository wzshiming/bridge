package main

import (
	"context"
	"fmt"
	"os"
	"syscall"

	flag "github.com/spf13/pflag"
	"github.com/wzshiming/bridge/bridge"
	_ "github.com/wzshiming/bridge/command"
	_ "github.com/wzshiming/bridge/connect"
	"github.com/wzshiming/bridge/internal/log"
	_ "github.com/wzshiming/bridge/netcat"
	_ "github.com/wzshiming/bridge/shadowsocks"
	_ "github.com/wzshiming/bridge/smux"
	_ "github.com/wzshiming/bridge/socks4"
	_ "github.com/wzshiming/bridge/socks5"
	_ "github.com/wzshiming/bridge/ssh"
	_ "github.com/wzshiming/bridge/tls"
	_ "github.com/wzshiming/bridge/ws"
	"github.com/wzshiming/notify"
)

var (
	ctx     = context.Background()
	listens []string
	dials   []string
	dump    bool
)

const defaults = `Bridge is a TCP proxy tool Support http(s)-connect socks4/4a/5/5h ssh proxycommand
More information, please go to https://github.com/wzshiming/bridge

Usage: bridge [-d] \
	[-b=[[(tcp://|unix://)]bind_address]:bind_port \
	[-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_bind_address:bridge_bind_port ...]]] \ // 
	-p=([(tcp://|unix://)]proxy_address:proxy_port|-) \
	[-p=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_proxy_address:bridge_proxy_port ...]
`

func init() {
	flag.StringSliceVarP(&listens, "bind", "b", nil, "The first is the listening address, and then the proxy through which the listening address passes.\nIf it is not filled in, it is redirected to the pipeline.\nonly ssh and local support listening, so the last proxy must be ssh.")
	flag.StringSliceVarP(&dials, "proxy", "p", nil, "The first is the dial-up address, followed by the proxy through which the dial-up address passes.")
	flag.BoolVarP(&dump, "debug", "d", dump, "Output the communication data.")
	flag.Parse()

	if len(dials) < 1 {
		fmt.Fprintf(os.Stderr, defaults)
		flag.PrintDefaults()
		os.Exit(1)
	}

	var cancel func()
	ctx, cancel = context.WithCancel(context.Background())
	notify.OnSlice([]os.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL}, cancel)
}

func main() {
	log.Println(bridge.ShowChain(dials, listens))
	err := bridge.Bridge(ctx, listens, dials, dump)
	if err != nil {
		log.Println(err)
	}
}
