package chain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/idle"
	"github.com/wzshiming/bridge/internal/netutils"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/logger"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
	"github.com/wzshiming/hostmatcher"
)

type Bridge struct {
	logger *slog.Logger
	dump   bool
	chain  *BridgeChain
}

func NewBridge(logger *slog.Logger, dump bool) *Bridge {
	return &Bridge{
		logger: logger,
		dump:   dump,
		chain:  Default,
	}
}

func (b *Bridge) BridgeWithConfig(ctx context.Context, config config.Chain) error {
	var (
		dialer       bridge.Dialer       = local.LOCAL
		listenConfig bridge.ListenConfig = local.LOCAL
	)
	dial := config.Proxy[0]
	dials := config.Proxy[1:]

	if len(dials) != 0 {
		d, err := b.chain.BridgeChainWithConfig(ctx, local.LOCAL, dials...)
		if err != nil {
			return err
		}
		dialer = d
	}

	// No listener is set, use stdio.
	if len(config.Bind) == 0 {
		var raw io.ReadWriteCloser = struct {
			io.ReadCloser
			io.Writer
		}{
			ReadCloser: io.NopCloser(os.Stdin),
			Writer:     os.Stdout,
		}

		if b.dump {
			raw = dump.NewDumpReadWriteCloser(raw, true, "STDIO", strings.Join(dial.LB, "|"))
		}

		return step(ctx, dialer, raw, dial.LB)
	}

	var allow hostmatcher.Matcher
	if len(config.Allow) != 0 {
		allow = hostmatcher.NewMatcher(config.Allow)
	}

	listen := config.Bind[0]
	listens := config.Bind[1:]

	if len(listens) != 0 {
		d, err := b.chain.BridgeChainWithConfig(ctx, local.LOCAL, listens...)
		if err != nil {
			return err
		}
		l, ok := d.(bridge.ListenConfig)
		if !ok || l == nil {
			return fmt.Errorf("the last proxy %T could not listen", d)
		}
		listenConfig = l
	}

	if len(dial.LB) != 0 && dial.LB[0] == "-" {
		return b.bridgeProxy(ctx, listenConfig, dialer, config.IdleTimeout, listen.LB, allow)
	} else {
		return b.bridgeStream(ctx, listenConfig, dialer, config.IdleTimeout, listen.LB, dial.LB, allow)
	}
}

func (b *Bridge) Bridge(ctx context.Context, listens, dials []string) error {
	conf, err := config.LoadConfigWithArgs(listens, dials)
	if err != nil {
		return err
	}
	return b.BridgeWithConfig(ctx, conf[0])
}

func (b *Bridge) bridgeStream(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, idleTimeout time.Duration, listens []string, dials []string, allow hostmatcher.Matcher) error {
	return b.serveListeners(ctx, listens, func(ctx context.Context, address string) (net.Listener, error) {
		network, listen, ok := scheme.SplitSchemeAddr(address)
		if !ok {
			return nil, fmt.Errorf("unsupported protocol format %q", address)
		}
		return netutils.Listen(ctx, listenConfig, network, listen)
	}, func(ctx context.Context, address string, listener net.Listener) error {
		for ctx.Err() == nil {
			raw, err := listener.Accept()
			if err != nil {
				return err
			}
			if allow != nil {
				host, _, err := net.SplitHostPort(raw.RemoteAddr().String())
				if err != nil {
					b.logger.Error("SplitHostPort", "err", err)
					raw.Close()
					continue
				}
				if !allow.Match(host) {
					b.logger.Warn("connection from remote address not in allow", "remote_addr", raw.RemoteAddr().String())
					raw.Close()
					continue
				}
			}
			if b.dump {
				raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), strings.Join(dials, "|"))
			}
			if idleTimeout != 0 {
				raw = idle.NewIdleConn(raw, idleTimeout)
			}
			go b.stepIgnoreErr(ctx, dialer, raw, dials)
		}
		return ctx.Err()
	})
}

func (b *Bridge) bridgeProxy(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, idleTimeout time.Duration, listens []string, allow hostmatcher.Matcher) error {
	svc, err := anyproxy.NewAnyProxy(ctx, listens, &anyproxy.Config{
		Dialer:       dialer,
		ListenConfig: listenConfig,
		Logger:       logger.Wrap(b.logger, "anyproxy"),
		BytesPool:    pool.Bytes,
	})
	if err != nil {
		return err
	}
	return b.serveListeners(ctx, svc.Hosts(), func(ctx context.Context, host string) (net.Listener, error) {
		return netutils.Listen(ctx, listenConfig, "tcp", host)
	}, func(ctx context.Context, host string, listener net.Listener) error {
		h := svc.Match(host)
		for ctx.Err() == nil {
			raw, err := listener.Accept()
			if err != nil {
				return err
			}
			if allow != nil {
				host, _, err := net.SplitHostPort(raw.RemoteAddr().String())
				if err != nil {
					b.logger.Error("SplitHostPort", "err", err)
					raw.Close()
					continue
				}
				if !allow.Match(host) {
					b.logger.Warn("connection from remote address not in allow", "remote_addr", raw.RemoteAddr().String())
					raw.Close()
					continue
				}
			}
			h := h
			if b.dump {
				dial := bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
					c, err = netutils.Dial(ctx, dialer, network, address)
					if err != nil {
						return nil, err
					}
					return dump.NewDumpConn(c, false, raw.RemoteAddr().String(), address), nil
				})
				svc, err := anyproxy.NewAnyProxy(ctx, listens, &anyproxy.Config{
					Dialer:       dial,
					ListenConfig: listenConfig,
					Logger:       logger.Wrap(b.logger, "anyproxy"),
					BytesPool:    pool.Bytes,
				})
				if err != nil {
					raw.Close()
					return err
				}
				h = svc.Match(host)
			}
			if idleTimeout != 0 {
				raw = idle.NewIdleConn(raw, idleTimeout)
			}
			go func() {
				stop := context.AfterFunc(ctx, func() { raw.Close() })
				defer stop()
				defer raw.Close()
				h.ServeConn(raw)
			}()
		}
		return ctx.Err()
	})
}

