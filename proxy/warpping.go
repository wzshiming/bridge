package proxy

import (
	"errors"
	"io"
	"net"
	"sync"
)

var ErrNetClosing = errors.New("use of closed network connection")

func UnreadConn(conn net.Conn, prefix []byte) net.Conn {
	if len(prefix) == 0 {
		return conn
	}
	if us, ok := conn.(*unreadConn); ok {
		us.Reader = Unread(us.Reader, prefix)
		return us
	}
	return &unreadConn{
		Reader: Unread(conn, prefix),
		Conn:   conn,
	}
}

type unreadConn struct {
	io.Reader
	net.Conn
}

func (c *unreadConn) Read(p []byte) (n int, err error) {
	return c.Reader.Read(p)
}

func Unread(reader io.Reader, prefix []byte) io.Reader {
	if len(prefix) == 0 {
		return reader
	}
	if ur, ok := reader.(*unread); ok {
		ur.prefix = append(prefix, ur.prefix...)
		return reader
	}
	return &unread{
		prefix: prefix,
		reader: reader,
	}
}

type unread struct {
	prefix []byte
	reader io.Reader
}

func (u *unread) Read(p []byte) (n int, err error) {
	if len(u.prefix) == 0 {
		return u.reader.Read(p)
	}
	n = copy(p, u.prefix)
	if n <= len(u.prefix) {
		u.prefix = u.prefix[n:]
		return n, nil
	}
	a, err := u.reader.Read(p[n:])
	if err == io.EOF {
		err = nil
	}
	n += a
	return n, err
}

type singleConnListener struct {
	addr net.Addr
	ch   chan net.Conn
	once sync.Once
}

func NewSingleConnListener(conn net.Conn) net.Listener {
	ch := make(chan net.Conn, 1)
	ch <- conn
	return &singleConnListener{
		addr: conn.LocalAddr(),
		ch:   ch,
	}
}

func (l *singleConnListener) Accept() (net.Conn, error) {
	conn, ok := <-l.ch
	if !ok || conn == nil {
		return nil, ErrNetClosing
	}
	return &connCloser{
		l:    l,
		Conn: conn,
	}, nil
}

func (l *singleConnListener) shutdown() error {
	l.once.Do(func() {
		close(l.ch)
	})
	return nil
}

func (l *singleConnListener) Close() error {
	return nil
}

func (l *singleConnListener) Addr() net.Addr {
	return l.addr
}

type connCloser struct {
	l *singleConnListener
	net.Conn
}

func (c *connCloser) Close() error {
	c.l.shutdown()
	return c.Conn.Close()
}
