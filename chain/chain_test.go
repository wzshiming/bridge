package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
	"github.com/wzshiming/bridge/logger"
)

const (
	rawHop  = "ssh://user:secret@127.0.0.1:1"
	safeHop = "ssh://127.0.0.1:1"
	target  = "example.com:80"
)

var (
	errBuild  = errors.New("build failed")
	errTarget = errors.New("target failed")
)

// hop is a fake previous proxy that can both dial and listen.
type hop struct {
	bridge.DialFunc
	bridge.ListenConfigFunc
}

// captureLog routes logger.Std to a JSON buffer for the duration of the test.
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	old := logger.Std
	logger.Std = slog.New(slog.NewJSONHandler(buf, nil))
	t.Cleanup(func() { logger.Std = old })
	return buf
}

// lastRecord asserts exactly one secret-free log line was written and returns it decoded.
func lastRecord(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 1 || lines[0] == "" {
		t.Fatalf("want 1 log line, got %d: %q", len(lines), buf.String())
	}
	if strings.Contains(lines[0], "secret") {
		t.Errorf("log line leaks secret: %s", lines[0])
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &rec); err != nil {
		t.Fatalf("decode %q: %v", lines[0], err)
	}
	return rec
}

func TestBackoffManagerLogOmitsUserInfo(t *testing.T) {
	tests := []struct {
		name       string
		listen     bool
		build      error
		target     error
		noListen   bool
		wantMsg    string
		wantTarget bool
	}{
		{name: "dial build fail", build: errBuild, wantMsg: "failed dial"},
		{name: "dial target fail", target: errTarget, wantMsg: "failed dial target", wantTarget: true},
		{name: "dial success", wantMsg: "success dial target", wantTarget: true},
		{name: "listen build fail", listen: true, build: errBuild, wantMsg: "failed dial"},
		{name: "listen unsupported", listen: true, noListen: true, wantMsg: "failed listen"},
		{name: "listen target fail", listen: true, target: errTarget, wantMsg: "failed listen target", wantTarget: true},
		{name: "listen success", listen: true, wantMsg: "success listen target", wantTarget: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := captureLog(t)
			var gotBuild, gotTarget []string
			record := func(network, address string) { gotTarget = append(gotTarget, network+" "+address) }
			build := func(_ context.Context, _ bridge.Dialer, address string) (bridge.Dialer, error) {
				gotBuild = append(gotBuild, address)
				if tt.build != nil {
					return nil, tt.build
				}
				h := hop{
					DialFunc: func(_ context.Context, network, address string) (net.Conn, error) {
						record(network, address)
						if tt.target != nil {
							return nil, tt.target
						}
						return struct{ net.Conn }{}, nil
					},
					ListenConfigFunc: func(_ context.Context, network, address string) (net.Listener, error) {
						record(network, address)
						if tt.target != nil {
							return nil, tt.target
						}
						return struct{ net.Listener }{}, nil
					},
				}
				if tt.noListen {
					return h.DialFunc, nil
				}
				return h, nil
			}

			m := newBackoffManager(nil, build, []string{rawHop})
			var err error
			if tt.listen {
				_, err = m.Listen(context.Background(), "tcp", target)
			} else {
				_, err = m.DialContext(context.Background(), "tcp", target)
			}
			wantErr := tt.build
			if wantErr == nil {
				wantErr = tt.target
			}
			if wantErr != nil && !errors.Is(err, wantErr) || (err != nil) != (wantErr != nil || tt.noListen) {
				t.Fatalf("err = %v, want %v", err, wantErr)
			}

			if len(gotBuild) != 1 || gotBuild[0] != rawHop {
				t.Errorf("builder got %q, want [%q]", gotBuild, rawHop)
			}
			var wantNet []string
			if tt.wantTarget {
				wantNet = []string{"tcp " + target}
			}
			if !slices.Equal(gotTarget, wantNet) {
				t.Errorf("previous hop got %q, want %q", gotTarget, wantNet)
			}

			rec := lastRecord(t, buf)
			if rec["msg"] != tt.wantMsg {
				t.Errorf("msg = %v, want %q", rec["msg"], tt.wantMsg)
			}
			if rec["previous"] != safeHop {
				t.Errorf("previous = %v, want %q", rec["previous"], safeHop)
			}
			if got, ok := rec["target"]; ok != tt.wantTarget || ok && got != target {
				t.Errorf("target = %v (present %v), want %q (present %v)", got, ok, target, tt.wantTarget)
			}
		})
	}
}

