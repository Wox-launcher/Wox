package launcher

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wox/ui/contract"
	"wox/util"
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
func (c *runtimeSettingsController) Reload(ctx context.Context, service contract.RuntimeSettingsServices, sessionID string) {
	var revision uint64
	if !c.deps.OnUI("start loading runtime statuses", func() {
		c.revision++
		revision = c.revision
		c.loading = true
		c.errMsg = ""
		c.deps.Invalidate()
	}) {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	loaded, err := service.RuntimeStatuses(timeoutCtx, sessionID)
	statuses := make([]runtimeStatus, len(loaded))
	for index, status := range loaded {
		statuses[index] = runtimeStatusFromContract(status)
	}

	c.deps.OnUI("apply runtime statuses", func() {
		if revision != c.revision {
			return
		}
		c.loading = false
		if err != nil {
			c.errMsg = err.Error()
		} else {
			c.statuses = statuses
			c.loaded = true
		}
		c.deps.Invalidate()
	})
}

// Restart restarts a runtime host, then invokes reloadAfter to refresh statuses.
// reloadAfter runs after restarting is cleared so the view reflects the in-flight
// restart correctly.
func (c *runtimeSettingsController) Restart(ctx context.Context, service contract.RuntimeSettingsServices, sessionID string, runtime string, reloadAfter func()) {
	runtime = strings.ToUpper(strings.TrimSpace(runtime))
	shouldRestart := false
	if !c.deps.OnUI("start restarting plugin runtime", func() {
		if runtime == "" || c.restarting != "" {
			return
		}
		for _, status := range c.statuses {
			if strings.EqualFold(status.Runtime, runtime) && status.CanRestart {
				shouldRestart = true
				break
			}
		}
		if !shouldRestart {
			return
		}
		c.restarting = runtime
		c.errMsg = ""
		c.deps.Invalidate()
	}) || !shouldRestart {
		return
	}

	util.Go(ctx, "restart plugin runtime", func() {
		timeoutCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
		err := service.RestartRuntime(timeoutCtx, sessionID, runtime)
		cancel()
		c.deps.OnUI("finish restarting plugin runtime", func() {
			c.restarting = ""
			if err != nil {
				c.errMsg = fmt.Sprintf("Could not restart %s: %v", runtimeDisplayName(runtime), err)
			}
			c.deps.Invalidate()
		})
		if reloadAfter != nil {
			reloadAfter()
		}
	})
}

// runtimeStatusFromContract isolates controller state from core-owned runtime slices.
func runtimeStatusFromContract(status contract.RuntimeStatus) runtimeStatus {
	return runtimeStatus{
		Runtime:           status.Runtime,
		IsStarted:         status.IsStarted,
		HostVersion:       status.HostVersion,
		StatusCode:        status.StatusCode,
		StatusMessage:     status.StatusMessage,
		ExecutablePath:    status.ExecutablePath,
		LastStartError:    status.LastStartError,
		CanRestart:        status.CanRestart,
		InstallURL:        status.InstallURL,
		LoadedPluginCount: status.LoadedPluginCount,
		LoadedPluginNames: append([]string(nil), status.LoadedPluginNames...),
	}
}

// SetError records an error from outside the reload/restart paths (e.g. install URL open failure).
func (c *runtimeSettingsController) SetError(msg string) {
	c.deps.OnUI("set runtime settings error", func() {
		c.errMsg = msg
		c.deps.Invalidate()
	})
}

// Snapshot returns a copy of the Runtime state for the view layer.
func (c *runtimeSettingsController) Snapshot() runtimeSettingsSnapshot {
	return runtimeSettingsSnapshot{
		Statuses:   cloneRuntimeStatuses(c.statuses),
		Loading:    c.loading,
		Loaded:     c.loaded,
		Error:      c.errMsg,
		Restarting: c.restarting,
		Revision:   c.revision,
	}
}
