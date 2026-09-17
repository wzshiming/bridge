package idle

import (
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeConn only implements the methods idleConn touches; the embedded net.Conn stays nil.
type fakeConn struct {
	net.Conn
	readErr  error
	writeErr error
	closed   atomic.Int32
}

func (f *fakeConn) Read(b []byte) (int, error) {
	if f.readErr != nil {
		return 0, f.readErr
	}
	return len(b), nil
}

func (f *fakeConn) Write(b []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(b), nil
}

func (f *fakeConn) Close() error {
	f.closed.Add(1)
	return nil
}

func TestCloseAlwaysClosesConn(t *testing.T) {
	tests := []struct {
		name string
		fc   *fakeConn
		io   func(net.Conn) error
	}{
		{name: "no prior error", fc: &fakeConn{}},
		{
			name: "after read error",
			fc:   &fakeConn{readErr: io.EOF},
			io: func(c net.Conn) error {
				_, err := c.Read(make([]byte, 1))
				return err
			},
		},
		{
			name: "after write error",
			fc:   &fakeConn{writeErr: errors.New("broken pipe")},
			io: func(c net.Conn) error {
				_, err := c.Write([]byte("x"))
				return err
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := NewIdleConn(tt.fc, time.Hour)
			if tt.io != nil {
				if err := tt.io(c); err == nil {
					t.Fatal("expected I/O error")
				}
			}
			if err := c.Close(); err != nil {
				t.Fatalf("Close: %v", err)
			}
			if got := tt.fc.closed.Load(); got != 1 {
				t.Fatalf("underlying Close called %d times, want 1", got)
			}
			if connManager.remove(c.(*idleConn)) {
				t.Fatal("conn still registered after Close")
			}
		})
	}
}

func TestConcurrentTrafficAndClear(t *testing.T) {
	c := NewIdleConn(&fakeConn{}, time.Hour)
	defer c.Close()

	var wg sync.WaitGroup
	spin := func(f func()) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 1000 {
				f()
			}
		}()
	}
	spin(func() { c.Write([]byte("x")) })
	spin(func() { c.Read(make([]byte, 1)) })
	spin(func() { connManager.Clear() })
	spin(func() { NewIdleConn(&fakeConn{}, time.Hour).Close() })
	wg.Wait()
}

func TestLastKeepsMonotonicClock(t *testing.T) {
	c := NewIdleConn(&fakeConn{}, time.Hour).(*idleConn)
	defer c.Close()
	steps := []struct {
		name  string
		touch func()
	}{
		{"new", func() {}},
		{"read", func() { c.Read(make([]byte, 1)) }},
		{"write", func() { c.Write([]byte("x")) }},
	}
	for _, s := range steps {
		s.touch()
		last := c.last.Load().(time.Time)
		// time.Time.String prints an "m=" field only while the monotonic reading is present.
		if !strings.Contains(last.String(), " m=") {
			t.Fatalf("%s: last lost its monotonic clock reading: %v", s.name, last)
		}
	}
}

func TestClearClosesOnlyExpired(t *testing.T) {
	m := newIdleConnManager()
	stale := time.Now().Add(-time.Minute)
	active := &idleConn{timeout: time.Hour, Conn: &fakeConn{}}
	active.last.Store(time.Now())
	written := &idleConn{timeout: time.Second, Conn: &fakeConn{}}
	written.last.Store(stale)
	read := &idleConn{timeout: time.Second, Conn: &fakeConn{}}
	read.last.Store(stale)
	expired := &idleConn{timeout: time.Second, Conn: &fakeConn{}}
	expired.last.Store(stale)
	for _, c := range []*idleConn{active, written, read, expired} {
		m.add(c)
	}
	if _, err := written.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	if _, err := read.Read(make([]byte, 1)); err != nil {
		t.Fatal(err)
	}

	m.Clear()
	m.Clear()

	for name, c := range map[string]*idleConn{"active": active, "written": written, "read": read} {
		if got := c.Conn.(*fakeConn).closed.Load(); got != 0 {
			t.Fatalf("%s conn closed %d times", name, got)
		}
		if !m.remove(c) {
			t.Fatalf("%s conn was unregistered by Clear", name)
		}
	}
	if got := expired.Conn.(*fakeConn).closed.Load(); got != 1 {
		t.Fatalf("expired conn closed %d times, want 1", got)
	}
	if m.remove(expired) {
		t.Fatal("expired conn still registered after Clear")
	}
}
