package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"syscall"
	"time"

	_ "github.com/wzshiming/bridge/protocols/command"
	_ "github.com/wzshiming/bridge/protocols/connect"
	_ "github.com/wzshiming/bridge/protocols/emux"
	_ "github.com/wzshiming/bridge/protocols/netcat"
	_ "github.com/wzshiming/bridge/protocols/permuteproxy"
	_ "github.com/wzshiming/bridge/protocols/shadowsocks"
	_ "github.com/wzshiming/bridge/protocols/snappy"
	_ "github.com/wzshiming/bridge/protocols/socks4"
	_ "github.com/wzshiming/bridge/protocols/socks5"
	_ "github.com/wzshiming/bridge/protocols/ssh"
	_ "github.com/wzshiming/bridge/protocols/tls"

	_ "github.com/wzshiming/anyproxy/pprof"
	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	_ "github.com/wzshiming/anyproxy/proxies/shadowsocks"
	_ "github.com/wzshiming/anyproxy/proxies/socks4"
	_ "github.com/wzshiming/anyproxy/proxies/socks5"
	_ "github.com/wzshiming/anyproxy/proxies/sshproxy"

	flag "github.com/spf13/pflag"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/logger"
	"github.com/wzshiming/notify"
)

var (
	ctx, globalCancel = context.WithCancel(context.Background())
	allow             []string
	configs           []string
	toConfig          bool
	listens           []string
	idleTimeout       time.Duration
	dials             []string
	dump              bool
	pprofAddress      string
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
	flag.StringSliceVar(&allow, "allow", nil, "The allow of remote addresses.")
	flag.DurationVar(&idleTimeout, "idle-timeout", 0, "The idle timeout for connections.")
	flag.StringVar(&pprofAddress, "pprof", "", "The pprof address.")
	flag.BoolVarP(&dump, "debug", "d", dump, "Output the communication data.")
	flag.Parse()

	signals := []os.Signal{syscall.SIGINT, syscall.SIGTERM}
	notify.OnceSlice(signals, func() {
		globalCancel()
		logger.Std.Info("Wait for the existing task to complete, and exit directly if the signal occurs again")
		notify.OnceSlice(signals, func() {
			os.Exit(1)
		})
	})
}

func printDefaults() {
	fmt.Fprintf(os.Stderr, defaults)
	flag.PrintDefaults()
}

func main() {
	if pprofAddress != "" {
		go func() {
			err := http.ListenAndServe(pprofAddress, http.DefaultServeMux)
			if err != nil {
				logger.Std.Error("ListenAndServe", "err", err)
			}
		}()
	}
	var tasks []config.Chain
	var err error
	if len(configs) != 0 {
		tasks, err = config.LoadConfig(configs...)
		if err != nil {
			printDefaults()
			logger.Std.Error("LoadConfig", "err", err)
			return
		}
	} else {
		tasks, err = config.LoadConfigWithArgs(listens, dials)
		if err != nil {
			printDefaults()
			logger.Std.Error("LoadConfigWithArgs", "err", err)
			return
		}
		if len(allow) > 0 {
			for i := range tasks {
				tasks[i].Allow = allow
			}
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

	if len(configs) != 0 {
		runWithReload(ctx, logger.Std, tasks, configs)
	} else {
		run(ctx, logger.Std, tasks)
	}
	return
}

func run(ctx context.Context, log *slog.Logger, tasks []config.Chain) {
	var wg sync.WaitGroup
	wg.Add(len(tasks))
	for _, task := range tasks {
		if task.IdleTimeout == 0 {
			task.IdleTimeout = idleTimeout
		}
		go func(task config.Chain) {
			defer wg.Done()
			log.Info(chain.ShowChainWithConfig(task))
			b := chain.NewBridge(log, dump)
			err := b.BridgeWithConfig(ctx, task)
			if err != nil {
				log.Error("BridgeWithConfig", "err", err)
			}
		}(task)
	}
	wg.Wait()
}
