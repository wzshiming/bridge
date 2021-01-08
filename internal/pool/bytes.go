package pool

import (
	"sync"
)

var DefaultSize = 32 * 1024

type bytesPool struct {
	sync.Pool
}

func (b *bytesPool) Get() []byte {
	buf := b.Pool.Get().([]byte)
	buf = buf[:cap(buf)]
	return buf
}

func (b *bytesPool) Put(d []byte) {
	if d == nil || len(d) < DefaultSize {
		return
	}
	b.Pool.Put(d)
}

var Bytes = &bytesPool{
	Pool: sync.Pool{
		New: func() interface{} {
			return make([]byte, DefaultSize)
		},
	},
}
