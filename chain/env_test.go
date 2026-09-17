package chain

import (
	"context"
	"errors"
	"net"
	"reflect"
	"testing"

	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/protocols/local"
	"github.com/wzshiming/hostmatcher"
)

type envCall struct {
	who              string
	ctx              context.Context
	network, address string
}

type envCtxKey struct{}

func envMatcher(host string) hostmatcher.Matcher {
	if host == "" {
		return nil
	}
	return hostmatcher.NewMatcher([]string{host})
}

func TestNewEnvDialer(t *testing.T) {
	noProxy, onlyProxy, localDialer := NoProxy, OnlyProxy, local.LOCAL.Dialer
	t.Cleanup(func() { NoProxy, OnlyProxy, local.LOCAL.Dialer = noProxy, onlyProxy, localDialer })

	tests := []struct {
		name      string
		noProxy   string
		onlyProxy string
		address   string
		wantDial  string
	}{
		{"no env", "", "", "a.test:80", "origin"},
		{"no_proxy matched", "a.test", "", "a.test:80", "local"},
		{"no_proxy unmatched", "a.test", "", "b.test:80", "origin"},
		{"only_proxy matched", "", "a.test", "a.test:80", "origin"},
		{"only_proxy unmatched", "", "a.test", "b.test:80", "local"},
		{"both no_proxy precedence", "a.test", "a.test", "a.test:80", "local"},
		{"both only_proxy matched", "a.test", "b.test", "b.test:80", "origin"},
		{"both neither matched", "a.test", "b.test", "c.test:80", "local"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			NoProxy, OnlyProxy = envMatcher(tt.noProxy), envMatcher(tt.onlyProxy)

			ctx := context.WithValue(context.Background(), envCtxKey{}, tt.name)
			errFake := errors.New("fake")
			var got []envCall
			local.LOCAL.Dialer = bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
				got = append(got, envCall{"local dial", ctx, network, address})
				return nil, errFake
			})
			origin := struct {
				bridge.Dialer
				bridge.ListenConfig
			}{
				bridge.DialFunc(func(ctx context.Context, network, address string) (net.Conn, error) {
					got = append(got, envCall{"origin dial", ctx, network, address})
					return nil, errFake
				}),
				bridge.ListenConfigFunc(func(ctx context.Context, network, address string) (net.Listener, error) {
					got = append(got, envCall{"origin listen", ctx, network, address})
					return nil, errFake
				}),
			}

			d := NewEnvDialer(origin)
			if _, err := d.DialContext(ctx, "tcp", tt.address); err != errFake {
				t.Errorf("DialContext() error = %v, want %v", err, errFake)
			}
			l, ok := d.(bridge.ListenConfig)
			if !ok {
				t.Fatalf("NewEnvDialer() = %T, lost ListenConfig", d)
			}
			if _, err := l.Listen(ctx, "tcp", "127.0.0.1:0"); err != errFake {
				t.Errorf("Listen() error = %v, want %v", err, errFake)
			}
			want := []envCall{
				{tt.wantDial + " dial", ctx, "tcp", tt.address},
				{"origin listen", ctx, "tcp", "127.0.0.1:0"},
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("calls = %+v, want %+v", got, want)
			}

			if _, ok := NewEnvDialer(origin.Dialer).(bridge.ListenConfig); ok {
				t.Errorf("NewEnvDialer(dial-only) gained ListenConfig")
			}
		})
	}
}
