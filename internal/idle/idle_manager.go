package idle

import (
	"sync"
	"time"

	"github.com/wzshiming/bridge/logger"
)

var (
	connManager = newIdleConnManager()
	once        sync.Once
)

type idleConnManager struct {
	list map[*idleConn]struct{}
	mut  sync.RWMutex
}

func newIdleConnManager() *idleConnManager {
	return &idleConnManager{
		list: map[*idleConn]struct{}{},
	}
}

func (m *idleConnManager) add(conn *idleConn) bool {
	once.Do(func() {
		go connManager.run()
	})
	m.mut.Lock()
	defer m.mut.Unlock()
	if _, ok := m.list[conn]; ok {
		return false
	}

	m.list[conn] = struct{}{}
	return true
}

func (m *idleConnManager) remove(conn *idleConn) bool {
	m.mut.Lock()
	defer m.mut.Unlock()
	if _, ok := m.list[conn]; !ok {
		return false
	}

	delete(m.list, conn)
	return true
}

func (m *idleConnManager) Clear() {
	now := time.Now()
	conns := make([]*idleConn, 0, len(m.list))

	m.mut.RLock()
	for conn := range m.list {
		if conn.last.Add(conn.timeout).Before(now) {
			conns = append(conns, conn)
		}
	}
	m.mut.RUnlock()

	if len(conns) == 0 {
		return
	}

	logger.Std.Info("Clear idle connections", "count", len(conns))
	for _, conn := range conns {
		conn.Close()
	}
}

func (m *idleConnManager) run() {
	ticker := time.NewTicker(time.Second)
	for range ticker.C {
		m.Clear()
	}
}
