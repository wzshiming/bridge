package bridge_test

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	bridgepkg "github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/chain"
	"github.com/wzshiming/socks5"
)

type ctxKey struct{}

func TestListenPacketConfigFuncPassesArgumentsAndResults(t *testing.T) {
	ctx := context.WithValue(context.Background(), ctxKey{}, "marker")
	wantConn := &net.UDPConn{}
	wantErr := errors.New("listen packet failed")

	var gotCtx context.Context
	var gotNetwork, gotAddress string
	var lpc bridgepkg.ListenPacketConfig = bridgepkg.ListenPacketConfigFunc(func(ctx context.Context, network, address string) (net.PacketConn, error) {
		gotCtx, gotNetwork, gotAddress = ctx, network, address
		return wantConn, wantErr
	})

	conn, err := lpc.ListenPacket(ctx, "udp4", "127.0.0.1:9")
	if gotCtx != ctx {
		t.Errorf("wrapped func got ctx %v, want %v", gotCtx, ctx)
	}
	if gotNetwork != "udp4" || gotAddress != "127.0.0.1:9" {
		t.Errorf("wrapped func got (%q, %q), want (%q, %q)", gotNetwork, gotAddress, "udp4", "127.0.0.1:9")
	}
	if conn != wantConn {
		t.Errorf("ListenPacket returned conn %v, want %v", conn, wantConn)
	}
	if err != wantErr {
		t.Errorf("ListenPacket returned err %v, want %v", err, wantErr)
	}
}

func TestSOCKS5UDPRelaySocketFromListenPacketConfigFunc(t *testing.T) {
	ctx := context.Background()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	go socks5.NewServer().Serve(l)

	echo, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer echo.Close()
	go func() {
		buf := make([]byte, 1024)
		for {
			n, from, err := echo.ReadFrom(buf)
			if err != nil {
				return
			}
			echo.WriteTo(buf[:n], from)
		}
	}()

	var opened atomic.Int32
	var dialer net.Dialer
	var listenConfig net.ListenConfig
	base := struct {
		bridgepkg.Dialer
		bridgepkg.ListenPacketConfigFunc
	}{
		Dialer: &dialer,
		ListenPacketConfigFunc: func(ctx context.Context, network, address string) (net.PacketConn, error) {
			opened.Add(1)
			return listenConfig.ListenPacket(ctx, network, address)
		},
	}

	d, err := chain.Default.BridgeChain(ctx, base, "socks5://"+l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	conn, err := d.DialContext(ctx, "udp", echo.LocalAddr().String())
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
	if got := opened.Load(); got != 1 {
		t.Errorf("ListenPacketConfigFunc called %d times, want 1", got)
	}
}
