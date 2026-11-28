package chain

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/wzshiming/permuteproxy"

	"github.com/wzshiming/bridge"
)

func init() {
	// own-socket is a proxy whose client binds a datagram socket of its own, as a UDP relay's does, and sends through it directly.
	permuteproxy.RegisterLayer("own-socket", permuteproxy.LayerSpec{
		Requires: permuteproxy.Stream,
		Provides: permuteproxy.Stream | permuteproxy.Packet,
		Networks: []string{"tcp", "udp"},
		NewClient: func(permuteproxy.Options) (*permuteproxy.ClientFuncs, error) {
			return &permuteproxy.ClientFuncs{
				Dial: func(ctx context.Context, inner permuteproxy.Transport, _, _ string) (net.Conn, error) {
					return inner.Dial(ctx, permuteproxy.Stream)
				},
				ListenPacket: func(ctx context.Context, _ permuteproxy.Transport, local permuteproxy.Network, network, address string) (net.PacketConn, error) {
					return local.ListenPacket(ctx, network, address)
				},
				Peer: func(_ context.Context, network, address string) (net.Addr, error) {
					return net.ResolveUDPAddr(network, address)
				},
			}, nil
		},
	})
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

// echoThroughHop dials echo over UDP through one hop over base, whose client binds its own socket, and checks a datagram round-trips.
func echoThroughHop(t *testing.T, base bridge.Dialer) {
	t.Helper()
	ctx := context.Background()
	d, err := Default.BridgeChain(ctx, base, "own-socket://127.0.0.1:1")
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

func TestClientSocketFromBase(t *testing.T) {
	var opened atomic.Int32
	echoThroughHop(t, struct {
		bridge.Dialer
		bridge.ListenPacketConfig
	}{
		&net.Dialer{},
		bridge.ListenPacketConfigFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
			opened.Add(1)
			return (&net.ListenConfig{}).ListenPacket(ctx, network, address)
		}),
	})
	if got := opened.Load(); got != 1 {
		t.Errorf("base.ListenPacket called %d times, want 1", got)
	}
}

func TestClientSocketWithoutPacketBase(t *testing.T) {
	echoThroughHop(t, &net.Dialer{})
}
