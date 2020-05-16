package main

import (
	"fmt"
	"os"

	_ "github.com/wzshiming/bridge/command"
	_ "github.com/wzshiming/bridge/connect"
	_ "github.com/wzshiming/bridge/socks"
	_ "github.com/wzshiming/bridge/socks5"
	_ "github.com/wzshiming/bridge/ssh"
	_ "github.com/wzshiming/bridge/tls"

	flag "github.com/spf13/pflag"
	"github.com/wzshiming/bridge/bridge"
	"github.com/wzshiming/bridge/internal/log"
)

var (
	listens []string
	dials   []string
	dump    bool
)

const defaults = `Bridge is a TCP proxy tool Support http(s)-connect socks4/4a/5/5h ssh
More information, please go to https://github.com/wzshiming/bridge

Usage: bridge [-d] \
	[-b=[bind_address]:bind_port \
	[-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks4|socks4a|socks5|socks5h|https|http|ssh)://bridge_bind_address:bridge_bind_port ...]]] \ // 
	-p=proxy_address:proxy_port \
	[-p=(socks4|socks4a|socks5|socks5h|https|http|ssh)://bridge_proxy_address:bridge_proxy_port ...]
`

func init() {
	flag.StringSliceVarP(&listens, "bind", "b", nil, "The first is the listening address, and then the proxy through which the listening address passes.\nIf it is not filled in, it is redirected to the pipeline.\nonly ssh and local support listening, so the last proxy must be ssh.")
	flag.StringSliceVarP(&dials, "proxy", "p", nil, "The first is the dial-up address, followed by the proxy through which the dial-up address passes.")
	flag.BoolVarP(&dump, "debug", "d", false, "Output the communication data.")
	flag.Parse()
}

func main() {
	if len(dials) < 1 {
		fmt.Fprintf(os.Stderr, defaults)
		flag.PrintDefaults()
		return
	}
	err := bridge.Bridge(listens, dials, dump)
	if err != nil {
		log.Fatalln(err)
		return
	}
}
