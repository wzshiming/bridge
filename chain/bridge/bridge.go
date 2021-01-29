package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/internal/common"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/multiple/proxy"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/commandproxy"
)

func Bridge(ctx context.Context, listens, dials []string, d bool) error {
	var (
		dialer       bridge.Dialer       = local.LOCAL
		listenConfig bridge.ListenConfig = local.LOCAL
	)
	dial := dials[0]
	dials = dials[1:]

	if len(dials) != 0 {
		b, err := chain.Default.BridgeChain(local.LOCAL, dials...)
		if err != nil {
			return err
		}
		dialer = b
	}

	// No listener is set, use stdio.
	if len(listens) == 0 {
		var raw io.ReadWriteCloser = struct {
			io.ReadCloser
			io.Writer
		}{
			ReadCloser: ioutil.NopCloser(os.Stdin),
			Writer:     os.Stdout,
		}

		if d {
			raw = dump.NewDumpReadWriteCloser(raw, true, "STDIO", dial)
		}
		return step(ctx, dialer, raw, dial)
	}

	listen := listens[0]
	listens = listens[1:]

	if len(listens) != 0 {
		d, err := chain.Default.BridgeChain(local.LOCAL, listens...)
		if err != nil {
			return err
		}
		l, ok := d.(bridge.ListenConfig)
		if !ok || l == nil {
			return errors.New("the last proxy could not listen")
		}
		listenConfig = l
	}

	if dial != "-" {
		return bridgeTCP(ctx, listenConfig, dialer, listen, dial, d)
	} else {
		return bridgeProxy(ctx, listenConfig, dialer, listen, d)
	}
}

func bridgeTCP(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listen string, dial string, d bool) error {
	listens := strings.Split(listen, "|")
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
					raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), dial)
				}
				go stepIgnoreErr(ctx, dialer, raw, dial)
			}
		}(listener)
	}
	wg.Wait()
	return nil
}

func bridgeProxy(ctx context.Context, listenConfig bridge.ListenConfig, dialer bridge.Dialer, listen string, d bool) error {
	listens := strings.Split(listen, "|")
	svc, err := proxy.NewProxy(ctx, listens, dialer)
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
					svc, err := proxy.NewProxy(ctx, listens, dial)
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

func stepIgnoreErr(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, addr string) {
	err := step(ctx, dialer, raw, addr)
	if ignoreClosedErr(err) != nil {
		log.Println(err)
	}
}

func step(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, addr string) error {
	defer raw.Close()
	network, address, ok := scheme.SplitSchemeAddr(addr)
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
