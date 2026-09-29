package cloudsync

import (
	"sync"
	"sync/atomic"

	"wox/util"
)

// localSyncMutationMu keeps last-write-wins comparison, remote apply, and oplog
// retirement atomic with local setting and install writes. Callers that also take
// a setting mutex must acquire this lock first: remote apply holds it and then
// locks the setting value.
var localSyncMutationMu localSyncMutex

// localSyncMutex is reentrant for the owning goroutine. Applying a remote setting
// can notify plugins on the same goroutine, and those callbacks may write another
// setting before oplog cleanup finishes.
type localSyncMutex struct {
	mu sync.Mutex
	// owner is the goroutine id while the mutex is held, or 0 when it is free.
	owner atomic.Int64
	// depth is only accessed by the owning goroutine.
	depth int
}

func (m *localSyncMutex) lock() {
	gid := util.GetGID()
	if gid != 0 && m.owner.Load() == gid {
		m.depth++
		return
	}
	m.mu.Lock()
	m.owner.Store(gid)
	m.depth = 1
}

func (m *localSyncMutex) unlock() {
	m.depth--
	if m.depth > 0 {
		return
	}
	m.owner.Store(0)
	m.mu.Unlock()
}

// WithLocalSyncMutation runs a local setting or install write that must not overlap
// remote apply and oplog cleanup for the same sync identity.
func WithLocalSyncMutation(fn func() error) error {
	localSyncMutationMu.lock()
	defer localSyncMutationMu.unlock()
	return fn()
}
