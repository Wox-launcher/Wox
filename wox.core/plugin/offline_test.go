package plugin

import (
	"context"
	"errors"
	"testing"
	"wox/network"
)

// TestOfflineSuspensionKeepsInstalledMetadata verifies runtime suspension and watchdog admission together.
func TestOfflineSuspensionKeepsInstalledMetadata(t *testing.T) {
	manager := newWatchdogTestManager()
	host := &fakeHost{started: true}
	instance := newInitTestInstance(&fakeLifecyclePlugin{})
	instance.Host = host
	instance.RuntimeLoaded = true
	manager.instances = []*Instance{instance}
	previous := AllHosts
	AllHosts = []Host{host}
	network.Default.SetOffline(true)
	t.Cleanup(func() { network.Default.SetOffline(false); AllHosts = previous })
	manager.SuspendOfflinePlugins(context.Background())
	if host.stopCount != 1 || instance.RuntimeLoaded || instance.Plugin != nil {
		t.Fatal("runtime still active")
	}
	if len(manager.GetPluginInstances()) != 1 {
		t.Fatal("installed metadata removed")
	}
	manager.checkHostsHealth(context.Background())
	if host.startCount != 0 {
		t.Fatal("watchdog restarted offline host")
	}
	if !errors.Is(ensureThirdPartyPluginsEnabled(context.Background()), network.ErrOffline) {
		t.Fatal("plugin loading not gated")
	}
	if manager.canOperateQuery(context.Background(), instance, Query{}) {
		t.Fatal("offline plugin can query")
	}
}

// TestOfflineSkipsQueuedPluginInit covers a toggle after loading but before Init runs.
func TestOfflineSkipsQueuedPluginInit(t *testing.T) {
	manager := newInitTestManager()
	target := &fakeLifecyclePlugin{}
	instance := newInitTestInstance(target)
	instance.Host = &fakeHost{}
	network.Default.SetOffline(true)
	defer network.Default.SetOffline(false)
	manager.initPlugin(context.Background(), instance)
	if target.initCalls.Load() != 0 {
		t.Fatal("third party initialization executed")
	}
	if !errors.Is(instance.WaitInit(context.Background()), network.ErrOffline) {
		t.Fatal("initialization waiter was not released")
	}
}

// TestOfflineRegistrationAndResumeKeepPreferences covers a cold offline launch and repeated toggles.
func TestOfflineRegistrationAndResumeKeepPreferences(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := newWatchdogTestManager()
	host := &countingLoadHost{}
	previous := AllHosts
	AllHosts = []Host{host}
	t.Cleanup(func() { network.Default.SetOffline(false); AllHosts = previous })
	network.Default.SetOffline(true)
	for _, id := range []string{"enabled-offline-plugin", "disabled-offline-plugin"} {
		if err := manager.registerOfflinePlugin(context.Background(), host, Metadata{Id: id, Name: "Offline test plugin", Runtime: string(PLUGIN_RUNTIME_PYTHON)}); err != nil {
			t.Fatal(err)
		}
	}
	enabled := manager.GetPluginInstanceById("enabled-offline-plugin")
	disabled := manager.GetPluginInstanceById("disabled-offline-plugin")
	if err := disabled.Setting.Disabled.SetLocal(true); err != nil {
		t.Fatal(err)
	}
	if host.loadCalls.Load() != 0 {
		t.Fatal("offline metadata registration loaded plugin code")
	}
	for cycle := int32(1); cycle <= 2; cycle++ {
		network.Default.SetOffline(false)
		if err := manager.ResumeOfflinePlugins(context.Background()); err != nil {
			t.Fatal(err)
		}
		if !enabled.RuntimeLoaded || disabled.RuntimeLoaded || host.loadCalls.Load() != cycle {
			t.Fatal("resume ignored saved plugin preferences")
		}
		network.Default.SetOffline(true)
		manager.SuspendOfflinePlugins(context.Background())
		if enabled.Setting.Disabled.Get() || !disabled.Setting.Disabled.Get() {
			t.Fatal("suspension rewrote saved preferences")
		}
	}
}

// TestOfflineResumeContinuesAfterInitFailure keeps broken plugins from blocking healthy ones.
func TestOfflineResumeContinuesAfterInitFailure(t *testing.T) {
	initPluginManagerLoadTest(t)
	manager := newWatchdogTestManager()
	host := &countingLoadHost{}
	previousHosts := AllHosts
	previousOffline := network.IsOffline()
	AllHosts = nil
	network.Default.SetOffline(false)
	t.Cleanup(func() { AllHosts = previousHosts; network.Default.SetOffline(previousOffline) })
	broken := errors.New("broken plugin init")
	healthy := &fakeLifecyclePlugin{}
	for index, target := range []*fakeLifecyclePlugin{{initErr: broken}, healthy} {
		id := []string{"broken-resume", "healthy-resume"}[index]
		if err := manager.registerOfflinePlugin(context.Background(), host, Metadata{Id: id, Name: "Resume test"}); err != nil {
			t.Fatal(err)
		}
		instance := manager.GetPluginInstanceById(id)
		instance.Plugin = target
		instance.RuntimeLoaded = true
	}
	if err := manager.ResumeOfflinePlugins(context.Background()); !errors.Is(err, broken) {
		t.Fatalf("missing plugin error: %v", err)
	}
	if healthy.initCalls.Load() != 1 {
		t.Fatal("healthy plugin was not resumed")
	}
	if network.IsOffline() {
		t.Fatal("plugin failure changed network policy")
	}
}
