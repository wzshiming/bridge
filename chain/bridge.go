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
	"github.com/wzshiming/bridge/internal/common"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/logger"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
)

func BridgeWithConfig(ctx context.Context, log logr.Logger, chain config.Chain, d bool) error {
	var (
		dialer       bridge.Dialer       = local.LOCAL
		listenConfig bridge.ListenConfig = local.LOCAL
	)
	dial := chain.Proxy[0]
	dials := chain.Proxy[1:]

	if len(dials) != 0 {
		d, err := Default.BridgeChainWithConfig(local.LOCAL, dials...)
		if err != nil {
			return err
		}
		dialer = d
	}

	// No listener is set, use stdio.
	if len(chain.Bind) == 0 {
		var raw io.ReadWriteCloser = struct {
			io.ReadCloser
			io.Writer
		}{
			ReadCloser: ioutil.NopCloser(os.Stdin),
			Writer:     os.Stdout,
		}

		if d {
			raw = dump.NewDumpReadWriteCloser(raw, true, "STDIO", strings.Join(dial.LB, "|"))
		}

		return step(ctx, dialer, raw, dial.LB)
	}

	listen := chain.Bind[0]
	listens := chain.Bind[1:]

	if len(listens) != 0 {
		d, err := Default.BridgeChainWithConfig(local.LOCAL, listens...)
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
		return bridgeProxy(ctx, log, listenConfig, dialer, listen.LB, d)
	} else {
		return bridgeTCP(ctx, log, listenConfig, dialer, listen.LB, dial.LB, d)
	}
}

func Bridge(ctx context.Context, log logr.Logger, listens, dials []string, d bool) error {
	chain, err := config.LoadConfigWithArgs(listens, dials)
	if err != nil {
		return err
	}
	return BridgeWithConfig(ctx, log, chain[0], d)
}

func bridgeTCP(ctx context.Context, log logr.Logger, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string, dials []string, d bool) error {
	wg := sync.WaitGroup{}
	listeners := make([]net.Listener, 0, len(listens))
	for _, l := range listens {
		network, listen, ok := scheme.SplitSchemeAddr(l)
		if !ok {
			log.Error(fmt.Errorf("unsupported protocol format %q", l), "")
			return fmt.Errorf("unsupported protocol format %q", l)
		}
		listener, err := common.Listen(ctx, listenConfig, network, listen)
		if err != nil {
			log.Error(err, "Listen")
			return err
		}
		listeners = append(listeners, listener)
	}

	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			log.Info("Close all listeners")
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
			defer wg.Done()
			listener := listeners[i]

			backoff := time.Second / 10
		loop:
			for ctx.Err() == nil {
				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						log.Error(err, "Accept")
					}

					for ctx.Err() == nil {
						backoff <<= 1
						if backoff > time.Second*30 {
							backoff = time.Second * 30
						}
						log.Info("Relisten", "backoff", backoff)
						time.Sleep(backoff)

						network, listen, ok := scheme.SplitSchemeAddr(l)
						if !ok {
							log.Error(fmt.Errorf("unsupported protocol format %q", l), "")
							return
						}
						listener, err = common.Listen(ctx, listenConfig, network, listen)
						if err == nil {
							listeners[i] = listener
							continue loop
						}
						log.Error(err, "Relisten")
					}
					return
				}
				if d {
					raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), strings.Join(dials, "|"))
				}
				backoff = time.Second / 10
				log.V(1).Info("Connect", "remote_address", raw.RemoteAddr().String())
				go stepIgnoreErr(ctx, log, dialer, raw, dials)
			}
		}(i, l)
	}
	wg.Wait()
	return nil
}

func bridgeProxy(ctx context.Context, log logr.Logger, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string, d bool) error {
	wg := sync.WaitGroup{}
	svc, err := anyproxy.NewAnyProxy(ctx, listens, &anyproxy.Config{
		Dialer:       dialer,
		ListenConfig: listenConfig,
		Logger:       logger.Wrap(log, "anyproxy"),
		BytesPool:    pool.Bytes,
	})
	if err != nil {
		return err
	}
	hosts := svc.Hosts()

	listeners := make([]net.Listener, 0, len(listens))
	for _, host := range hosts {
		listener, err := common.Listen(ctx, listenConfig, "tcp", host)
		if err != nil {
			log.Error(err, "Listen")
			return err
		}
		listeners = append(listeners, listener)
	}
	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			log.Info("Close all listeners")
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
			defer wg.Done()
			listener := listeners[i]
			h := svc.Match(host)

			backoff := time.Second / 10
		loop:
			for ctx.Err() == nil {
				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						log.Error(err, "Accept")
					}
					for ctx.Err() == nil {
						backoff <<= 1
						if backoff > time.Second*30 {
							backoff = time.Second * 30
						}
						log.Info("Relisten", "backoff", backoff)
						time.Sleep(backoff)

						listener, err = common.Listen(ctx, listenConfig, "tcp", host)
						if err == nil {
							listeners[i] = listener
							continue loop
						}
						log.Error(err, "Relisten")
					}
					return
				}
				h := h
				if d {
					// In dubug mode, need to know the address of the client.
					// Because it is debug, performance is not considered here.
					dial := bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
						c, err = common.Dial(ctx, dialer, network, address)
						if err != nil {
							return nil, err
						}
						return dump.NewDumpConn(c, false, raw.RemoteAddr().String(), address), nil
					})
					svc, err := anyproxy.NewAnyProxy(ctx, listens, &anyproxy.Config{
						Dialer:       dial,
						ListenConfig: listenConfig,
						Logger:       logger.Wrap(log, "anyproxy"),
						BytesPool:    pool.Bytes,
					})
					if err != nil {
						log.Error(err, "NewAnyProxy")
						return
					}
					h = svc.Match(host)
				}
				backoff = time.Second / 10
				log.V(1).Info("Connect", "remote_address", raw.RemoteAddr().String())
				go h.ServeConn(raw)
			}
		}(i, host)
	}
	wg.Wait()
	return nil
}

func ignoreClosedErr(err error) error {
	if err != nil && err != io.EOF && err != io.ErrClosedPipe && !common.IsClosedConnError(err) {
		return err
	}
	return nil
}

func stepIgnoreErr(ctx context.Context, log logr.Logger, dialer bridge.Dialer, raw io.ReadWriteCloser, dials []string) {
	err := step(ctx, dialer, raw, dials)
	if ignoreClosedErr(err) != nil {
		log.Error(err, "Step")
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

	conn, err := common.Dial(ctx, dialer, network, address)
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

func ShowChainWithConfig(chain config.Chain) string {
	dials := make([]string, 0, len(chain.Proxy))
	listens := make([]string, 0, len(chain.Bind))
	for _, proxy := range chain.Proxy {
		dials = append(dials, strings.Join(proxy.LB, "|"))
	}
	for _, bind := range chain.Bind {
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