func TestHopFuncLazyReuse(t *testing.T) {
	b := NewBridgeChain()
	b.DialerFunc = nil
	var builds, wraps, wrappedCalls, rawCalls int
	raw := bridge.DialFunc(func(_ context.Context, network, address string) (net.Conn, error) {
		rawCalls++
		if network != "tcp" || address != target {
			t.Fatalf("dial = %s %s, want tcp %s", network, address, target)
		}
		return struct{ net.Conn }{}, nil
	})
	if err := b.Register("ssh", bridge.BridgeFunc(func(_ context.Context, _ bridge.Dialer, address string) (bridge.Dialer, error) {
		builds++
		if address != rawHop {
			t.Fatalf("build address = %q, want %q", address, rawHop)
		}
		return raw, nil
	})); err != nil {
		t.Fatal(err)
	}
	b.HopFunc = func(index int, address string, dialer bridge.Dialer) bridge.Dialer {
		wraps++
		if index != 0 || address != rawHop {
			t.Fatalf("hop = %d %q, want 0 %q", index, address, rawHop)
		}
		return bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
			wrappedCalls++
			return dialer.DialContext(ctx, network, address)
		})
	}
	dialer, err := b.BridgeChain(context.Background(), nil, rawHop)
	if err != nil {
		t.Fatal(err)
	}
	if builds != 0 || wraps != 0 {
		t.Fatalf("eager construction: builds = %d, wraps = %d", builds, wraps)
	}
	for use := 1; use <= 2; use++ {
		if _, err := dialer.DialContext(context.Background(), "tcp", target); err != nil {
			t.Fatal(err)
		}
		if builds != 1 || wraps != 1 || wrappedCalls != use || rawCalls != use {
			t.Fatalf("use %d: builds = %d, wraps = %d, wrapped calls = %d, raw calls = %d", use, builds, wraps, wrappedCalls, rawCalls)
		}
	}
}

func buildHopChain(t *testing.T, b *BridgeChain, base bridge.Dialer, configPath bool, addresses ...string) bridge.Dialer {
	t.Helper()
	var dialer bridge.Dialer
	var err error
	if configPath {
		nodes := make([]config.Node, len(addresses))
		for index, address := range addresses {
			nodes[index].LB = strings.Split(address, "|")
		}
		dialer, err = b.BridgeChainWithConfig(context.Background(), base, nodes...)
	} else {
		dialer, err = b.BridgeChain(context.Background(), base, addresses...)
	}
	if err != nil {
		t.Fatal(err)
	}
	return dialer
}

