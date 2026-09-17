package idle

import (
	"net"
	"sync/atomic"
	"time"
)

type idleConn struct {
	timeout time.Duration
	last    atomic.Value // time.Time of the last I/O, monotonic reading kept
	net.Conn
}

// NewIdleConn wraps a net.Conn with idle timeout.
func NewIdleConn(conn net.Conn, timeout time.Duration) net.Conn {
	c := &idleConn{
		timeout: timeout,
		Conn:    conn,
	}
	c.last.Store(time.Now())
	_ = connManager.add(c)
	return c
}

func (c *idleConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if err != nil {
		connManager.remove(c)
	} else {
		c.last.Store(time.Now())
	}
	return n, err
}

func (c *idleConn) Write(b []byte) (int, error) {
	c.last.Store(time.Now())
	n, err := c.Conn.Write(b)
	if err != nil {
		connManager.remove(c)
	} else {
		c.last.Store(time.Now())
	}
	return n, err
}

func (c *idleConn) Close() error {
	connManager.remove(c)
	return c.Conn.Close()
}
