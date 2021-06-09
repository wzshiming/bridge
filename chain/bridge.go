package chain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/internal/common"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
)

func BridgeWithConfig(ctx context.Context, chain config.Chain, d bool) error {
	var (
		dialer       bridge.Dialer       = local.LOCAL
		listenConfig bridge.ListenConfig = local.LOCAL
	)
	dial := chain.Proxy[0]
	dials := chain.Proxy[1:]

	if len(dials) != 0 {
		b, err := Default.BridgeChainWithConfig(local.LOCAL, dials...)
		if err != nil {
			return err
		}
		dialer = b
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
			return errors.New("the last proxy could not listen")
		}
		listenConfig = l
	}

	if len(dial.LB) != 0 && dial.LB[0] == "-" {
		return bridgeProxy(ctx, listenConfig, dialer, listen.LB, d)
	} else {
		return bridgeTCP(ctx, listenConfig, dialer, listen.LB, dial.LB, d)
	}
}

func Bridge(ctx context.Context, listens, dials []string, d bool) error {
	chain, err := config.LoadConfigWithArgs(listens, dials)
	if err != nil {
		return err
	}
	return BridgeWithConfig(ctx, chain[0], d)
}

func bridgeTCP(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string, dials []string, d bool) error {
	listeners := []net.Listener{}
	for _, l := range listens {
		network, listen, ok := scheme.SplitSchemeAddr(l)
		if !ok {
			return fmt.Errorf("unsupported protocol format %q", l)
		}
		listener, err := listenConfig.Listen(ctx, network, listen)
		if err != nil {
			return err
		}
		listeners = append(listeners, listener)
	}
	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			for _, listener := range listeners {
				listener.Close()
			}
		}()
	}

	wg := sync.WaitGroup{}
	wg.Add(len(listeners))
	for _, listener := range listeners {
		go func(listener net.Listener) {
			defer wg.Done()
			for {

				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						log.Println(err)
					}
					return
				}
				if d {
					raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), strings.Join(dials, "|"))
				}
				go stepIgnoreErr(ctx, dialer, raw, dials)
			}
		}(listener)
	}
	wg.Wait()
	return nil
}

func bridgeProxy(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listens []string, d bool) error {
	svc, err := anyproxy.NewAnyProxy(ctx, listens, dialer, log.Std, pool.Bytes)
	if err != nil {
		return err
	}
	hosts := svc.Hosts()
	listeners := []net.Listener{}
	for _, listen := range hosts {
		listener, err := listenConfig.Listen(ctx, "tcp", listen)
		if err != nil {
			return err
		}
		listeners = append(listeners, listener)
	}
	if ctx != context.Background() {
		go func() {
			<-ctx.Done()
			for _, listener := range listeners {
				listener.Close()
			}
		}()
	}

	wg := sync.WaitGroup{}
	wg.Add(len(listeners))
	for i, listener := range listeners {
		go func(host string, listener net.Listener) {
			defer wg.Done()
			h := svc.Match(host)
			for {
				raw, err := listener.Accept()
				if err != nil {
					if ignoreClosedErr(err) != nil {
						log.Println(err)
					}
					return
				}
				h := h
				if d {
					// In dubug mode, need to know the address of the client.
					// Because it is debug, performance is not considered here.
					dial := bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
						c, err = dialer.DialContext(ctx, network, address)
						if err != nil {
							return nil, err
						}
						return dump.NewDumpConn(c, false, raw.RemoteAddr().String(), address), nil
					})
					svc, err := anyproxy.NewAnyProxy(ctx, listens, dial, log.Std, pool.Bytes)
					if err != nil {
						log.Println(err)
						return
					}
					h = svc.Match(host)
				}
				go h.ServeConn(raw)
			}
		}(hosts[i], listener)
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

func stepIgnoreErr(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, dials []string) {
	err := step(ctx, dialer, raw, dials)
	if ignoreClosedErr(err) != nil {
		log.Println(err)
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
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		return err
	}
	buf1 := pool.Bytes.Get()
	buf2 := pool.Bytes.Get()
	defer func() {
		pool.Bytes.Put(buf1)
		pool.Bytes.Put(buf2)
	}()
	return commandproxy.Tunnel(ctx, conn, raw, buf1, buf2)
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
