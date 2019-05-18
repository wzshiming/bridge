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

func init() {
	flag.StringVar(&addres, "a", "", "pipe or tcp address, the default is pipe")
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr,
			`usage: bridge [-a address[:port]] [(socks5|ssh)://address:port ..]
`,
		)
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
		conn, err := dial("tcp", target)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}

		go io.Copy(conn, os.Stdin)
		io.Copy(os.Stdout, conn)
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

			go func() {
				conn, err := dial("tcp", target)
				if err != nil {
					fmt.Fprintln(os.Stderr, err.Error())
					return
				}

				go io.Copy(conn, raw)
				io.Copy(raw, conn)
			}()
		}
	}
}
