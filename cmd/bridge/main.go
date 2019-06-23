package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

var addres string
var dump bool

const defaults = `usage: bridge [-d] [-a [bind_address]:bind_port] proxy_address:proxy_port
              [(socks5|socks4|socks4a|https|http||ssh)://bridge_address:bridge_port ..]
`

func init() {
	flag.StringVar(&addres, "a", "", "Pipe or tcp address, the default is pipe")
	flag.BoolVar(&dump, "d", false, "Output the communication data")
	flag.Parse()
}

func main() {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, defaults)
		flag.PrintDefaults()
		return
	}

	var bri bridge.Dialer = &net.Dialer{}

	target := args[0]

	args = args[1:]
	if len(args) != 0 {
		d, err := chain.Default.BridgeChain(nil, args...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		bri = d
	}

	var dumper io.Writer
	if dump {
		dumper = &syncWriter{w: hex.Dumper(os.Stderr)}
	}
	if addres == "" {
		connect(context.Background(), struct {
			io.ReadCloser
			io.Writer
		}{ioutil.NopCloser(os.Stdin), os.Stdout}, bri, target, dumper)
	} else {
		listener, err := net.Listen("tcp", addres)
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

			go connect(context.Background(), raw, bri, target, dumper)
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
