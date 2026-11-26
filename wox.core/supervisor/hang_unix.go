//go:build linux || darwin

package supervisor

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/pprof"
	"sync/atomic"
	"syscall"
	"time"
)

const unixUIHeartbeatInterval = 5 * time.Second

func (m *Manager) uiHeartbeatPath(pid int) string {
	return filepath.Join(m.RuntimeDirectory(), fmt.Sprintf("ui-heartbeat-%d", pid))
}

// StartUIHeartbeat publishes progress only when the UI thread dispatches a posted callback.
func (m *Manager) StartUIHeartbeat(ctx context.Context, post func(func()) error) {
	if err := m.EnsureDirectories(); err != nil {
		m.logf("prepare UI watchdog directories: %v", err)
		return
	}
	pid := os.Getpid()
	path := m.uiHeartbeatPath(pid)
	requests := make(chan os.Signal, 1)
	signal.Notify(requests, syscall.SIGUSR1)
	go func() {
		defer signal.Stop(requests)
		interval := unixUIHeartbeatInterval
		if DebugModeEnabled() {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var pending atomic.Bool
		var initialized bool
		beats := make(chan time.Time, 1)
		for {
			select {
			case <-ctx.Done():
				return
			case <-requests:
				m.captureGoroutineStacks(pid)
			case at := <-beats:
				if !initialized {
					if err := os.WriteFile(path, nil, 0600); err != nil {
						m.logf("create UI heartbeat: %v", err)
						continue
					}
					initialized = true
					continue
				}
				if err := os.Chtimes(path, at, at); err != nil {
					m.logf("update UI heartbeat: %v", err)
					initialized = false
				}
			case <-ticker.C:
				if !pending.CompareAndSwap(false, true) {
					continue
				}
				if err := post(func() {
					defer pending.Store(false)
					select {
					case beats <- time.Now():
					default:
					}
				}); err != nil {
					pending.Store(false)
				}
			}
		}
	}()
}

// watchUI observes heartbeat progress outside the application process.
func (m *Manager) watchUI(ctx context.Context, pid int) func() {
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	path := m.uiHeartbeatPath(pid)
	started := time.Now()
	go func() {
		defer close(done)
		defer os.Remove(path)
		debug := DebugModeEnabled()
		interval := unixUIHeartbeatInterval
		if debug {
			interval = time.Second
		}
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		var observation hangObservation
		var previous time.Time
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				info, err := os.Stat(path)
				if err != nil || info.ModTime().Before(started) {
					continue // The child may not have entered the UI loop yet.
				}
				now := time.Now()
				if !observation.lastSample.IsZero() && now.Round(0).Sub(observation.lastSample.Round(0))-now.Sub(observation.lastSample) > 3*time.Second {
					observation = hangObservation{}
				}
				heartbeat := info.ModTime()
				if observation.observe(now, heartbeat.Equal(previous) && now.Sub(heartbeat) >= interval, debug) && ctx.Err() == nil {
					if err := syscall.Kill(pid, syscall.SIGUSR1); err != nil {
						m.logf("request UI hang stacks: pid=%d error=%v", pid, err)
					} else {
						m.logf("requested UI hang stacks: pid=%d", pid)
					}
				}
				previous = heartbeat
			}
		}
	}()
	return func() { cancel(); <-done }
}

// captureGoroutineStacks publishes the complete stack file atomically for local diagnosis.
func (m *Manager) captureGoroutineStacks(pid int) {
	path := filepath.Join(m.CrashDumpsDirectory(), fmt.Sprintf("wox-%d-hang-%d.txt", pid, time.Now().UnixMilli()))
	file, err := os.OpenFile(path+".tmp", os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err == nil {
		err = pprof.Lookup("goroutine").WriteTo(file, 2)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(path+".tmp", path)
		}
		if err != nil {
			_ = os.Remove(path + ".tmp")
		}
	}
	if err != nil {
		m.logf("capture UI hang stacks: pid=%d error=%v", pid, err)
		return
	}
	m.retainNewestCrashFiles(m.CrashDumpsDirectory(), ".txt", retainedCrashArtifacts)
	m.logf("captured UI hang stacks: pid=%d path=%s", pid, path)
}
