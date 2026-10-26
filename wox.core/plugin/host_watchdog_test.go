package plugin

import (
	"context"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"

	"wox/util"
)

// fakeHost implements Host for watchdog tests, recording lifecycle calls
// without spawning any real runtime process.
type fakeHost struct {
	started    bool
	stopCount  int
	startCount int
	status     RuntimeHostStatus
}

func (f *fakeHost) GetRuntime(ctx context.Context) Runtime {
	return PLUGIN_RUNTIME_PYTHON
}

func (f *fakeHost) Start(ctx context.Context) error {
	f.startCount++
	f.started = true
	return nil
}

func (f *fakeHost) Stop(ctx context.Context) {
	f.stopCount++
}

func (f *fakeHost) IsStarted(ctx context.Context) bool {
	return f.started
}

func (f *fakeHost) RuntimeStatus(ctx context.Context) RuntimeHostStatus {
	return f.status
}

func (f *fakeHost) LoadPlugin(ctx context.Context, metadata Metadata, pluginDirectory string) (Plugin, error) {
	return nil, nil
}

func (f *fakeHost) UnloadPlugin(ctx context.Context, metadata Metadata) {}

// newWatchdogTestManager returns a manager wired for watchdog tests with the
// package logger available and no plugin instances.
func newWatchdogTestManager() *Manager {
	logger = util.GetLogger()
	return &Manager{lifecycle: pluginLifecycle{hostWatchdogStates: map[string]*hostWatchdogState{}}}
}

func TestHostHasLiveInstances(t *testing.T) {
	m := &Manager{}
	hostA := &fakeHost{}
	hostB := &fakeHost{}

	assert.False(t, m.lifecycle.hostHasLiveInstances(hostA))

	m.lifecycle.instances = []*Instance{
		{Host: hostB, RuntimeLoaded: true},
	}
	assert.False(t, m.lifecycle.hostHasLiveInstances(hostA))
	assert.True(t, m.lifecycle.hostHasLiveInstances(hostB))

	// unloaded/disabled instances must not require host recovery
	m.lifecycle.instances = append(m.lifecycle.instances, &Instance{Host: hostA, RuntimeLoaded: false})
	assert.False(t, m.lifecycle.hostHasLiveInstances(hostA))
}

func TestWatchdogRestartsDeadHostAndResetsOnRecovery(t *testing.T) {
	host := &fakeHost{started: false}
	m := newWatchdogTestManager()

	// checkHostsHealth only iterates AllHosts, so register the fake host there
	// for the recovery check and remove it afterwards.
	AllHosts = append(AllHosts, host)
	defer func() {
		var remaining []Host
		for _, h := range AllHosts {
			if h != host {
				remaining = append(remaining, h)
			}
		}
		AllHosts = remaining
	}()

	m.lifecycle.restartHostForWatchdog(context.Background(), host, PLUGIN_RUNTIME_PYTHON)

	assert.Equal(t, 1, host.stopCount)
	assert.Equal(t, 1, host.startCount)
	assert.True(t, host.IsStarted(context.Background()))
	assert.Equal(t, 1, m.lifecycle.hostWatchdogStates["PYTHON"].consecutiveRestarts)

	// A healthy host observed at the next check clears the failure counters.
	m.lifecycle.checkHostsHealth(context.Background())
	assert.Equal(t, 0, m.lifecycle.hostWatchdogStates["PYTHON"].consecutiveRestarts)
	assert.Equal(t, int64(0), m.lifecycle.hostWatchdogStates["PYTHON"].cooldownUntil)
}

func TestWatchdogBacksOffAfterRepeatedFastFailures(t *testing.T) {
	host := &fakeHost{started: false}
	m := newWatchdogTestManager()

	// Simulate a plugin that keeps killing the host right after each restart.
	for i := 0; i < hostWatchdogMaxConsecutiveFailures; i++ {
		host.started = false
		m.lifecycle.restartHostForWatchdog(context.Background(), host, PLUGIN_RUNTIME_PYTHON)
	}

	state := m.lifecycle.hostWatchdogStates["PYTHON"]
	assert.Equal(t, hostWatchdogMaxConsecutiveFailures, state.consecutiveRestarts)
	assert.Greater(t, state.cooldownUntil, int64(0))

	// While in cooldown, a still-dead host is not restarted again.
	stopBefore := host.stopCount
	startBefore := host.startCount
	m.lifecycle.restartHostForWatchdog(context.Background(), host, PLUGIN_RUNTIME_PYTHON)
	assert.Equal(t, stopBefore, host.stopCount)
	assert.Equal(t, startBefore, host.startCount)
}

func TestWatchdogIgnoresHostsWithoutLiveInstances(t *testing.T) {
	m := newWatchdogTestManager()

	// checkHostsHealth also iterates the real AllHosts entries; without loaded
	// instances the watchdog must not try to start any host.
	m.lifecycle.checkHostsHealth(context.Background())
	assert.Empty(t, m.lifecycle.instances)
}

func TestHostHasLiveInstancesDuringInstanceChanges(t *testing.T) {
	m := newWatchdogTestManager()
	host := &fakeHost{}
	var workers sync.WaitGroup

	workers.Add(2)
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.lifecycle.appendPluginInstance(&Instance{Host: host, RuntimeLoaded: true})
			m.lifecycle.removePluginInstances(func(instance *Instance) bool {
				return instance.Host == host
			})
		}
	}()
	go func() {
		defer workers.Done()
		for i := 0; i < 100; i++ {
			m.lifecycle.hostHasLiveInstances(host)
		}
	}()

	workers.Wait()
}

func TestStopHostWatchdogWaitsForExit(t *testing.T) {
	m := newWatchdogTestManager()
	m.lifecycle.startHostWatchdog(context.Background())
	m.lifecycle.stopHostWatchdog()

	select {
	case <-m.lifecycle.hostWatchdogDone:
	default:
		t.Fatal("host watchdog is still running after stop")
	}
}
