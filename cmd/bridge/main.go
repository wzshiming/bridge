package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"

	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/smux"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"
	_ "github.com/wzshiming/bridge/protocols/ws"

	flag "github.com/spf13/pflag"
	"github.com/wzshiming/bridge/chain/bridge"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/internal/scheme"
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
	notify.On(cancel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
}

func main() {
	if len(dials) > 0 && len(listens) > 0 && dials[0] == "-" {
		proxies := strings.Split(listens[0], "|")
		if len(proxies) == 1 {
			network, address, _ := scheme.SplitSchemeAddr(proxies[0])
			if network == "tcp" {
				proxies = []string{"http://" + address, "socks5://" + address, "socks4://" + address}
			}
		}
		listens[0] = strings.Join(proxies, "|")
	}

	log.Println(bridge.ShowChain(dials, listens))
	err := bridge.Bridge(ctx, listens, dials, dump)
	if err != nil {
		log.Println(err)
	}
}
