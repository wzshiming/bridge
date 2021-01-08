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

	"github.com/wzshiming/anyproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/internal/pool"
	"github.com/wzshiming/bridge/internal/scheme"
	"github.com/wzshiming/bridge/local"
	"github.com/wzshiming/commandproxy"
)

func Bridge(ctx context.Context, listens, dials []string, d bool) error {
	log.Println(showChain(dials, listens))

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
	} else {
		network, listen, ok := scheme.SplitSchemeAddr(listens[0])
		if !ok {
			return fmt.Errorf("unsupported protocol format %q", listens[0])
		}
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

		listener, err := listenConfig.Listen(ctx, network, listen)
		if err != nil {
			return err
		}

		if dial == "-" {
			if d {
				for {
					raw, err := listener.Accept()
					if err != nil {
						return err
					}
					from := raw.RemoteAddr().String()
					svc := anyproxy.NewAnyProxy(ctx, bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
						c, err = dialer.DialContext(ctx, network, address)
						if err != nil {
							return nil, err
						}
						return dump.NewDumpConn(c, false, from, address), nil
					}), log.Std, pool.Bytes)
					go svc.ServeConn(raw)
				}
			} else {
				svc := anyproxy.NewAnyProxy(ctx, dialer, log.Std, pool.Bytes)
				for {
					raw, err := listener.Accept()
					if err != nil {
						return err
					}
					go svc.ServeConn(raw)
				}
			}
		} else {
			for {
				raw, err := listener.Accept()
				if err != nil {
					return err
				}
				if d {
					raw = dump.NewDumpConn(raw, true, raw.RemoteAddr().String(), dial)
				}
				go stepIgnoreErr(ctx, dialer, raw, dial)
			}
		}
	}
}

func stepIgnoreErr(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, addr string) {
	err := step(ctx, dialer, raw, addr)
	if err != nil {
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

func showChain(dials, listens []string) string {
	dials = removeUserInfo(dials)
	listens = reverse(removeUserInfo(listens))

	if len(listens) == 0 {
		return fmt.Sprintln("DIAL", strings.Join(dials, " <- "), "<- LOCAL <- STDIO")
	}
	return fmt.Sprintln("DIAL", strings.Join(dials, " <- "), "<- LOCAL <-", strings.Join(listens, " <- "), "LISTEN")
}

func removeUserInfo(s []string) []string {
	s = stringsClone(s)
	for i := 0; i != len(s); i++ {
		sch, addr, ok := scheme.SplitSchemeAddr(s[i])
		if !ok {
			continue
		}
		p, ok := scheme.JoinSchemeAddr(sch, addr)
		if !ok {
			continue
		}
		s[i] = p
	}
	for i := 0; i != len(s); i++ {
		s[i] = strconv.Quote(s[i])
	}
	return s
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