func TestHopFuncPaths(t *testing.T) {
	for _, configPath := range []bool{false, true} {
		name := "strings"
		if configPath {
			name = "config"
		}
		t.Run(name, func(t *testing.T) {
			b := NewBridgeChain()
			b.DialerFunc = nil
			const exitOne = "fake://user:secret@exit-one:1?option=value"
			const exitTwo = "fake://user:secret@exit-two:2"
			const nearBase = "fake://user:secret@base:3"
			type wrappedHop struct {
				index   int
				address string
			}
			var wrapped []wrappedHop
			var builds, calls []string
			base := bridge.DialFunc(func(_ context.Context, network, address string) (net.Conn, error) {
				if network != "tcp" || address != target {
					t.Fatalf("dial = %s %s, want tcp %s", network, address, target)
				}
				calls = append(calls, "base")
				return struct{ net.Conn }{}, nil
			})
			if err := b.Register("fake", bridge.BridgeFunc(func(_ context.Context, base bridge.Dialer, address string) (bridge.Dialer, error) {
				builds = append(builds, address)
				return &hop{DialFunc: func(ctx context.Context, network, target string) (net.Conn, error) {
					calls = append(calls, "raw "+address)
					return base.DialContext(ctx, network, target)
				}}, nil
			})); err != nil {
				t.Fatal(err)
			}
			b.HopFunc = func(index int, address string, dialer bridge.Dialer) bridge.Dialer {
				if _, ok := dialer.(*hop); !ok {
					t.Fatalf("hook received %T, want raw protocol dialer", dialer)
				}
				wrapped = append(wrapped, wrappedHop{index, address})
				return bridge.DialFunc(func(ctx context.Context, network, target string) (net.Conn, error) {
					calls = append(calls, "wrapped "+address)
					return dialer.DialContext(ctx, network, target)
				})
			}
			dialer := buildHopChain(t, b, base, configPath, exitOne+"|"+exitTwo, nearBase)
			if len(builds) != 0 || len(wrapped) != 0 {
				t.Fatal("chain construction was not lazy")
			}
			for _, exit := range []string{exitOne, exitTwo, exitOne, exitTwo} {
				calls = nil
				if _, err := dialer.DialContext(context.Background(), "tcp", target); err != nil {
					t.Fatal(err)
				}
				want := []string{"wrapped " + exit, "raw " + exit, "wrapped " + nearBase, "raw " + nearBase, "base"}
				if !slices.Equal(calls, want) {
					t.Fatalf("calls = %q, want %q", calls, want)
				}
			}
			wantWrapped := []wrappedHop{{0, exitOne}, {1, nearBase}, {0, exitTwo}}
			if !reflect.DeepEqual(wrapped, wantWrapped) {
				t.Errorf("wrapped = %+v, want %+v", wrapped, wantWrapped)
			}
			if want := []string{exitOne, nearBase, exitTwo}; !slices.Equal(builds, want) {
				t.Errorf("builds = %q, want %q", builds, want)
			}
		})
	}
}

func TestHopFuncListenReuse(t *testing.T) {
	b := NewBridgeChain()
	b.DialerFunc = nil
	var builds, wraps int
	var calls []string
	raw := &hop{
		DialFunc: func(_ context.Context, network, address string) (net.Conn, error) {
			calls = append(calls, "raw dial "+network+" "+address)
			return struct{ net.Conn }{}, nil
		},
		ListenConfigFunc: func(_ context.Context, network, address string) (net.Listener, error) {
			calls = append(calls, "raw listen "+network+" "+address)
			return struct{ net.Listener }{}, nil
		},
	}
	if err := b.Register("ssh", bridge.BridgeFunc(func(context.Context, bridge.Dialer, string) (bridge.Dialer, error) {
		builds++
		return raw, nil
	})); err != nil {
		t.Fatal(err)
	}
	b.HopFunc = func(index int, address string, dialer bridge.Dialer) bridge.Dialer {
		wraps++
		if index != 0 || address != rawHop || dialer != raw {
			t.Fatalf("unexpected hop: %d %q %T", index, address, dialer)
		}
		return &hop{
			DialFunc: func(ctx context.Context, network, address string) (net.Conn, error) {
				calls = append(calls, "wrapped dial")
				return dialer.DialContext(ctx, network, address)
			},
			ListenConfigFunc: func(ctx context.Context, network, address string) (net.Listener, error) {
				calls = append(calls, "wrapped listen")
				return dialer.(bridge.ListenConfig).Listen(ctx, network, address)
			},
		}
	}
	dialer := buildHopChain(t, b, nil, true, rawHop)
	if builds != 0 || wraps != 0 {
		t.Fatal("chain construction was not lazy")
	}
	listener, ok := dialer.(bridge.ListenConfig)
	if !ok {
		t.Fatalf("chain %T lost ListenConfig", dialer)
	}
	for use := 0; use < 2; use++ {
		if _, err := listener.Listen(context.Background(), "tcp", "127.0.0.1:0"); err != nil {
			t.Fatal(err)
		}
		if _, err := dialer.DialContext(context.Background(), "tcp", target); err != nil {
			t.Fatal(err)
		}
	}
	want := []string{
		"wrapped listen", "raw listen tcp 127.0.0.1:0", "wrapped dial", "raw dial tcp " + target,
		"wrapped listen", "raw listen tcp 127.0.0.1:0", "wrapped dial", "raw dial tcp " + target,
	}
	if builds != 1 || wraps != 1 || !slices.Equal(calls, want) {
		t.Fatalf("builds = %d, wraps = %d, calls = %q, want 1, 1, %q", builds, wraps, calls, want)
	}
}

