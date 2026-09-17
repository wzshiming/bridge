package chain

import (
	"context"
	"io"

	"github.com/wzshiming/bridge/internal/pool"
)

func Tunnel(ctx context.Context, first, second io.ReadWriteCloser) error {
	closeBoth := func() {
		first.Close()
		second.Close()
	}
	defer closeBoth()
	stop := context.AfterFunc(ctx, closeBoth)
	defer stop()
	type result struct {
		err        error
		halfClosed bool
	}
	done := make(chan result, 2)
	copyStream := func(destination, source io.ReadWriteCloser) {
		buffer := pool.Bytes.Get()
		_, err := io.CopyBuffer(destination, source, buffer)
		pool.Bytes.Put(buffer)
		completed := result{err: err}
		if closer, ok := destination.(interface{ CloseWrite() error }); ok && err == nil {
			completed.halfClosed = closer.CloseWrite() == nil
		}
		done <- completed
	}
	go copyStream(first, second)
	go copyStream(second, first)
	completed := <-done
	if !completed.halfClosed {
		closeBoth()
	}
	reverse := <-done
	if completed.err != nil {
		return completed.err
	}
	return reverse.err
}
