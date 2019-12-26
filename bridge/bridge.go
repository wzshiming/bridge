package bridge

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/bridge/internal/log"
)

func Bridge(listens, dials []string, dump bool) error {
	log.Println(showChain(dials, listens))

	var (
		bialer       bridge.Dialer       = &net.Dialer{}
		listenConfig bridge.ListenConfig = &net.ListenConfig{}
	)

	dial := dials[0]
	dials = dials[1:]
	if len(dials) != 0 {
		b, _, err := chain.Default.BridgeChain(nil, dials...)
		if err != nil {
			return err
		}
		bialer = b
	}

	if len(listens) == 0 {
		connect(context.Background(), struct {
			io.ReadCloser
			io.Writer
		}{ioutil.NopCloser(os.Stdin), os.Stdout}, bialer, "STDIO", dial, dump)
	} else {
		listen := resolveAddr(listens[0])
		listens = listens[1:]

		if len(listens) != 0 {
			_, l, err := chain.Default.BridgeChain(nil, listens...)
			if err != nil {
				return err
			}
			if l == nil {
				return errors.New("the last proxy could not listen")
			}
			listenConfig = l
		}

		listener, err := listenConfig.Listen(context.Background(), "tcp", listen)
		if err != nil {
			return err
		}
		for {
			raw, err := listener.Accept()
			if err != nil {
				return err
			}

			go connect(context.Background(), raw, bialer, raw.RemoteAddr().String(), dial, dump)
		}
	}
	return nil
}

func connect(ctx context.Context, raw io.ReadWriteCloser, bri bridge.Dialer, from string, to string, dump bool) {
	conn, err := bri.DialContext(ctx, "tcp", to)
	if err != nil {
		log.Println(err)
		return
	}
	defer raw.Close()
	defer conn.Close()
	if dump {
		dumpRaw := &syncWriter{Prefix: fmt.Sprintf("Send:    %s -> %s", from, to)}
		dumpConn := &syncWriter{Prefix: fmt.Sprintf("Receive: %s <- %s", from, to)}

		go io.Copy(conn, io.TeeReader(raw, dumpRaw))
		io.Copy(raw, io.TeeReader(conn, dumpConn))
	} else {
		go io.Copy(conn, raw)
		io.Copy(raw, conn)
	}
}

var mut = sync.Mutex{}

// syncWriter the asynchronous output is locked only for debug with no performance considerations
type syncWriter struct {
	Prefix string
	Count  int64
}

func (s *syncWriter) Write(p []byte) (n int, err error) {
	mut.Lock()
	defer mut.Unlock()
	s.Count++
	log.Printf(" %d. %s \n", s.Count, s.Prefix)
	w := hex.Dumper(os.Stderr)
	defer w.Close()
	return w.Write(p)
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
