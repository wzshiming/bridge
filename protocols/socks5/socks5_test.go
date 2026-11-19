package socks5

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/socks5"
)

// packetBase dials with a net.Dialer and records the packet sockets it opens.
type packetBase struct {
	net.Dialer
	mu     sync.Mutex
	opened []*countingPacketConn
}

func (b *packetBase) ListenPacket(ctx context.Context, network, address string) (net.PacketConn, error) {
	var lc net.ListenConfig
	conn, err := lc.ListenPacket(ctx, network, address)
	if err != nil {
		return nil, err
	}
	c := &countingPacketConn{PacketConn: conn}
	b.mu.Lock()
	b.opened = append(b.opened, c)
	b.mu.Unlock()
	return c, nil
}

// countingPacketConn counts the datagrams that leave and arrive through it.
type countingPacketConn struct {
	net.PacketConn
	writes, reads atomic.Int32
}

func (c *countingPacketConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	n, err := c.PacketConn.WriteTo(p, addr)
	if err == nil {
		c.writes.Add(1)
	}
	return n, err
}

func (c *countingPacketConn) ReadFrom(p []byte) (int, net.Addr, error) {
	n, addr, err := c.PacketConn.ReadFrom(p)
	if err == nil {
		c.reads.Add(1)
	}
	return n, addr, err
}

// startServer serves SOCKS5 on loopback until the test ends and returns its URL.
func startServer(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go socks5.NewServer().Serve(l)
	return "socks5://" + l.Addr().String()
}

// startEcho echoes UDP datagrams on loopback until the test ends.
func startEcho(t *testing.T) string {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	go func() {
		buf := make([]byte, 1024)
		for {
			n, from, err := conn.ReadFrom(buf)
			if err != nil {
				return
			}
			conn.WriteTo(buf[:n], from)
		}
	}()
	return conn.LocalAddr().String()
}

// echoThroughChain dials echo over UDP through one socks5 hop on base and checks a datagram round-trips.
func echoThroughChain(t *testing.T, base bridge.Dialer) {
	t.Helper()
	ctx := context.Background()
	d, err := chain.Default.BridgeChain(ctx, base, startServer(t))
	if err != nil {
		t.Fatal(err)
	}
	conn, err := d.DialContext(ctx, "udp", startEcho(t))
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := conn.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 64)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "ping" {
		t.Errorf("echo = %q, want %q", buf[:n], "ping")
	}
}

func TestUDPRelaySocketFromBase(t *testing.T) {
	base := &packetBase{}
	echoThroughChain(t, base)

	base.mu.Lock()
	defer base.mu.Unlock()
	if len(base.opened) != 1 {
		t.Fatalf("base.ListenPacket called %d times, want 1", len(base.opened))
	}
	if w, r := base.opened[0].writes.Load(), base.opened[0].reads.Load(); w != 1 || r != 1 {
		t.Errorf("base socket carried %d writes and %d reads, want 1 each", w, r)
	}
}

func TestUDPRelayWithoutPacketBase(t *testing.T) {
	var dialer net.Dialer
	echoThroughChain(t, bridge.DialFunc(dialer.DialContext))
}
