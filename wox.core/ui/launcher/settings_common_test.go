package launcher

import "sync"

// testUIRunner models the synchronous serialization provided by woxui.Call.
type testUIRunner struct {
	mu sync.Mutex
}

func (r *testUIRunner) Run(_ string, fn func()) error {
	r.Do(fn)
	return nil
}

func (r *testUIRunner) Do(fn func()) {
	r.mu.Lock()
	defer r.mu.Unlock()
	fn()
}
