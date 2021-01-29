package lb

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wzshiming/bridge"
)

const (
	minTimeout = time.Second / 2
)

type Dialer struct {
	pool    sync.Pool
	dialers []bridge.Dialer
	timeout time.Duration
}

func NewDialer(dialers []bridge.Dialer) bridge.Dialer {
	if len(dialers) == 1 {
		return dialers[0]
	}
	d := &Dialer{
		timeout: 30 * time.Second,
		dialers: dialers,
	}

	d.pool.New = func() interface{} {
		for _, dialer := range d.dialers[1:] {
			d.pool.Put(dialer)
		}
		return d.dialers[0]
	}
	return d
}

func (d *Dialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if d.timeout == minTimeout {
		return d.dialContext(ctx, network, address, len(d.dialers))
	}
	now := time.Now()
	conn, err := d.dialContext(ctx, network, address, len(d.dialers))
	if err != nil {
		return nil, err
	}
	if sub := time.Since(now); sub < d.timeout {
		if sub < minTimeout {
			sub = minTimeout
		}
		atomic.StoreInt64((*int64)(&d.timeout), int64(sub))
	}
	return conn, nil
}

func (d *Dialer) dialContext(ctx context.Context, network, address string, size int) (net.Conn, error) {
	ctx0, _ := context.WithTimeout(ctx, d.timeout*2)
	dialer := d.pool.Get().(bridge.Dialer)
	conn, err := dialer.DialContext(ctx0, network, address)
	if err != nil {
		if size == 0 {
			return nil, err
		}
		return d.dialContext(ctx, network, address, size-1)
	}
	d.pool.Put(dialer)
	return conn, nil
}
