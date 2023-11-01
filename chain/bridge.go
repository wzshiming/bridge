package chain

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/netutils"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/logger"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
)

type Bridge struct {
	logger logr.Logger
	dump   bool
	chain  *BridgeChain
}

func NewBridge(logger logr.Logger, dump bool) *Bridge {
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
		d, err := b.chain.BridgeChainWithConfig(local.LOCAL, dials...)
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
			ReadCloser: ioutil.NopCloser(os.Stdin),
			Writer:     os.Stdout,
		}

		if b.dump {
			raw = dump.NewDumpReadWriteCloser(raw, true, "STDIO", strings.Join(dial.LB, "|"))
		}

		return step(ctx, dialer, raw, dial.LB)
	}

	listen := config.Bind[0]
	listens := config.Bind[1:]

	if len(listens) != 0 {
		d, err := b.chain.BridgeChainWithConfig(local.LOCAL, listens...)
		if err != nil {
			return err
		}
		l, ok := d.(bridge.ListenConfig)
		if !ok || l == nil {
			return fmt.Errorf("the last proxy could not listen")
		}
		listenConfig = l
	}

	if len(dial.LB) != 0 && dial.LB[0] == "-" {
		return b.bridgeProxy(ctx, listenConfig, dialer, listen.LB)
	} else {
		return b.bridgeStream(ctx, listenConfig, dialer, listen.LB, dial.LB)
	}
}

func (b *Bridge) Bridge(ctx context.Context, listens, dials []string) error {
	conf, err := config.LoadConfigWithArgs(listens, dials)
	if err != nil {
		return err
	}
	return b.BridgeWithConfig(ctx, conf[0])
}

func (b *Bridge) bridgeStream(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string, dials []string) error {
	wg := sync.WaitGroup{}
	listeners := make([]net.Listener, 0, len(listens))
	for _, l := range listens {
		network, listen, ok := scheme.SplitSchemeAddr(l)
		if !ok {
			err := fmt.Errorf("unsupported protocol format %q", l)
			b.logger.Error(err, "SplitSchemeAddr")
			return err
		}
		listener, err := netutils.Listen(ctx, listenConfig, network, listen)
		if err != nil {
			b.logger.Error(err, "Listen")
			return err
		}
		listeners = append(listeners, listener)
	}

	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			b.logger.Info("Close all listeners")
			for _, listener := range listeners {
				if listener == nil {
					continue
				}
				listener.Close()
			}
		}()
	}

	wg.Add(len(listens))
	for i, l := range listens {
		go func(i int, l string) {
			defer func() {
				b.logger.Info("Close listener", "listen", l)
				wg.Done()
			}()
			listener := listeners[i]

			backoff := time.Second / 10
		loop:
			for ctx.Err() == nil {
				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						b.logger.Error(err, "Accept")
					}

					for ctx.Err() == nil {
						backoff <<= 1
						if backoff > time.Second*30 {
							backoff = time.Second * 30
						}
						b.logger.Info("Relisten", "backoff", backoff)
						time.Sleep(backoff)

						network, listen, ok := scheme.SplitSchemeAddr(l)
						if !ok {
							b.logger.Error(fmt.Errorf("unsupported protocol format %q", l), "")
							return
						}
						listener, err = netutils.Listen(ctx, listenConfig, network, listen)
						if err == nil {
							listeners[i] = listener
							continue loop
						}
						b.logger.Error(err, "Relisten")
					}
					return
				}
				if b.dump {
					raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), strings.Join(dials, "|"))
				}
				backoff = time.Second / 10
				b.logger.V(1).Info("Connect", "remote_address", raw.RemoteAddr().String())
				go b.stepIgnoreErr(ctx, dialer, raw, dials)
			}
		}(i, l)
	}
	wg.Wait()
	return nil
}

func (b *Bridge) bridgeProxy(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string) error {
	wg := sync.WaitGroup{}
	svc, err := anyproxy.NewAnyProxy(ctx, listens, &anyproxy.Config{
		Dialer:       dialer,
		ListenConfig: listenConfig,
		Logger:       logger.Wrap(b.logger, "anyproxy"),
		BytesPool:    pool.Bytes,
	})
	if err != nil {
		return err
	}
	hosts := svc.Hosts()

	listeners := make([]net.Listener, 0, len(listens))
	for _, host := range hosts {
		listener, err := netutils.Listen(ctx, listenConfig, "tcp", host)
		if err != nil {
			b.logger.Error(err, "Listen")
			return err
		}
		listeners = append(listeners, listener)
	}

	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			b.logger.Info("Close all listeners")
			for _, listener := range listeners {
				if listener == nil {
					continue
				}
				listener.Close()
			}
		}()
	}

	wg.Add(len(hosts))
	for i, host := range hosts {
		go func(i int, host string) {
			defer func() {
				b.logger.Info("Close listener", "listen", host)
				wg.Done()
			}()

			listener := listeners[i]
			h := svc.Match(host)

			backoff := time.Second / 10
		loop:
			for ctx.Err() == nil {
				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						b.logger.Error(err, "Accept")
					}
					for ctx.Err() == nil {
						backoff <<= 1
						if backoff > time.Second*30 {
							backoff = time.Second * 30
						}
						b.logger.Info("Relisten", "backoff", backoff)
						time.Sleep(backoff)

						listener, err = netutils.Listen(ctx, listenConfig, "tcp", host)
						if err == nil {
							listeners[i] = listener
							continue loop
						}
						b.logger.Error(err, "Relisten")
					}
					return
				}
				h := h
				if b.dump {
					// In dubug mode, need to know the address of the client.
					// Because it is debug, performance is not considered here.
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
						b.logger.Error(err, "NewAnyProxy")
						return
					}
					h = svc.Match(host)
				}
				backoff = time.Second / 10
				b.logger.V(1).Info("Connect", "remote_address", raw.RemoteAddr().String())
				go h.ServeConn(raw)
			}
		}(i, host)
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
		b.logger.Error(err, "Step")
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
