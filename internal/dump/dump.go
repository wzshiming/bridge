package dump

import (
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"sync"

	"github.com/wzshiming/bridge/internal/log"
)

var mut = sync.Mutex{}

// syncDumper the asynchronous output is locked only for debug with no performance considerations
type syncDumper struct {
	Prefix string
	Count  int64
}

func (s *syncDumper) Dump(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil
	}
	mut.Lock()
	defer mut.Unlock()
	s.Count++
	log.Printf(" %d. %s \n", s.Count, s.Prefix)
	w := hex.Dumper(os.Stderr)
	defer w.Close()
	return w.Write(p)
}

type dumpConn struct {
	net.Conn
	R syncDumper
	W syncDumper
}

func NewDumpConn(conn net.Conn, rev bool, from, to string) net.Conn {
	w := syncDumper{Prefix: fmt.Sprintf("Send:    %s -> %s", from, to)}
	r := syncDumper{Prefix: fmt.Sprintf("Receive: %s <- %s", from, to)}
	if rev {
		r, w = w, r
	}
	return &dumpConn{
		Conn: conn,
		W:    w,
		R:    r,
	}
}

func (s *dumpConn) Write(p []byte) (n int, err error) {
	n, err = s.Conn.Write(p)
	s.W.Dump(p[:n])
	return n, err
}

func (s *dumpConn) Read(p []byte) (n int, err error) {
	n, err = s.Conn.Read(p)
	s.R.Dump(p[:n])
	return n, err
}

type dumpReadWriteCloser struct {
	io.ReadWriteCloser
	R syncDumper
	W syncDumper
}

func NewDumpReadWriteCloser(rwc io.ReadWriteCloser, rev bool, from, to string) io.ReadWriteCloser {
	w := syncDumper{Prefix: fmt.Sprintf("Send:    %s -> %s", from, to)}
	r := syncDumper{Prefix: fmt.Sprintf("Receive: %s <- %s", from, to)}
	if rev {
		r, w = w, r
	}
	return &dumpReadWriteCloser{
		ReadWriteCloser: rwc,
		W:               w,
		R:               r,
	}
}

func (s *dumpReadWriteCloser) Write(p []byte) (n int, err error) {
	n, err = s.ReadWriteCloser.Write(p)
	s.W.Dump(p[:n])
	return n, err
}

func (s *dumpReadWriteCloser) Read(p []byte) (n int, err error) {
	n, err = s.ReadWriteCloser.Read(p)
	s.R.Dump(p[:n])
	return n, err
}
