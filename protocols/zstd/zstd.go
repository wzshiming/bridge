package zstd

import (
	"context"
	"io"
	"net"
	"runtime"
	"sync"

	zstd "github.com/klauspost/compress/zstd"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
)

// ZStd zstd:
func ZStd(dialer bridge.Dialer, cmd string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}

	if l, ok := dialer.(bridge.ListenConfig); ok {
		return struct {
			bridge.Dialer
			bridge.ListenConfig
		}{
			zstdDialer{dialer},
			zstdListenConfig{l},
		}, nil
	}
	return zstdDialer{dialer}, nil
}

type zstdDialer struct {
	dialer bridge.Dialer
}

func (n zstdDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	c, err := n.dialer.DialContext(ctx, network, address)
	if err != nil {
		return nil, err
	}
	conn, err := newWarpConn(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	return conn, nil
}

type zstdListenConfig struct {
	listenConfig bridge.ListenConfig
}

func (n zstdListenConfig) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	l, err := n.listenConfig.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return wrapListener{l}, nil
}

func newWarpConn(conn net.Conn) (net.Conn, error) {
	w := compressor(conn)
	r := decompressor(conn)
	return wrapConn{
		Conn: conn,
		w:    w,
		r:    r,
	}, nil
}

type wrapConn struct {
	net.Conn
	w io.WriteCloser
	r io.ReadCloser
}

func (w wrapConn) Read(b []byte) (int, error) {
	return w.r.Read(b)
}

func (w wrapConn) Write(b []byte) (int, error) {
	n, err := w.w.Write(b)
	if err != nil {
		return n, err
	}
	return n, nil
}

func (w wrapConn) Close() error {
	err := w.Conn.Close()
	w.r.Close()
	w.w.Close()
	return err
}

type wrapListener struct {
	net.Listener
}

func (w wrapListener) Accept() (net.Conn, error) {
	c, err := w.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return newWarpConn(c)
}

var (
	encoderPool sync.Pool // *encoder
	decoderPool sync.Pool // *decoder
)

func decompressor(r io.Reader) io.ReadCloser {
	p := &reader{}
	if dec, _ := decoderPool.Get().(*decoder); dec == nil {
		z, err := zstd.NewReader(r)
		if err != nil {
			p.err = err
		} else {
			p.dec = &decoder{z}
			// We need a finalizer because the reader spawns goroutines
			// that will only be stopped if the Close method is called.
			runtime.SetFinalizer(p.dec, (*decoder).finalize)
		}
	} else {
		p.dec = dec
		p.err = dec.Reset(r)
	}
	return p
}

type decoder struct {
	*zstd.Decoder
}

func (d *decoder) finalize() {
	d.Close()
}

type reader struct {
	dec *decoder
	err error
}

func (r *reader) Close() error {
	if r.dec != nil {
		r.dec.Reset(devNull{})
		decoderPool.Put(r.dec)
		r.dec = nil
		r.err = io.ErrClosedPipe
	}
	return nil
}

func (r *reader) Read(p []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.dec.Read(p)
}

// WriteTo implements the io.WriterTo interface.
func (r *reader) WriteTo(w io.Writer) (int64, error) {
	if r.err != nil {
		return 0, r.err
	}
	return r.dec.WriteTo(w)
}

func compressor(w io.Writer) io.WriteCloser {
	p := &writer{}
	if enc, _ := encoderPool.Get().(*encoder); enc == nil {
		z, err := zstd.NewWriter(w, zstd.WithEncoderLevel(zstd.SpeedFastest))
		if err != nil {
			p.err = err
		} else {
			p.enc = &encoder{z}
			// We need a finalizer because the writer spawns goroutines
			// that will only be stopped if the Close method is called.
			runtime.SetFinalizer(p.enc, (*encoder).finalize)
		}
	} else {
		p.enc = enc
		p.enc.Reset(w)
	}
	return p
}

type encoder struct {
	*zstd.Encoder
}

func (e *encoder) finalize() {
	e.Close()
}

type writer struct {
	enc *encoder
	err error
}

func (w *writer) Close() error {
	if w.enc != nil {
		// Close needs to be called to write the end of stream marker and flush
		// the buffers. The zstd package documents that the encoder is re-usable
		// after being closed.
		err := w.enc.Close()
		if err != nil {
			w.err = err
		}
		w.enc.Reset(devNull{}) // don't retain the underlying writer
		encoderPool.Put(w.enc)
		w.enc = nil
		return err
	}
	return nil
}

func (w *writer) Write(p []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.enc == nil {
		return 0, io.ErrClosedPipe
	}
	n, err := w.enc.Write(p)
	if err != nil {
		return n, err
	}
	err = w.enc.Flush()
	if err != nil {
		return n, err
	}
	return n, nil
}

func (w *writer) ReadFrom(r io.Reader) (int64, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.enc == nil {
		return 0, io.ErrClosedPipe
	}
	return w.enc.ReadFrom(r)
}

type devNull struct{}

func (devNull) Read([]byte) (int, error)  { return 0, io.EOF }
func (devNull) Write([]byte) (int, error) { return 0, nil }
