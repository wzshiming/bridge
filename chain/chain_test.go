package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net"
	"slices"
	"strings"
	"testing"

	"github.com/wzshiming/bridge"
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
