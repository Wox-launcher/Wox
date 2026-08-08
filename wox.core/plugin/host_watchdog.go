package plugin

import (
	"context"
	"fmt"
	"time"

	"wox/util"
)

const (
	// hostWatchdogCheckInterval is how often the manager verifies that shared
	// runtime host processes (python/nodejs) are still alive. All plugins of a
	// runtime live in one host process, so one crashed process takes the whole
	// runtime down with it; the watchdog restarts the host instead of leaving
	// the runtime broken until the next manual plugin load.
	hostWatchdogCheckInterval = 5 * time.Second

	// hostWatchdogMaxConsecutiveFailures is how many watchdog restarts may fail
	// (host dies again shortly after restart) before auto-restart backs off.
	hostWatchdogMaxConsecutiveFailures = 3

	// hostWatchdogCooldownDuration suspends watchdog auto-restart after repeated
	// fast failures so a plugin that crashes the host on every load cannot force
	// an endless restart loop.
	hostWatchdogCooldownDuration = 5 * time.Minute
)

// hostWatchdogState tracks watchdog restart attempts per runtime. Without the
// counters, a plugin that kills the host on every load would make the watchdog
// restart the host in a tight loop.
type hostWatchdogState struct {
	consecutiveRestarts int
	cooldownUntil       int64
}

// startHostWatchdog launches the shared-host health monitor. The watchdog only
// acts when a runtime host was started and still has loaded plugin instances;
// a stopped host with no plugins is left alone.
func (m *Manager) startHostWatchdog(ctx context.Context) {
	m.hostWatchdogLifecycle.Lock()
	if m.hostWatchdogStarted {
		m.hostWatchdogLifecycle.Unlock()
		return
	}
	if m.hostWatchdogStop == nil {
		m.hostWatchdogStop = make(chan struct{})
	}
	if m.hostWatchdogDone == nil {
		m.hostWatchdogDone = make(chan struct{})
	}
	m.hostWatchdogStarted = true
	m.hostWatchdogLifecycle.Unlock()

	util.Go(ctx, "start host watchdog", func() {
		defer close(m.hostWatchdogDone)
		logger.Info(ctx, "start host watchdog")
		ticker := time.NewTicker(hostWatchdogCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-m.hostWatchdogStop:
				logger.Info(ctx, "host watchdog stopped")
				return
			case <-ticker.C:
				m.checkHostsHealth(util.NewTraceContext())
			}
		}
	})
}

// stopHostWatchdog stops the health monitor so Wox shutdown does not try to
// restart hosts that Manager.Stop is intentionally tearing down.
func (m *Manager) stopHostWatchdog() {
	m.hostWatchdogLifecycle.Lock()
	if !m.hostWatchdogStarted {
		m.hostWatchdogLifecycle.Unlock()
		return
	}
	select {
	case <-m.hostWatchdogStop:
	default:
		close(m.hostWatchdogStop)
	}
	done := m.hostWatchdogDone
	m.hostWatchdogLifecycle.Unlock()
	<-done
}

// checkHostsHealth verifies every shared runtime host is still alive and
// recovers dead hosts by restarting them together with their plugins.
func (m *Manager) checkHostsHealth(ctx context.Context) {
	for _, pluginHost := range AllHosts {
		runtimeName := pluginHost.GetRuntime(ctx)
		if runtimeName == PLUGIN_RUNTIME_GO || runtimeName == PLUGIN_RUNTIME_SCRIPT {
			continue
		}

		if pluginHost.IsStarted(ctx) {
			m.resetHostWatchdogState(string(runtimeName))
			continue
		}

		// Only recover hosts that should be running. Runtimes with no loaded
		// instances (never started, or every plugin disabled) stay untouched.
		if !m.hostHasLiveInstances(pluginHost) {
			continue
		}

		m.restartHostForWatchdog(ctx, pluginHost, runtimeName)
	}
}

// hostHasLiveInstances reports whether any plugin instance expects the given
// host to be running, which is the watchdog's signal that a stopped host needs
// recovery.
func (m *Manager) hostHasLiveInstances(pluginHost Host) bool {
	for _, instance := range m.pluginInstancesSnapshot() {
		if instance.Host == pluginHost && instance.RuntimeLoaded {
			return true
		}
	}
	return false
}

// resetHostWatchdogState clears the failure counters once a host is observed
// alive again, so one healthy period is enough to forgive earlier failures.
func (m *Manager) resetHostWatchdogState(runtimeName string) {
	m.hostRestartMu.Lock()
	defer m.hostRestartMu.Unlock()

	state := m.hostWatchdogStates[runtimeName]
	if state != nil && (state.consecutiveRestarts > 0 || state.cooldownUntil > 0) {
		state.consecutiveRestarts = 0
		state.cooldownUntil = 0
	}
}

// restartHostForWatchdog restarts a dead host. It serializes with user-triggered
// restarts through hostRestartMu and re-checks the host after acquiring the
// lock, because a concurrent restart may have already recovered the runtime.
func (m *Manager) restartHostForWatchdog(ctx context.Context, pluginHost Host, runtimeName Runtime) {
	m.hostRestartMu.Lock()
	defer m.hostRestartMu.Unlock()

	now := util.GetSystemTimestamp()
	state := m.hostWatchdogStates[string(runtimeName)]
	if state == nil {
		state = &hostWatchdogState{}
		m.hostWatchdogStates[string(runtimeName)] = state
	}
	if state.cooldownUntil > now {
		logger.Warn(ctx, fmt.Sprintf("<%s HOST> watchdog restart is in cooldown, skip auto-restart", runtimeName))
		return
	}

	// Another restart (settings/uninstall) may have completed while waiting for
	// the lock; nothing to do if the host is already back.
	if pluginHost.IsStarted(ctx) {
		return
	}

	logger.Warn(ctx, fmt.Sprintf("<%s HOST> watchdog detected dead host, restarting and reloading plugins", runtimeName))
	restartErr := m.restartHostInternal(ctx, pluginHost, nil, nil)
	state.consecutiveRestarts++
	if restartErr != nil {
		logger.Error(ctx, fmt.Sprintf("<%s HOST> watchdog restart failed: %s", runtimeName, restartErr.Error()))
	} else {
		logger.Info(ctx, fmt.Sprintf("<%s HOST> watchdog restart finished", runtimeName))
	}
	if state.consecutiveRestarts >= hostWatchdogMaxConsecutiveFailures {
		state.cooldownUntil = now + int64(hostWatchdogCooldownDuration/time.Millisecond)
		logger.Error(ctx, fmt.Sprintf("<%s HOST> watchdog restart failed %d times in a row, back off auto-restart", runtimeName, state.consecutiveRestarts))
	}
}
