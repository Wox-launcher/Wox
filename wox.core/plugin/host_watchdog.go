package plugin

import (
	"context"
	"fmt"
	"time"

	"wox/network"
	"wox/util"
)

const (
	// hostWatchdogCheckInterval is how often the manager verifies that shared
	// runtime host processes (python/nodejs) are still alive. All plugins of a
	// runtime live in one host process, so one crashed process takes the whole
	// runtime down with it; the watchdog restarts the host instead of leaving
	// the runtime broken until the next manual plugin load.
	hostWatchdogCheckInterval = 5 * time.Second
	hostWatchdogPingTimeout   = 3 * time.Second
	hostWatchdogPingThreshold = 2

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
	pingFailures        int
	consecutiveRestarts int
	cooldownUntil       int64
}

// startHostWatchdog launches the shared-host health monitor. The watchdog only
// acts when a runtime host was started and still has loaded plugin instances;
// a stopped host with no plugins is left alone.
func (l *pluginLifecycle) startHostWatchdog(ctx context.Context) {
	l.hostWatchdogLifecycle.Lock()
	if l.hostWatchdogStarted {
		l.hostWatchdogLifecycle.Unlock()
		return
	}
	if l.hostWatchdogStop == nil {
		l.hostWatchdogStop = make(chan struct{})
	}
	if l.hostWatchdogDone == nil {
		l.hostWatchdogDone = make(chan struct{})
	}
	l.hostWatchdogStarted = true
	l.hostWatchdogLifecycle.Unlock()

	util.Go(ctx, "start host watchdog", func() {
		defer close(l.hostWatchdogDone)
		logger.Info(ctx, "start host watchdog")
		ticker := time.NewTicker(hostWatchdogCheckInterval)
		defer ticker.Stop()
		for {
			select {
			case <-l.hostWatchdogStop:
				logger.Info(ctx, "host watchdog stopped")
				return
			case <-ticker.C:
				l.checkHostsHealth(util.NewTraceContext())
			case <-l.hostWatchdogWake:
				l.checkHostsHealth(util.NewTraceContext())
			}
		}
	})
}

// CheckHostsAfterWake coalesces resume checks into the existing watchdog loop.
func (m *Manager) CheckHostsAfterWake() {
	select {
	case m.lifecycle.hostWatchdogWake <- struct{}{}:
	default:
	}
}

// stopHostWatchdog stops the health monitor so Wox shutdown does not try to
// restart hosts that pluginLifecycle.Stop is intentionally tearing down.
func (l *pluginLifecycle) stopHostWatchdog() {
	l.hostWatchdogLifecycle.Lock()
	if !l.hostWatchdogStarted {
		l.hostWatchdogLifecycle.Unlock()
		return
	}
	select {
	case <-l.hostWatchdogStop:
	default:
		close(l.hostWatchdogStop)
	}
	done := l.hostWatchdogDone
	l.hostWatchdogLifecycle.Unlock()
	<-done
}

// checkHostsHealth verifies every shared runtime host is still alive and
// recovers dead hosts by restarting them together with their plugins.
func (l *pluginLifecycle) checkHostsHealth(ctx context.Context) {
	if network.IsOffline() {
		return
	}
	for _, pluginHost := range AllHosts {
		runtimeName := pluginHost.GetRuntime(ctx)
		if runtimeName == PLUGIN_RUNTIME_GO || runtimeName == PLUGIN_RUNTIME_SCRIPT {
			continue
		}

		if pluginHost.IsStarted(ctx) {
			if !l.hostHasLiveInstances(pluginHost) {
				l.resetHostWatchdogState(string(runtimeName))
				continue
			}
			probe, supported := pluginHost.(interface{ Ping(context.Context) error })
			if supported {
				pingCtx, cancel := context.WithTimeout(ctx, hostWatchdogPingTimeout)
				err := probe.Ping(pingCtx)
				cancel()
				if err != nil {
					if l.recordHostPingFailure(runtimeName) >= hostWatchdogPingThreshold {
						l.restartHostForWatchdog(ctx, pluginHost, runtimeName, true)
					}
					continue
				}
			}
			l.resetHostWatchdogState(string(runtimeName))
			continue
		}

		// Only recover hosts that should be running. Runtimes with no loaded
		// instances (never started, or every plugin disabled) stay untouched.
		if !l.hostHasLiveInstances(pluginHost) {
			continue
		}

		l.restartHostForWatchdog(ctx, pluginHost, runtimeName, false)
	}
}

// hostHasLiveInstances reports whether any plugin instance expects the given
// host to be running, which is the watchdog's signal that a stopped host needs
// recovery.
func (l *pluginLifecycle) hostHasLiveInstances(pluginHost Host) bool {
	for _, instance := range l.pluginInstancesSnapshot() {
		if instance.Host == pluginHost && instance.RuntimeLoaded {
			return true
		}
	}
	return false
}

// resetHostWatchdogState clears the failure counters once a host is observed
// alive again, so one healthy period is enough to forgive earlier failures.
func (l *pluginLifecycle) resetHostWatchdogState(runtimeName string) {
	l.hostRestartMu.Lock()
	defer l.hostRestartMu.Unlock()

	state := l.hostWatchdogStates[runtimeName]
	if state != nil {
		state.pingFailures = 0
		state.consecutiveRestarts = 0
		state.cooldownUntil = 0
	}
}

func (l *pluginLifecycle) recordHostPingFailure(runtimeName Runtime) int {
	l.hostRestartMu.Lock()
	defer l.hostRestartMu.Unlock()
	state := l.hostWatchdogStates[string(runtimeName)]
	if state == nil {
		state = &hostWatchdogState{}
		l.hostWatchdogStates[string(runtimeName)] = state
	}
	state.pingFailures++
	return state.pingFailures
}

// restartHostForWatchdog restarts a dead host. It serializes with user-triggered
// restarts through hostRestartMu and re-checks the host after acquiring the
// lock, because a concurrent restart may have already recovered the runtime.
func (l *pluginLifecycle) restartHostForWatchdog(ctx context.Context, pluginHost Host, runtimeName Runtime, unresponsive bool) {
	l.hostRestartMu.Lock()
	defer l.hostRestartMu.Unlock()

	now := util.GetSystemTimestamp()
	state := l.hostWatchdogStates[string(runtimeName)]
	if state == nil {
		state = &hostWatchdogState{}
		l.hostWatchdogStates[string(runtimeName)] = state
	}
	if state.cooldownUntil > now {
		logger.Warn(ctx, fmt.Sprintf("<%s HOST> watchdog restart is in cooldown, skip auto-restart", runtimeName))
		return
	}

	// Another restart (settings/uninstall) may have completed while waiting for
	// the lock; nothing to do if the host is already back.
	if unresponsive && state.pingFailures < hostWatchdogPingThreshold {
		return
	}
	if !unresponsive && pluginHost.IsStarted(ctx) {
		return
	}

	state.pingFailures = 0
	logger.Warn(ctx, fmt.Sprintf("<%s HOST> watchdog detected an unavailable host, restarting and reloading plugins", runtimeName))
	restartErr := l.restartHostInternal(ctx, pluginHost, nil, nil)
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

// CheckWatchersAfterWake lets system plugins revalidate owned subscriptions.
func (m *Manager) CheckWatchersAfterWake(ctx context.Context) {
	for _, instance := range m.GetPluginInstances() {
		if instance.IsSystemPlugin {
			if watcher, ok := instance.Plugin.(interface{ CheckAfterWake(context.Context) }); ok {
				watcher.CheckAfterWake(ctx)
			}
		}
	}
}
