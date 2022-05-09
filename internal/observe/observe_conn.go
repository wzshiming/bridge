package observe

import (
	"io"
	"net"
	"time"

	"github.com/wzshiming/geario"
)

type observeConn struct {
	net.Conn
	readerGear *geario.Gear
	writerGear *geario.Gear
	reader     io.Reader
	writer     io.Writer
	update     chan struct{}
	done       chan struct{}
}

type ObserveConnFunc func(conn net.Conn, r, w *geario.Gear, isClosed bool)

func NewObserveConn(conn net.Conn, observeConnFunc ObserveConnFunc) net.Conn {
	readerGear := geario.NewGear(time.Second, -1)
	writerGear := geario.NewGear(time.Second, -1)
	c := &observeConn{
		Conn:       conn,
		readerGear: readerGear,
		writerGear: writerGear,
		reader:     readerGear.Reader(conn),
		writer:     writerGear.Writer(conn),
		update:     make(chan struct{}),
		done:       make(chan struct{}, 1),
	}
	go c.startObservable(observeConnFunc)
	return c
}
func (c *observeConn) startObservable(observeConnFunc ObserveConnFunc) {
	ticker := time.NewTicker(10 * time.Second)
	defer func() {
		ticker.Stop()
		observeConnFunc(c.Conn, c.readerGear, c.writerGear, true)
	}()
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			select {
			case <-c.done:
				return
			case <-c.update:
				observeConnFunc(c.Conn, c.readerGear, c.writerGear, false)
			}
		}
	}
}

func (c *observeConn) Read(b []byte) (int, error) {
	n, err := c.reader.Read(b)
	select {
	case c.update <- struct{}{}:
	default:
	}
	return n, err
}

func (c *observeConn) Write(b []byte) (int, error) {
	n, err := c.writer.Write(b)
	select {
	case c.update <- struct{}{}:
	default:
	}
	return n, err
}

func (c *observeConn) Close() error {
	select {
	case c.done <- struct{}{}:
	default:
	}
	return c.Conn.Close()
}