func (b *Bridge) serveListeners(ctx context.Context, addresses []string, listen func(context.Context, string) (net.Listener, error), serve func(context.Context, string, net.Listener) error) error {
	listeners := make([]net.Listener, 0, len(addresses))
	for _, address := range addresses {
		if ctx.Err() != nil {
			return nil
		}
		listener, err := listen(ctx, address)
		if listener != nil {
			stop := context.AfterFunc(ctx, func() { listener.Close() })
			defer stop()
			defer listener.Close()
		}
		if err != nil {
			if ctx.Err() != nil && errors.Is(err, ctx.Err()) {
				return nil
			}
			return err
		}
		if listener == nil {
			return errors.New("listen returned no listener")
		}
		listeners = append(listeners, listener)
	}
	wg := sync.WaitGroup{}
	for index, listener := range listeners {
		address := addresses[index]
		wg.Add(1)
		go func() {
			defer wg.Done()
			first := listener
			Serve(ctx, func(ctx context.Context) (net.Listener, error) {
				if first != nil {
					opened := first
					first = nil
					return opened, nil
				}
				return listen(ctx, address)
			}, func(ctx context.Context, listener net.Listener) error {
				return serve(ctx, address, listener)
			}, func(event Event) {
				if event.Err != nil {
					b.logger.Error("Relisten", "err", event.Err, "attempt", event.Attempt, "backoff", event.Backoff)
				} else {
					b.logger.Info("Listen", "address", event.Addr)
				}
			})
		}()
	}
	wg.Wait()
	return nil
}

func ignoreClosedErr(err error) error {
	if err != nil && err != io.EOF && err != io.ErrClosedPipe && !netutils.IsClosedConnError(err) {
		return err
	}
	return nil
}

func (b *Bridge) stepIgnoreErr(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, dials []string) {
	err := step(ctx, dialer, raw, dials)
	if ignoreClosedErr(err) != nil {
		b.logger.Error("Step", "err", err)
	}
}

func step(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, dials []string) error {
	defer raw.Close()

	dial := dials[0]
	if len(dials) > 1 {
		dial = dials[rand.Int()%len(dials)]
	}
	network, address, ok := scheme.SplitSchemeAddr(dial)
	if !ok {
		return fmt.Errorf("unsupported protocol format %q", address)
	}

	conn, err := netutils.Dial(ctx, dialer, network, address)
	if err != nil {
		return err
	}
	buf1 := pool.Bytes.Get()
	buf2 := pool.Bytes.Get()
	defer func() {
		pool.Bytes.Put(buf1)
		pool.Bytes.Put(buf2)
	}()
	return commandproxy.Tunnel(context.Background(), conn, raw, buf1, buf2)
}

func ShowChainWithConfig(config config.Chain) string {
	dials := make([]string, 0, len(config.Proxy))
	listens := make([]string, 0, len(config.Bind))
	for _, proxy := range config.Proxy {
		dials = append(dials, strings.Join(proxy.LB, "|"))
	}
	for _, bind := range config.Bind {
		listens = append(listens, strings.Join(bind.LB, "|"))
	}
	return ShowChain(dials, listens)
}

func ShowChain(dials, listens []string) string {
	dials = removeUserInfo(dials)
	listens = reverse(removeUserInfo(listens))

	if len(listens) == 0 {
		return fmt.Sprintln("DIAL", strings.Join(dials, " <- "), "<- LOCAL <- STDIO")
	}
	return fmt.Sprintln("DIAL", strings.Join(dials, " <- "), "<- LOCAL <-", strings.Join(listens, " <- "), "LISTEN")
}

func removeUserInfo(addresses []string) []string {
	addresses = stringsClone(addresses)
	for i := 0; i != len(addresses); i++ {
		address := strings.Split(addresses[i], "|")
		for j := 0; j != len(address); j++ {
			sch, addr, ok := scheme.SplitSchemeAddr(address[j])
			if !ok {
				continue
			}
			p, ok := scheme.JoinSchemeAddr(sch, addr)
			if !ok {
				continue
			}
			address[j] = p
		}
		addresses[i] = strings.Join(address, "|")
	}
	for i := 0; i != len(addresses); i++ {
		addresses[i] = strconv.Quote(addresses[i])
	}
	return addresses
}

func stringsClone(s []string) []string {
	n := make([]string, len(s))
	copy(n, s)
	return n
}

func reverse(s []string) []string {
	if len(s) < 2 {
		return s
	}
	for i := 0; i != len(s)/2; i++ {
		s[i], s[len(s)-1] = s[len(s)-1], s[i]
	}
	return s
}
