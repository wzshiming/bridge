package chain

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestServeCancelDuringBackoff(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	failure := errors.New("listen failed")
	reported := make(chan Event, 1)
	done := make(chan error, 1)
	calls := 0
	go func() {
		done <- serveWithBackoff(ctx, func(context.Context) (net.Listener, error) {
			calls++
			return nil, failure
		}, func(context.Context, net.Listener) error {
			t.Error("unexpected serve call")
			return nil
		}, func(event Event) {
			reported <- event
		}, time.Hour, time.Hour)
	}()
	select {
	case event := <-reported:
		if event != (Event{Err: failure, Attempt: 1, Backoff: time.Hour}) {
			t.Errorf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("listen failure was not reported")
	}
	cancel()
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Fatalf("Serve = %v, want context.Canceled", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancellation did not interrupt backoff")
	}
	if calls != 1 {
		t.Errorf("listen calls = %d, want 1", calls)
	}
}

func TestServeRetriesAndResetsAfterListen(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	failure := errors.New("listen failed")
	serveFailure := errors.New("serve failed")
	initial := time.Millisecond
	maximum := 3 * initial
	var events []Event
	var listeners []net.Listener
	calls := 0
	served := 0
	err := serveWithBackoff(ctx, func(callCtx context.Context) (net.Listener, error) {
		if callCtx != ctx {
			t.Error("listen context not forwarded")
		}
		calls++
		if calls <= 4 {
			return nil, failure
		}
		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err == nil {
			listeners = append(listeners, listener)
			t.Cleanup(func() { listener.Close() })
		}
		return listener, err
	}, func(callCtx context.Context, listener net.Listener) error {
		if callCtx != ctx || listener != listeners[len(listeners)-1] {
			t.Error("serve arguments not forwarded")
		}
		served++
		if served == 1 {
			return serveFailure
		}
		return nil
	}, func(event Event) {
		events = append(events, event)
		if len(events) == 8 {
			cancel()
		}
	}, initial, maximum)
	if err != context.Canceled || calls != 6 || served != 2 {
		t.Fatalf("err=%v listen=%d serve=%d", err, calls, served)
	}
	want := []Event{
		{Err: failure, Attempt: 1, Backoff: initial},
		{Err: failure, Attempt: 2, Backoff: 2 * initial},
		{Err: failure, Attempt: 3, Backoff: maximum},
		{Err: failure, Attempt: 4, Backoff: maximum},
		{Addr: listeners[0].Addr()},
		{Err: serveFailure, Attempt: 1, Backoff: initial},
		{Addr: listeners[1].Addr()},
		{Err: events[7].Err, Attempt: 1, Backoff: initial},
	}
	if events[7].Err == nil {
		t.Fatal("nil serve return was not reported as a failure")
	}
	for index, event := range events {
		if event != want[index] {
			t.Errorf("event %d=%+v, want %+v", index, event, want[index])
		}
	}
	for _, listener := range listeners {
		listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
		if conn, err := listener.Accept(); !errors.Is(err, net.ErrClosed) {
			if conn != nil {
				conn.Close()
			}
			t.Errorf("listener not closed: %v", err)
		}
	}
}

func TestServeClosesListenerOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- Serve(ctx, func(context.Context) (net.Listener, error) {
			return listener, nil
		}, func(context.Context, net.Listener) error {
			close(started)
			conn, err := listener.Accept()
			if conn != nil {
				conn.Close()
			}
			if !errors.Is(err, net.ErrClosed) {
				t.Errorf("Accept = %v, want net.ErrClosed", err)
			}
			return err
		}, nil)
	}()
	select {
	case <-started:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("serve did not start")
	}
	select {
	case err := <-done:
		if err != context.Canceled {
			t.Errorf("Serve = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not unblock Accept")
	}
}

func TestServeListenGuards(t *testing.T) {
	for _, withListener := range []bool{false, true} {
		t.Run(fmt.Sprint(withListener), func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			var listener net.Listener
			var failure error
			if withListener {
				var err error
				listener, err = net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				defer listener.Close()
				failure = errors.New("listen returned listener and error")
			}
			err := Serve(ctx, func(context.Context) (net.Listener, error) {
				return listener, failure
			}, func(context.Context, net.Listener) error {
				t.Error("serve called after invalid listen result")
				return nil
			}, func(event Event) {
				if event.Addr != nil || event.Err == nil || event.Attempt != 1 || event.Backoff != 100*time.Millisecond {
					t.Errorf("event = %+v", event)
				}
				if failure != nil && event.Err != failure {
					t.Errorf("error = %v, want original error", event.Err)
				}
				cancel()
			})
			if err != context.Canceled {
				t.Errorf("Serve = %v", err)
			}
			if listener != nil {
				listener.(*net.TCPListener).SetDeadline(time.Now().Add(time.Second))
				if conn, err := listener.Accept(); !errors.Is(err, net.ErrClosed) {
					if conn != nil {
						conn.Close()
					}
					t.Errorf("listener not closed: %v", err)
				}
			}
		})
	}
}
