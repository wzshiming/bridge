package netutils

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"
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
	addr := Addr(address)
	listener := newVirtualNetwork(v, addr)

	v.mut.Lock()
	defer v.mut.Unlock()
	l, ok := v.address[address]
	if ok {
		old := l
		defer old.Close()
	}
	v.address[address] = listener
	return listener, nil
}

func (v *virtualNetworkManager) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	v.mut.RLock()
	defer v.mut.RUnlock()
	l, ok := v.address[address]
	if ok {
		return l.Conn(Addr(address))
	}
	return nil, fmt.Errorf("couldn't connect to virtual server %s://%s", network, address)
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
	isClose    uint32
}

func newVirtualNetwork(parent *virtualNetworkManager, serverAddr net.Addr) *VirtualNetwork {
	return &VirtualNetwork{
		parent:     parent,
		serverAddr: serverAddr,
		ch:         make(chan net.Conn),
	}
}

func (l *VirtualNetwork) Accept() (net.Conn, error) {
	conn, ok := <-l.ch
	if !ok {
		return nil, ErrClosedConn
	}
	return conn, nil
}

func (l *VirtualNetwork) Close() error {
	if atomic.CompareAndSwapUint32(&l.isClose, 0, 1) {
		close(l.ch)
		if l.parent != nil {
			l.parent.close(l)
		}
	}
	return nil
}

func (l *VirtualNetwork) Addr() net.Addr {
	return l.serverAddr
}

func (l *VirtualNetwork) Conn(clientAddr net.Addr) (net.Conn, error) {
	if atomic.LoadUint32(&l.isClose) == 1 {
		return nil, ErrClosedConn
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
	l.ch <- s
	return c, nil
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
