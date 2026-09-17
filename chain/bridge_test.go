package chain

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	_ "github.com/wzshiming/anyproxy/proxies/httpproxy"
	"github.com/wzshiming/bridge"
	"github.com/wzshiming/bridge/config"
)

type runtimeListener struct {
	net.Listener
	accepting  chan struct{}
	closed     chan struct{}
	acceptOnce sync.Once
	closeOnce  sync.Once
	next       net.Conn
	acceptErr  error
}

func newRuntimeListener(t *testing.T) *runtimeListener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	result := &runtimeListener{Listener: listener, accepting: make(chan struct{}), closed: make(chan struct{})}
	t.Cleanup(func() { result.Close() })
	return result
}

func (listener *runtimeListener) Accept() (net.Conn, error) {
	listener.acceptOnce.Do(func() { close(listener.accepting) })
	if listener.next != nil {
		conn := listener.next
		listener.next = nil
		return conn, nil
	}
	if listener.acceptErr != nil {
		return nil, listener.acceptErr
	}
	return listener.Listener.Accept()
}

func (listener *runtimeListener) Close() error {
	err := listener.Listener.Close()
	listener.closeOnce.Do(func() { close(listener.closed) })
	return err
}

func waitRuntime(t *testing.T, signal <-chan struct{}) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for listener lifecycle")
	}
}

func runRuntime(ctx context.Context, proxy bool, listen bridge.ListenConfig, addresses []string) error {
	runtime := NewBridge(slog.New(slog.NewTextHandler(io.Discard, nil)), false)
	dialer := &net.Dialer{}
	if proxy {
		return runtime.bridgeProxy(ctx, listen, dialer, 0, addresses, nil)
	}
	return runtime.bridgeStream(ctx, listen, dialer, 0, addresses, []string{"tcp://127.0.0.1:1"}, nil)
}

func runtimeAddresses(proxy bool) []string {
	if proxy {
		return []string{"http://127.0.0.1:10001", "http://127.0.0.1:10002"}
	}
	return []string{"tcp://127.0.0.1:10001", "tcp://127.0.0.1:10002"}
}

func TestBridgeInitialListenFailure(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		for _, partial := range []bool{false, true} {
			t.Run(fmt.Sprintf("proxy=%v/partial=%v", proxy, partial), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				listener := newRuntimeListener(t)
				failure := errors.New("bind failed")
				var calls atomic.Int32
				listen := bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
					call := calls.Add(1)
					if partial && call == 1 {
						return listener, nil
					}
					return nil, failure
				})
				done := make(chan error, 1)
				go func() { done <- runRuntime(ctx, proxy, listen, runtimeAddresses(proxy)) }()
				select {
				case err := <-done:
					if err != failure {
						t.Errorf("runtime = %v, want original bind error", err)
					}
				case <-time.After(time.Second):
					t.Fatal("initial bind failure did not return")
				}
				wantCalls := int32(1)
				if partial {
					wantCalls++
					waitRuntime(t, listener.closed)
				}
				if calls.Load() != wantCalls {
					t.Errorf("listen calls=%d, want %d without retry", calls.Load(), wantCalls)
				}
			})
		}
	}
}

func TestBridgeCancelDuringInitialBinds(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		t.Run(fmt.Sprint(proxy), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := newRuntimeListener(t)
			second := newRuntimeListener(t)
			binding := make(chan struct{})
			var calls atomic.Int32
			listen := bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
				if calls.Add(1) == 1 {
					return first, nil
				}
				close(binding)
				select {
				case <-first.closed:
				case <-time.After(time.Second):
					t.Error("first listener not closed while next bind blocked")
				}
				return second, nil
			})
			done := make(chan error, 1)
			go func() { done <- runRuntime(ctx, proxy, listen, runtimeAddresses(proxy)) }()
			waitRuntime(t, binding)
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("runtime cancellation = %v, want nil", err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("runtime did not finish canceled startup")
			}
			waitRuntime(t, first.closed)
			waitRuntime(t, second.closed)
		})
	}
}

func TestBridgeRelistensAndClosesOnCancel(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		t.Run(fmt.Sprint(proxy), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := newRuntimeListener(t)
			first.acceptErr = errors.New("accept failed")
			second := newRuntimeListener(t)
			var calls atomic.Int32
			listen := bridge.ListenConfigFunc(func(callCtx context.Context, network, address string) (net.Listener, error) {
				if callCtx != ctx || network != "tcp" || address != "127.0.0.1:10001" {
					t.Errorf("listen arguments = %v %s %s", callCtx, network, address)
				}
				if calls.Add(1) == 1 {
					return first, nil
				}
				return second, nil
			})
			done := make(chan error, 1)
			go func() { done <- runRuntime(ctx, proxy, listen, runtimeAddresses(proxy)[:1]) }()
			waitRuntime(t, second.accepting)
			waitRuntime(t, first.closed)
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("runtime cancellation = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("cancel did not unblock runtime Accept")
			}
			waitRuntime(t, second.closed)
			if calls.Load() != 2 {
				t.Errorf("listen calls = %d, want 2", calls.Load())
			}
		})
	}
}

