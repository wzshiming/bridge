package smux

import (
	"context"
	"fmt"
	"net"
	"sync"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/internal/log"
	"github.com/wzshiming/bridge/local"
	"github.com/xtaci/smux"
)

var conf = smux.DefaultConfig()

func SMux(dialer bridge.Dialer, addr string) (bridge.Dialer, error) {
	if dialer == nil {
		dialer = local.LOCAL
	}
	listenConfig, _ := dialer.(bridge.ListenConfig)
	return &sMux{dialer: dialer, listenConfig: listenConfig}, nil
}

type sMux struct {
	mux          sync.Mutex
	dialer       bridge.Dialer
	listenConfig bridge.ListenConfig
	sess         *smux.Session
}

func (m *sMux) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	if m.sess == nil || m.sess.IsClosed() {
		conn, err := m.dialer.DialContext(ctx, network, address)
		if err != nil {
			return nil, err
		}
		sess, err := smux.Client(conn, conf)
		if err != nil {
			return nil, err
		}
		m.sess = sess
	}
	conn, err := m.sess.OpenStream()
	if err != nil {
		m.sess.Close()
		return m.DialContext(ctx, network, address)
	}
	return conn, nil
}

func (m *sMux) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	if m.listenConfig == nil {
		return nil, fmt.Errorf("does not support the listen")
	}
	listener, err := m.listenConfig.Listen(ctx, network, address)
	if err != nil {
		return nil, err
	}
	return newListenerSession(ctx, listener), nil
}

type listenerSession struct {
	ctx      context.Context
	cancel   func()
	listener net.Listener
	conns    chan net.Conn
}

func newListenerSession(ctx context.Context, listener net.Listener) *listenerSession {
	ctx, cancel := context.WithCancel(ctx)
	l := &listenerSession{ctx: ctx, cancel: cancel, listener: listener, conns: make(chan net.Conn)}
	go l.run()
	return l
}

func (l *listenerSession) run() {
	defer l.Close()
	for l.ctx.Err() == nil {
		sess, err := l.accept()
		if err != nil {
			log.Println(err)
			return
		}
		go func() {
			err = l.acceptSession(sess)
			if err != nil {
				log.Println(err)
			}
		}()
	}
}

func (l *listenerSession) accept() (*smux.Session, error) {
	conn, err := l.listener.Accept()
	if err != nil {
		return nil, err
	}
	return smux.Server(conn, conf)
}

func (l *listenerSession) acceptSession(sess *smux.Session) error {
	for l.ctx.Err() == nil && !sess.IsClosed() {
		stm, err := sess.AcceptStream()
		if err != nil {
			return err
		}
		l.conns <- stm
	}
	return nil
}

func (l *listenerSession) Accept() (net.Conn, error) {
	select {
	case conn := <-l.conns:
		return conn, nil
	case <-l.ctx.Done():
		return nil, l.ctx.Err()
	}
}

func (l *listenerSession) Close() error {
	l.cancel()
	return l.Close()
}

func (l *listenerSession) Addr() net.Addr {
	return l.listener.Addr()
}
