package launcher

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// runtimeSettingsSnapshot is the immutable Runtime tab state consumed by the view layer.
type runtimeSettingsSnapshot struct {
	Statuses   []runtimeStatus
	Loading    bool
	Loaded     bool
	Error      string
	Restarting string
	Revision   uint64
}

// runtimeSettingsController owns the Runtime tab state (statuses, loading, restarting).
// Reload is revision-guarded so responses from superseded refreshes are discarded.
type runtimeSettingsController struct {
	deps       CommonDeps
	mu         sync.RWMutex
	statuses   []runtimeStatus
	loading    bool
	loaded     bool
	errMsg     string
	restarting string
	revision   uint64
}

func newRuntimeSettingsController(deps CommonDeps) *runtimeSettingsController {
	return &runtimeSettingsController{deps: deps}
}

// Reload fetches runtime statuses; ignores responses superseded by a newer refresh.
func (c *runtimeSettingsController) Reload(ctx context.Context, client backendClient) {
	c.mu.Lock()
	c.revision++
	revision := c.revision
	c.loading = true
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	var statuses []runtimeStatus
	err := client.Post(timeoutCtx, "/runtime/status", map[string]any{}, &statuses)

	c.mu.Lock()
	if revision != c.revision {
		c.mu.Unlock()
		return
	}
	c.loading = false
	if err != nil {
		c.errMsg = err.Error()
	} else {
		c.statuses = statuses
		c.loaded = true
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Restart restarts a runtime host, then invokes reloadAfter to refresh statuses.
// reloadAfter runs after restarting is cleared so the view reflects the in-flight
// restart correctly; canRestart is checked under c.mu so the controller does not
// need to hold a.mu to read runtime statuses.
func (c *runtimeSettingsController) Restart(ctx context.Context, client backendClient, runtime string, reloadAfter func()) {
	runtime = strings.ToUpper(strings.TrimSpace(runtime))
	c.mu.Lock()
	if runtime == "" || c.restarting != "" {
		c.mu.Unlock()
		return
	}
	canRestart := false
	for _, status := range c.statuses {
		if strings.EqualFold(status.Runtime, runtime) {
			canRestart = status.CanRestart
			break
		}
	}
	if !canRestart {
		c.mu.Unlock()
		return
	}
	c.restarting = runtime
	c.errMsg = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	go func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := client.Post(timeoutCtx, "/runtime/restart", map[string]string{"Runtime": runtime}, nil)
		cancel()
		c.mu.Lock()
		c.restarting = ""
		c.mu.Unlock()
		if reloadAfter != nil {
			reloadAfter()
		}
		if err != nil {
			c.mu.Lock()
			c.errMsg = fmt.Sprintf("Could not restart %s: %v", runtimeDisplayName(runtime), err)
			c.mu.Unlock()
			c.deps.Invalidate()
		}
	}()
}

// SetError records an error from outside the reload/restart paths (e.g. install URL open failure).
func (c *runtimeSettingsController) SetError(msg string) {
	c.mu.Lock()
	c.errMsg = msg
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Runtime state for the view layer.
func (c *runtimeSettingsController) Snapshot() runtimeSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return runtimeSettingsSnapshot{
		Statuses:   cloneRuntimeStatuses(c.statuses),
		Loading:    c.loading,
		Loaded:     c.loaded,
		Error:      c.errMsg,
		Restarting: c.restarting,
		Revision:   c.revision,
	}
}