func TestBridgeProxyClosesAcceptedSession(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener := newRuntimeListener(t)
	server, client := net.Pipe()
	defer client.Close()
	defer server.Close()
	listener.next = server
	done := make(chan error, 1)
	go func() {
		done <- runRuntime(ctx, true, bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
			return listener, nil
		}), runtimeAddresses(true)[:1])
	}()
	client.SetDeadline(time.Now().Add(time.Second))
	if _, err := client.Write([]byte("GET /")); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Errorf("runtime cancellation = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("proxy runtime did not stop")
	}
	var buffer [1]byte
	if _, err := client.Read(buffer[:]); err != io.EOF {
		t.Errorf("accepted session read = %v, want EOF", err)
	}
}

func TestBridgeWithConfigReturnsBindError(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		t.Run(fmt.Sprint(proxy), func(t *testing.T) {
			failure := errors.New("bind failed")
			var calls atomic.Int32
			runtime := NewBridge(slog.New(slog.NewTextHandler(io.Discard, nil)), false)
			runtime.chain = NewBridgeChain()
			runtime.chain.DialerFunc = nil
			runtime.chain.Register("fake", bridge.BridgeFunc(func(context.Context, bridge.Dialer, string) (bridge.Dialer, error) {
				return hop{DialFunc: (&net.Dialer{}).DialContext, ListenConfigFunc: func(context.Context, string, string) (net.Listener, error) {
					calls.Add(1)
					return nil, failure
				}}, nil
			}))
			dials := []string{"tcp://127.0.0.1:1"}
			if proxy {
				dials = []string{"-"}
			}
			err := runtime.BridgeWithConfig(context.Background(), config.Chain{
				Bind:  []config.Node{{LB: runtimeAddresses(proxy)[:1]}, {LB: []string{"fake://upstream"}}},
				Proxy: []config.Node{{LB: dials}},
			})
			if !errors.Is(err, failure) || calls.Load() != 1 {
				t.Errorf("BridgeWithConfig = %v, calls=%d", err, calls.Load())
			}
		})
	}
}

func TestBridgeBindsAllBeforeAccepting(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		t.Run(fmt.Sprint(proxy), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			first := newRuntimeListener(t)
			second := newRuntimeListener(t)
			var calls atomic.Int32
			listen := bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
				if calls.Add(1) == 1 {
					return first, nil
				}
				select {
				case <-first.accepting:
					t.Error("Accept started before all binds completed")
				default:
				}
				return second, nil
			})
			done := make(chan error, 1)
			go func() { done <- runRuntime(ctx, proxy, listen, runtimeAddresses(proxy)) }()
			waitRuntime(t, first.accepting)
			waitRuntime(t, second.accepting)
			cancel()
			select {
			case err := <-done:
				if err != nil {
					t.Errorf("runtime = %v", err)
				}
			case <-time.After(time.Second):
				t.Fatal("runtime did not stop")
			}
			waitRuntime(t, first.closed)
			waitRuntime(t, second.closed)
			if calls.Load() != 2 {
				t.Errorf("initial listeners were not reused: calls=%d", calls.Load())
			}
		})
	}
}

func TestBridgeInitialListenGuards(t *testing.T) {
	for _, proxy := range []bool{false, true} {
		for _, result := range []string{"nil", "listener-error", "canceled", "canceled-bind-error"} {
			t.Run(fmt.Sprintf("proxy=%v/%s", proxy, result), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				listener := newRuntimeListener(t)
				failure := errors.New("bind failed")
				calls := 0
				listen := bridge.ListenConfigFunc(func(context.Context, string, string) (net.Listener, error) {
					calls++
					switch result {
					case "nil":
						return nil, nil
					case "listener-error":
						return listener, failure
					case "canceled":
						cancel()
						return listener, ctx.Err()
					default:
						cancel()
						return listener, failure
					}
				})
				err := runRuntime(ctx, proxy, listen, runtimeAddresses(proxy))
				switch result {
				case "nil":
					if err == nil {
						t.Error("nil listener was accepted")
					}
				case "canceled":
					if err != nil {
						t.Errorf("cancellation = %v, want nil", err)
					}
				default:
					if err != failure {
						t.Errorf("runtime = %v, want original bind error", err)
					}
				}
				if calls != 1 {
					t.Errorf("listen calls=%d, want 1", calls)
				}
				if result != "nil" {
					waitRuntime(t, listener.closed)
				}
			})
		}
	}
}
