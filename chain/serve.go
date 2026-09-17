package chain

import (
	"context"
	"errors"
	"net"
	"time"
)

const (
	initialListenBackoff = 100 * time.Millisecond
	maxListenBackoff     = 30 * time.Second
)

type Event struct {
	Addr    net.Addr
	Err     error
	Attempt int
	Backoff time.Duration
}

func Serve(ctx context.Context, listen func(context.Context) (net.Listener, error), serve func(context.Context, net.Listener) error, report func(Event)) error {
	return serveWithBackoff(ctx, listen, serve, report, initialListenBackoff, maxListenBackoff)
}

func serveWithBackoff(ctx context.Context, listen func(context.Context) (net.Listener, error), serve func(context.Context, net.Listener) error, report func(Event), initial, maximum time.Duration) error {
	backoff := initial
	attempt := 0
	for ctx.Err() == nil {
		listener, err := listen(ctx)
		if err == nil && listener == nil {
			err = errors.New("listen returned no listener")
		}
		if err == nil {
			backoff = initial
			attempt = 0
			stop := context.AfterFunc(ctx, func() { listener.Close() })
			if report != nil {
				report(Event{Addr: listener.Addr()})
			}
			err = serve(ctx, listener)
			stop()
			listener.Close()
			if err == nil {
				err = errors.New("listener closed")
			}
		} else if listener != nil {
			listener.Close()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt++
		if report != nil {
			report(Event{Err: err, Attempt: attempt, Backoff: backoff})
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
		backoff = min(backoff*2, maximum)
	}
	return ctx.Err()
}
