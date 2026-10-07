package plugin

import (
	"context"
	"errors"
	"fmt"
	"wox/network"
	"wox/setting"
	"wox/util"
)

// SuspendOfflinePlugins releases third-party runtimes without changing saved plugin preferences.
func (m *Manager) SuspendOfflinePlugins(ctx context.Context) {
	m.offlineLifecycle.Lock()
	defer m.offlineLifecycle.Unlock()
	for _, instance := range m.pluginInstancesSnapshot() {
		if instance.Host != nil {
			m.deactivatePlugin(ctx, instance)
		}
	}
	for _, host := range AllHosts {
		if host.GetRuntime(ctx) != PLUGIN_RUNTIME_GO {
			host.Stop(ctx)
		}
	}
}

// ResumeOfflinePlugins reactivates known instances and discovers plugins skipped at offline startup.
func (m *Manager) ResumeOfflinePlugins(ctx context.Context) error {
	if util.IsThirdPartyPluginsDisabled() {
		return nil
	}
	var failures []error
	for _, instance := range m.pluginInstancesSnapshot() {
		if instance.Host != nil && !instance.Setting.Disabled.Get() {
			if err := m.activatePlugin(ctx, instance); err != nil {
				failures = append(failures, fmt.Errorf("plugin %s: %w", instance.Metadata.Id, err))
			}
		}
	}
	if network.IsOffline() {
		return errors.Join(append(failures, network.ErrOffline)...)
	}
	return errors.Join(append(failures, m.loadUserPlugins(ctx, true))...)
}

// registerOfflinePlugin exposes installed metadata without executing any third-party code.
func (m *Manager) registerOfflinePlugin(ctx context.Context, host Host, metadata Metadata) error {
	if err := ensureWoxVersionSupported(metadata.GetName(ctx), metadata.MinWoxVersion); err != nil {
		return err
	}
	settings, err := setting.GetSettingManager().LoadPluginSetting(ctx, metadata.Id, metadata.SettingDefinitions.ToMap())
	if err != nil {
		return err
	}
	instance := &Instance{Metadata: metadata, Host: host, PluginDirectory: metadata.Directory, Setting: settings, IsDevPlugin: metadata.IsDev, DevPluginDirectory: metadata.DevPluginDirectory}
	instance.API = NewAPI(instance)
	instance.beginInitCycle()
	instance.finishInit(false, nil)
	m.appendPluginInstance(instance)
	return nil
}
