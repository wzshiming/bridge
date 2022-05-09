package observe

import (
	"net"
	"time"

	"github.com/wzshiming/geario"
)

type observeConns struct {
	readerGear         *geario.Gear
	writerGear         *geario.Gear
	observeConnAllFunc ObserveConnAllFunc
	done               chan struct{}
}

type ObserveConnAllFunc func(r, w *geario.Gear)

func NewObserveConns(observeConnAllFunc ObserveConnAllFunc) *observeConns {
	readerGear := geario.NewGear(time.Second, -1)
	writerGear := geario.NewGear(time.Second, -1)
	c := &observeConns{
		readerGear:         readerGear,
		writerGear:         writerGear,
		observeConnAllFunc: observeConnAllFunc,
		done:               make(chan struct{}, 1),
	}
	go c.startObservable(observeConnAllFunc)
	return c
}

func (c *observeConns) startObservable(observeConnAllFunc ObserveConnAllFunc) {
	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-c.done:
			return
		case <-ticker.C:
			observeConnAllFunc(c.readerGear, c.writerGear)
		}
	}
}

func (c *observeConns) Close() {
	select {
	case c.done <- struct{}{}:
	default:
	}
}

func (c *observeConns) Conn(conn net.Conn) net.Conn {
	return geario.ConnGear(conn, c.readerGear, c.writerGear)
}
