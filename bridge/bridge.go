package bridge

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/internal/dump"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/internal/proxy"
	"github.com/wzshiming/bridge/local"
	"github.com/wzshiming/commandproxy"
)

var ctx = context.Background()

func Bridge(listens, dials []string, d bool) error {
	log.Println(showChain(dials, listens))

	var (
		dialer       bridge.Dialer       = local.LOCAL
		listenConfig bridge.ListenConfig = local.LOCAL
	)

	dial := dials[0]
	dials = dials[1:]
	if len(dials) != 0 {
		b, err := chain.Default.BridgeChain(nil, dials...)
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
		step(ctx, dialer, raw, dial)
	} else {
		network, listen := resolveProtocol(listens[0])
		listens = listens[1:]

		if len(listens) != 0 {
			d, err := chain.Default.BridgeChain(nil, listens...)
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
			for {
				raw, err := listener.Accept()
				if err != nil {
					return err
				}
				from := raw.RemoteAddr().String()
				svc := proxy.NewProxy(bridge.DialFunc(func(ctx context.Context, network, address string) (c net.Conn, err error) {
					c, err = dialer.DialContext(ctx, network, address)
					if d {
						c = dump.NewDumpConn(c, false, from, address)
					}
					return c, err
				}))
				go svc.ServeConn(raw)
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
				go step(ctx, dialer, raw, dial)
			}
		}
	}
	return nil
}

func step(ctx context.Context, dialer bridge.Dialer, raw io.ReadWriteCloser, addr string) {
	defer raw.Close()
	network, address := resolveProtocol(addr)
	conn, err := dialer.DialContext(ctx, network, address)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	err = commandproxy.Tunnel(ctx, conn, raw)
	if err != nil {
		log.Println(err)
	}
}

func resolveProtocol(addr string) (network, address string) {
	network = "tcp"
	u, err := url.Parse(addr)
	if err != nil {
		return network, addr
	}
	if u.Host == "" {
		return network, addr
	}
	address = u.Host
	if u.Scheme != "" {
		network = u.Scheme
	}
	return network, address
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
		u, err := url.Parse(s[i])
		if err != nil {
			continue
		}

		changeFlag := false
		if u.User != nil {
			u.User = nil
			changeFlag = true
		}
		if u.ForceQuery {
			u.ForceQuery = false
			changeFlag = true
		}
		if u.RawQuery != "" {
			u.RawQuery = ""
			changeFlag = true
		}

		if changeFlag {
			s[i] = u.String()
		}
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
