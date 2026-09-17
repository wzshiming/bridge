package netutils

import (
	"context"
	"fmt"
	"net"
	"sync"
)

var virtualNetwork = newVirtualNetworkManager()

type virtualNetworkManager struct {
	address map[string]*VirtualNetwork
	mut     sync.RWMutex
}

func newVirtualNetworkManager() *virtualNetworkManager {
	return &virtualNetworkManager{
		address: map[string]*VirtualNetwork{},
	}
}

type Addr string

func (a Addr) Network() string {
	return "virtual"
}

func (a Addr) String() string {
	return string(a)
}

func (v *virtualNetworkManager) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	listener := newVirtualNetwork(v, Addr(address))

	v.mut.Lock()
	old := v.address[address]
	v.address[address] = listener
	v.mut.Unlock()

	// Close takes the manager lock again, so it must run after Unlock.
	if old != nil {
		old.Close()
	}
	return listener, nil
}

func (v *virtualNetworkManager) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	v.mut.RLock()
	l, ok := v.address[address]
	v.mut.RUnlock()
	if !ok {
		return nil, fmt.Errorf("couldn't connect to virtual server %s://%s", network, address)
	}
	return l.Conn(ctx, Addr(address))
}

func (v *virtualNetworkManager) close(listener *VirtualNetwork) error {
	address := listener.Addr()
	addr := address.String()

	v.mut.Lock()
	defer v.mut.Unlock()
	l, ok := v.address[addr]
	if ok && l == listener {
		delete(v.address, addr)
	}
	return nil
}

type VirtualNetwork struct {
	parent     *virtualNetworkManager
	serverAddr net.Addr
	ch         chan net.Conn
	done       chan struct{}
	closeOnce  sync.Once
}

func newVirtualNetwork(parent *virtualNetworkManager, serverAddr net.Addr) *VirtualNetwork {
	return &VirtualNetwork{
		parent:     parent,
		serverAddr: serverAddr,
		ch:         make(chan net.Conn),
		done:       make(chan struct{}),
	}
}

func (l *VirtualNetwork) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, ErrClosedConn
	case conn := <-l.ch:
		return conn, nil
	}
}

func (l *VirtualNetwork) Close() error {
	l.closeOnce.Do(func() {
		close(l.done)
		if l.parent != nil {
			l.parent.close(l)
		}
	})
	return nil
}

func (l *VirtualNetwork) Addr() net.Addr {
	return l.serverAddr
}

func (l *VirtualNetwork) Conn(ctx context.Context, clientAddr net.Addr) (net.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	select {
	case <-l.done:
		return nil, ErrClosedConn
	default:
	}
	c, s := net.Pipe()
	s = &pipeConn{
		Conn:       s,
		remoteAddr: clientAddr,
		localAddr:  l.serverAddr,
	}
	c = &pipeConn{
		Conn:       c,
		remoteAddr: l.serverAddr,
		localAddr:  clientAddr,
	}
	select {
	case l.ch <- s:
		return c, nil
	case <-l.done:
		c.Close()
		s.Close()
		return nil, ErrClosedConn
	case <-ctx.Done():
		c.Close()
		s.Close()
		return nil, ctx.Err()
	}
}

type pipeConn struct {
	net.Conn
	localAddr  net.Addr
	remoteAddr net.Addr
}

func (c *pipeConn) LocalAddr() net.Addr {
	return c.localAddr
}

func (c *pipeConn) RemoteAddr() net.Addr {
	return c.remoteAddr
}
