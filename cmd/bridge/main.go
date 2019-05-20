package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/wzshiming/bridge/chain"
)

var addres string

const defaults = `usage: bridge [-a [bind_address]:bind_port] proxy_address:proxy_port
              [(socks5|socks4|socks4a|https|http||ssh)://bridge_address:bridge_port ..]
`

func init() {
	flag.StringVar(&addres, "a", "", "pipe or tcp address, the default is pipe")
	flag.Parse()
}

func main() {
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, defaults)
		flag.PrintDefaults()
		return
	}

	dial := net.Dial
	target := args[0]

	args = args[1:]
	if len(args) != 0 {
		d, err := chain.Default.BridgeChain(nil, args...)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
		dial = d.Dial
	}

	if addres == "" {
		connect(struct {
			io.Reader
			io.Writer
		}{os.Stdin, os.Stdout}, dial, target)
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
			go connect(raw, dial, target)
		}
	}
}

func connect(raw io.ReadWriter, dial func(network, address string) (net.Conn, error), target string) {
	conn, err := dial("tcp", target)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		return
	}

	go io.Copy(conn, raw)
	io.Copy(raw, conn)
}
