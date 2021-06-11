package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"
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
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/notify"
)

var (
	ctx      = context.Background()
	configs  []string
	toConfig bool
	listens  []string
	dials    []string
	dump     bool
)

const defaults = `Bridge is a TCP proxy tool Support http(s)-connect socks4/4a/5/5h ssh proxycommand
More information, please go to https://github.com/wzshiming/bridge

Usage: bridge [-f path/to/config] [-t] [-d] \
	[-b=[[(tcp://|unix://)]bind_address]:bind_port \
	[-b=ssh://bridge_bind_address:bridge_bind_port [-b=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_bind_address:bridge_bind_port ...]]] \ // 
	-p=([(tcp://|unix://)]proxy_address:proxy_port|-) \
	[-p=(socks4://|socks4a://|socks5://|socks5h://|https://|http://|ssh://|cmd:)bridge_proxy_address:bridge_proxy_port ...]
`

func init() {
	flag.StringSliceVarP(&configs, "config", "c", nil, "load from config and ignore --bind and --proxy")
	flag.BoolVarP(&toConfig, "to-config", "t", false, "args to config")
	flag.StringSliceVarP(&listens, "bind", "b", nil, "The first is the listening address, and then the proxy through which the listening address passes.\nIf it is not filled in, it is redirected to the pipeline.\nonly ssh and local support listening, so the last proxy must be ssh.")
	flag.StringSliceVarP(&dials, "proxy", "p", nil, "The first is the dial-up address, followed by the proxy through which the dial-up address passes.")
	flag.BoolVarP(&dump, "debug", "d", dump, "Output the communication data.")
	flag.Parse()

	var cancel func()
	ctx, cancel = context.WithCancel(context.Background())
	notify.On(cancel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGKILL)
}

func printDefaults() {
	fmt.Fprintf(os.Stderr, defaults)
	flag.PrintDefaults()
}

func main() {
	var tasks []config.Chain
	var err error
	if len(configs) != 0 {
		tasks, err = config.LoadConfig(configs...)
		if err != nil {
			printDefaults()
			log.Println(err)
			return
		}
	} else {
		tasks, err = config.LoadConfigWithArgs(listens, dials)
		if err != nil {
			printDefaults()
			log.Println(err)
			return
		}
	}

	if toConfig {
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		encoder.Encode(config.Config{
			Chains: tasks,
		})
		return
	}

	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		go func(task config.Chain) {
			defer wg.Done()
			log.Println(chain.ShowChainWithConfig(task))
			err := chain.BridgeWithConfig(ctx, task, dump)
			if err != nil {
				log.Println(err)
			}
		}(task)
	}
	wg.Wait()
	return
}