func TestHopFuncBuildFailure(t *testing.T) {
	b := NewBridgeChain()
	b.DialerFunc = nil
	var builds, wraps int
	if err := b.Register("ssh", bridge.BridgeFunc(func(context.Context, bridge.Dialer, string) (bridge.Dialer, error) {
		builds++
		return nil, errBuild
	})); err != nil {
		t.Fatal(err)
	}
	b.HopFunc = func(_ int, _ string, dialer bridge.Dialer) bridge.Dialer {
		wraps++
		return dialer
	}
	dialer := buildHopChain(t, b, nil, false, rawHop)
	if _, err := dialer.DialContext(context.Background(), "tcp", target); !errors.Is(err, errBuild) {
		t.Fatalf("DialContext error = %v, want %v", err, errBuild)
	}
	if _, err := dialer.(bridge.ListenConfig).Listen(context.Background(), "tcp", target); !errors.Is(err, errBuild) {
		t.Fatalf("Listen error = %v, want %v", err, errBuild)
	}
	if builds != 2 || wraps != 0 {
		t.Fatalf("builds = %d, wraps = %d, want 2, 0", builds, wraps)
	}
}

func TestHopFuncCompatibility(t *testing.T) {
	for _, configPath := range []bool{false, true} {
		b := NewBridgeChain()
		b.DialerFunc = nil
		var builds, dialerWraps, rawCalls int
		base := &hop{DialFunc: func(context.Context, string, string) (net.Conn, error) {
			rawCalls++
			return struct{ net.Conn }{}, nil
		}}
		if err := b.Register("ssh", bridge.BridgeFunc(func(_ context.Context, dialer bridge.Dialer, _ string) (bridge.Dialer, error) {
			builds++
			return dialer, nil
		})); err != nil {
			t.Fatal(err)
		}
		b.DialerFunc = func(dialer bridge.Dialer) bridge.Dialer {
			dialerWraps++
			return dialer
		}
		dialer := buildHopChain(t, b, base, configPath, rawHop)
		if _, err := dialer.DialContext(context.Background(), "tcp", target); err != nil {
			t.Fatal(err)
		}
		wantWraps := 0
		if configPath {
			wantWraps = 1
		}
		if builds != 1 || rawCalls != 1 || dialerWraps != wantWraps {
			t.Fatalf("config %v: builds = %d, calls = %d, DialerFunc calls = %d, want 1, 1, %d", configPath, builds, rawCalls, dialerWraps, wantWraps)
		}
		b.HopFunc = func(_ int, _ string, dialer bridge.Dialer) bridge.Dialer {
			t.Fatal("HopFunc called for empty chain")
			return dialer
		}
		for _, emptyBase := range []bridge.Dialer{base, nil} {
			if got := buildHopChain(t, b, emptyBase, configPath); got != emptyBase {
				t.Fatalf("config %v: empty chain = %v, want %v", configPath, got, emptyBase)
			}
		}
		if builds != 1 || dialerWraps != wantWraps {
			t.Fatalf("config %v: empty chain invoked constructor or DialerFunc", configPath)
		}
	}
}

func TestLogAddr(t *testing.T) {
	tests := map[string]string{
		rawHop:                        safeHop,
		"socks5://u:p@[::1]:1080?k=v": "socks5://[::1]:1080",
		"127.0.0.1:1111":              "tcp://127.0.0.1:1111",
		"cmd:nc %h %p":                "cmd://nc %h %p",
		"cmd:ssh user@jump nc %h %p":  "", // '@' survives the split, so the command is not echoed
		"ssh://user:p%zz@127.0.0.1:1": "", // invalid escape: split falls back to the raw address
		"user:secret@127.0.0.1:1":     "", // missing scheme: password lands in the address part
		"nonsense":                    "",
	}
	for in, want := range tests {
		if got := logAddr(in); got != want {
			t.Errorf("logAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
