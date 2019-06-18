package main

import (
	"context"
	"encoding/hex"
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
)

var addres string
var dumper bool

const defaults = `usage: bridge [-d] [-a [bind_address]:bind_port] proxy_address:proxy_port
              [(socks5|socks4|socks4a|https|http||ssh)://bridge_address:bridge_port ..]
`

func init() {
	flag.StringVar(&addres, "a", "", "Pipe or tcp address, the default is pipe")
	flag.BoolVar(&dumper, "d", false, "Output the communication data")
	flag.Parse()
}

func main() {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, defaults)
		flag.PrintDefaults()
		return
	}

	var bri bridge.Dialer

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

	if addres == "" {
		connect(context.Background(), struct {
			io.Reader
			io.Writer
		}{os.Stdin, os.Stdout}, bri, target, dumper)
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

func connect(ctx context.Context, raw io.ReadWriter, bri bridge.Dialer, target string, dumper bool) {
	conn, err := bri.DialContext(ctx, "tcp", target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	if dumper {
		dump := hex.Dumper(os.Stderr)
		go io.Copy(conn, io.TeeReader(raw, dump))
		io.Copy(raw, io.TeeReader(conn, dump))
	} else {
		go io.Copy(conn, raw)
		io.Copy(raw, conn)
	}
}
