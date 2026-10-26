package plugin

import (
	"context"
	"testing"
	"time"

	"wox/util"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestLifecycleStopOwnsMonitorAndTimers checks teardown without a Manager or real host process.
func TestLifecycleStopOwnsMonitorAndTimers(t *testing.T) {
	logger = util.GetLogger()
	lifecycle := newPluginLifecycle()
	previous := AllHosts
	t.Cleanup(func() { AllHosts = previous })
	host := &lifecycleStopHost{onStop: func() {
		select {
		case <-lifecycle.hostWatchdogDone:
		default:
			t.Error("host stopped before watchdog exited")
		}
	}}
	AllHosts = []Host{host}
	lifecycle.startHostWatchdog(context.Background())
	scriptTimer := time.AfterFunc(time.Hour, func() { t.Error("script reloaded after stop") })
	singleFileTimer := time.AfterFunc(time.Hour, func() { t.Error("single-file plugin reloaded after stop") })
	t.Cleanup(func() { scriptTimer.Stop(); singleFileTimer.Stop() })
	lifecycle.scriptReloadTimers.Store("script", scriptTimer)
	lifecycle.singleFileReloadTimers.Store("single", singleFileTimer)
	lifecycle.Stop(context.Background())
	assert.False(t, scriptTimer.Stop())
	assert.False(t, singleFileTimer.Stop())
	assert.Zero(t, lifecycle.scriptReloadTimers.Len())
	assert.Zero(t, lifecycle.singleFileReloadTimers.Len())
	assert.Equal(t, 1, host.stopCount)
}

type lifecycleStopHost struct {
	fakeHost
	onStop func()
}

func (h *lifecycleStopHost) Stop(ctx context.Context) {
	h.onStop()
	h.fakeHost.Stop(ctx)
}

// TestLifecycleRestartReleasesOnlyReplacedRuntime verifies the cleanup boundary on host replacement.
func TestLifecycleRestartReleasesOnlyReplacedRuntime(t *testing.T) {
	logger = util.GetLogger()
	lifecycle := newPluginLifecycle()
	host := &fakeHost{}
	replaced := &Instance{Metadata: Metadata{Id: "replaced", Runtime: "PYTHON"}, Host: host}
	retained := &Instance{Metadata: Metadata{Id: "retained", Runtime: "NODEJS"}}
	lifecycle.appendPluginInstance(replaced)
	lifecycle.appendPluginInstance(retained)
	var released []string
	lifecycle.releasePresentation = func(id string) { released = append(released, id) }
	require.NoError(t, lifecycle.restartHostInternal(context.Background(), host, []string{"replaced"}, nil))
	assert.Equal(t, []string{"replaced"}, released)
	assert.Equal(t, []*Instance{retained}, lifecycle.pluginInstancesSnapshot())
	assert.Equal(t, 1, host.startCount)
}
