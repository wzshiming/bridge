package idle

import (
	"net"
	"time"
)

type idleConn struct {
	timeout time.Duration
	last    time.Time
	net.Conn
}

// NewIdleConn wraps a net.Conn with idle timeout.
func NewIdleConn(conn net.Conn, timeout time.Duration) net.Conn {
	c := &idleConn{
		timeout: timeout,
		last:    time.Now(),
		Conn:    conn,
	}
	_ = connManager.add(c)
	return c
}

func (c *idleConn) Read(b []byte) (int, error) {
	n, err := c.Conn.Read(b)
	if err != nil {
		connManager.remove(c)
	} else {
		c.last = time.Now()
	}
	return n, err
}

func (c *idleConn) Write(b []byte) (int, error) {
	c.last = time.Now()
	n, err := c.Conn.Write(b)
	if err != nil {
		connManager.remove(c)
	} else {
		c.last = time.Now()
	}
	return n, err
}

func (c *idleConn) Close() error {
	if !connManager.remove(c) {
		return nil
	}
	return c.Conn.Close()
}
