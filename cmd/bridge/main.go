package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	flag "github.com/spf13/pflag"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	_ "github.com/wzshiming/bridge/connect"
	_ "github.com/wzshiming/bridge/socks"
	_ "github.com/wzshiming/bridge/socks5"
	_ "github.com/wzshiming/bridge/ssh"
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

	showChain(dials, listens)

	var (
		bialer       bridge.Dialer       = &net.Dialer{}
		listenConfig bridge.ListenConfig = &net.ListenConfig{}
	)

	var dumper io.Writer
	if dump {
		dumper = &syncWriter{w: hex.Dumper(os.Stderr)}
	}

	dial := dials[0]
	dials = dials[1:]
	if len(dials) != 0 {
		d, _, err := chain.Default.BridgeChain(nil, dials...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		bialer = d
	}

	listen := ""
	if len(listens) != 0 {
		listen = resolveAddr(listens[0])
		listens = listens[1:]
		if len(listens) != 0 {
			_, l, err := chain.Default.BridgeChain(nil, listens...)
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				return
			}
			if l == nil {
				fmt.Fprintln(os.Stderr, "The last proxy could not listen")
				return
			}
			listenConfig = l
		}
	}

	if listen == "" {
		connect(context.Background(), struct {
			io.ReadCloser
			io.Writer
		}{ioutil.NopCloser(os.Stdin), os.Stdout}, bialer, dial, dumper)
	} else {
		listener, err := listenConfig.Listen(context.Background(), "tcp", listen)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		for {
			raw, err := listener.Accept()
			if err != nil {
				fmt.Fprintln(os.Stderr, err.Error())
				return
			}

			go connect(context.Background(), raw, bialer, dial, dumper)
		}
	}
}

func connect(ctx context.Context, raw io.ReadWriteCloser, bri bridge.Dialer, target string, dumper io.Writer) {
	conn, err := bri.DialContext(ctx, "tcp", target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}
	defer raw.Close()
	defer conn.Close()
	if dumper != nil {
		go io.Copy(conn, io.TeeReader(raw, dumper))
		io.Copy(raw, io.TeeReader(conn, dumper))
	} else {
		go io.Copy(conn, raw)
		io.Copy(raw, conn)
	}
}

// The asynchronous output is locked only for debug with no performance considerations
type syncWriter struct {
	w io.Writer
	sync.Mutex
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	s.Lock()
	defer s.Unlock()
	return s.w.Write(p)
}

func resolveAddr(addr string) string {
	a, err := net.ResolveTCPAddr("tcp", addr)
	if err != nil {
		return addr
	}
	if len(a.IP) == 0 {
		a.IP = net.IP{0, 0, 0, 0}
		return a.String()
	}
	return addr
}

func showChain(dials, listens []string) {
	dials = removeUserInfo(dials)
	listens = reverse(removeUserInfo(listens))

	if len(listens) == 0 {
		fmt.Fprintln(os.Stderr, "Bridge: DIAL", strings.Join(dials, " <- "), "<- LOCAL <- STDIO")
	} else {
		fmt.Fprintln(os.Stderr, "Bridge: DIAL", strings.Join(dials, " <- "), "<- LOCAL <-", strings.Join(listens, " <- "), "LISTEN")
	}
	return
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
