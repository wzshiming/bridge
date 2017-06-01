package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"strings"
	"time"

	"github.com/wzshiming/ffmt"
)

func main() {
	ps := flag.String("port", "", "[ip]:port[/protocol]->ip:port[/protocol][,[ip]:port[/protocol]->ip:port[/protocol]]")
	flag.Parse()

	rr, err := ParseCommon(*ps)
	if err != nil {
		fmt.Println(err)
		flag.PrintDefaults()
		return
	}
	if len(rr) == 0 {
		flag.PrintDefaults()
		return
	}

	for k, v := range rr {
		go Bridge(k, v)
	}
	for {
		time.Sleep(time.Second)
	}
}

func ParseCommon(s string) (r map[string]string, err error) {
	r = map[string]string{}
	as := strings.Split(s, ",")
	for _, v := range as {
		pp := strings.SplitN(v, "->", 2)
		if len(pp) != 2 {
			return nil, fmt.Errorf(v)
		}
		r[pp[0]] = pp[1]
	}
	return
}

func Bridge(a1, a2 string) {
	ls := strings.SplitN(a1, "/", 2)
	ds := strings.SplitN(a2, "/", 2)

	if len(ls) == 1 {
		ls = append(ls, "tcp")
	}
	if len(ds) == 1 {
		ds = append(ds, "tcp")
	}
	listen, err := net.Listen(ls[1], ls[0])
	if err != nil {
		ffmt.Mark(err)
		return
	}
	ffmt.Mark("listen ", ls, "->", ds)
	for {
		src, err := listen.Accept()
		if err != nil {
			ffmt.Mark(err)
			continue
		}

		dst, err := net.Dial(ds[1], ds[0])
		if err != nil {
			ffmt.Mark(err)
			continue
		}

		fmt.Println(src.RemoteAddr(), "->", listen.Addr(), "->", dst.RemoteAddr())
		bridge(dst, src)
	}
}

func bridge(c1, c2 net.Conn) {
	go io.Copy(c1, c2)
	go io.Copy(c2, c1)
}
