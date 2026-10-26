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
func (l *pluginLifecycle) SuspendOfflinePlugins(ctx context.Context) {
	l.offlineLifecycle.Lock()
	defer l.offlineLifecycle.Unlock()
	for _, instance := range l.pluginInstancesSnapshot() {
		if instance.Host != nil {
			l.deactivatePlugin(ctx, instance)
		}
	}
	for _, host := range AllHosts {
		if host.GetRuntime(ctx) != PLUGIN_RUNTIME_GO {
			host.Stop(ctx)
		}
	}
}

// ResumeOfflinePlugins reactivates known instances and discovers plugins skipped at offline startup.
func (l *pluginLifecycle) ResumeOfflinePlugins(ctx context.Context) error {
	if util.IsThirdPartyPluginsDisabled() {
		return nil
	}
	var failures []error
	for _, instance := range l.pluginInstancesSnapshot() {
		if instance.Host != nil && !instance.Setting.Disabled.Get() {
			if err := l.activatePlugin(ctx, instance); err != nil {
				failures = append(failures, fmt.Errorf("plugin %s: %w", instance.Metadata.Id, err))
			}
		}
	}
	if network.IsOffline() {
		return errors.Join(append(failures, network.ErrOffline)...)
	}
	return errors.Join(append(failures, l.loadUserPlugins(ctx, true))...)
}

// registerOfflinePlugin exposes installed metadata without executing any third-party code.
func (l *pluginLifecycle) registerOfflinePlugin(ctx context.Context, host Host, metadata Metadata) error {
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
	l.appendPluginInstance(instance)
	return nil
}
