package netcat

import (
	"context"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/wzshiming/bridge"
)

const prefix = "ssh example"

type wantKey struct{}

// want is what one operation expects the command and its bridge to receive; it rides in ctx.
type want struct{ network, address, cmd string }

// fakeBridge forwards each dial or listen to the check it wraps.
type fakeBridge func(ctx context.Context, network, address string)

func (f fakeBridge) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	f(ctx, network, address)
	return nil, nil
}

func (f fakeBridge) Listen(ctx context.Context, network, address string) (net.Listener, error) {
	f(ctx, network, address)
	return nil, nil
}

// newNetCat builds a netCat whose command checks the chosen command and returns a bridge that checks ctx, network and address, counting both.
func newNetCat(t *testing.T, cmds, calls *atomic.Int32) *netCat {
	d, err := NetCat(context.Background(), nil, "nc:"+prefix)
	if err != nil {
		t.Fatal(err)
	}
	n := d.(*netCat)
	n.command = func(ctx context.Context, _ bridge.Dialer, cmd string) (bridge.Dialer, error) {
		cmds.Add(1)
		w, _ := ctx.Value(wantKey{}).(want)
		if cmd != w.cmd {
			t.Errorf("command = %q, want %q", cmd, w.cmd)
		}
		return fakeBridge(func(got context.Context, network, address string) {
			calls.Add(1)
			if got != ctx || network != w.network || address != w.address {
				t.Errorf("%q bridge got (same ctx: %t) %s %s, want %s %s", cmd, got == ctx, network, address, w.network, w.address)
			}
		}), nil
	}
	return n
}

// call issues one dial or listen carrying its expectations in ctx.
func call(n *netCat, listen bool, network, address, cmd string) error {
	ctx := context.WithValue(context.Background(), wantKey{}, want{network, address, "cmd: " + prefix + " " + cmd})
	if listen {
		_, err := n.Listen(ctx, network, address)
		return err
	}
	_, err := n.DialContext(ctx, network, address)
	return err
}

var networks = []struct{ network, address, dial, listen string }{
	{"tcp", "host:1", "nc %h %p", "nc -l %h %p"},
	{"tcp4", "host:4", "nc -4 %h %p", "nc -4l %h %p"},
	{"tcp6", "host:6", "nc -6 %h %p", "nc -6l %h %p"},
	{"unix", "/run/nc.sock", "nc -U %h", "nc -Ul %h"},
	{"tcp", "host:2", "nc %h %p", "nc -l %h %p"},
}

func TestNetworkSelection(t *testing.T) {
	for _, op := range []string{"dial", "listen"} {
		t.Run(op, func(t *testing.T) {
			var cmds, calls atomic.Int32
			n := newNetCat(t, &cmds, &calls)
			for i, tt := range networks {
				cmd := tt.dial
				if op == "listen" {
					cmd = tt.listen
				}
				if err := call(n, op == "listen", tt.network, tt.address, cmd); err != nil {
					t.Fatal(err)
				}
				if c, b := cmds.Load(), calls.Load(); c != int32(i+1) || b != int32(i+1) {
					t.Errorf("%s: %d commands and %d bridge calls after %d operations", tt.network, c, b, i+1)
				}
			}
		})
	}
}

func TestConcurrentNetworks(t *testing.T) {
	var cmds, calls atomic.Int32
	n := newNetCat(t, &cmds, &calls)
	var wg sync.WaitGroup
	for _, tt := range networks {
		for _, listen := range []bool{false, true} {
			cmd := tt.dial
			if listen {
				cmd = tt.listen
			}
			wg.Go(func() {
				if err := call(n, listen, tt.network, tt.address, cmd); err != nil {
					t.Error(err)
				}
			})
		}
	}
	wg.Wait()
	if c, b, total := cmds.Load(), calls.Load(), int32(2*len(networks)); c != total || b != total {
		t.Errorf("%d commands and %d bridge calls, want %d each", c, b, total)
	}
}
