package local

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/wzshiming/bridge"
)

// roundTrip sends a datagram from conn to an echo peer on loopback and checks it comes back.
func roundTrip(t *testing.T, conn net.PacketConn) {
	t.Helper()
	peer, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer peer.Close()
	go func() {
		buf := make([]byte, 64)
		n, from, err := peer.ReadFrom(buf)
		if err == nil {
			peer.WriteTo(buf[:n], from)
		}
	}()

	if _, err := conn.WriteTo([]byte("ping"), peer.LocalAddr()); err != nil {
		t.Fatal(err)
	}
	conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	buf := make([]byte, 64)
	n, from, err := conn.ReadFrom(buf)
	if err != nil {
		t.Fatal(err)
	}
	if string(buf[:n]) != "ping" || from.String() != peer.LocalAddr().String() {
		t.Errorf("got %q from %v, want %q from %v", buf[:n], from, "ping", peer.LocalAddr())
	}
}

func TestListenPacketRoundTrip(t *testing.T) {
	conn, err := LOCAL.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	roundTrip(t, conn)
}

func TestListenPacketUsesListenConfig(t *testing.T) {
	var controls atomic.Int32
	l := &Local{ListenConfig: &net.ListenConfig{
		Control: func(network, address string, c syscall.RawConn) error {
			controls.Add(1)
			return nil
		},
	}}
	conn, err := l.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if got := controls.Load(); got != 1 {
		t.Errorf("ListenConfig.Control ran %d times, want 1", got)
	}
	roundTrip(t, conn)
}

func TestListenPacketWithoutPacketListenConfig(t *testing.T) {
	l := &Local{ListenConfig: bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
		return nil, nil
	})}
	_, err := l.ListenPacket(context.Background(), "udp", "127.0.0.1:0")
	if !errors.Is(err, errors.ErrUnsupported) {
		t.Errorf("err = %v, want errors.ErrUnsupported", err)
	}
}
