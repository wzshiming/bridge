package bridge_test

import (
	"context"
	"errors"
	"net"
	"testing"

	bridgepkg "github.com/wzshiming/bridge"
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
