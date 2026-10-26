package plugin

import (
	"context"
	"strings"
)

func (m *Manager) Stop(ctx context.Context) {
	m.lifecycle.Stop(ctx)
}

func (m *Manager) pluginInstancesSnapshot() []*Instance {
	return m.lifecycle.pluginInstancesSnapshot()
}

func (m *Manager) ReloadPlugin(ctx context.Context, metadata Metadata) error {
	return m.lifecycle.ReloadPlugin(ctx, metadata)
}

func (m *Manager) LoadPlugin(ctx context.Context, pluginDirectory string) error {
	return m.lifecycle.LoadPlugin(ctx, pluginDirectory)
}

func (m *Manager) LoadRuntimePlugin(ctx context.Context, metadata Metadata) error {
	return m.lifecycle.LoadRuntimePlugin(ctx, metadata)
}

func (m *Manager) UnloadPlugin(ctx context.Context, pluginInstance *Instance) {
	m.lifecycle.UnloadPlugin(ctx, pluginInstance)
}

// DisablePlugin releases the runtime before refreshing plugin-dependent UI.
func (m *Manager) DisablePlugin(ctx context.Context, pluginId string) error {
	if err := m.lifecycle.DisablePlugin(ctx, pluginId); err != nil {
		return err
	}
	if m.ui != nil {
		m.ui.ReloadChatResources(ctx, "mentions")
	}
	if strings.EqualFold(pluginId, AttentionPluginID) {
		// Hide the query-box unread badge as soon as the inbox plugin is turned off.
		PublishAttentionUnreadCount(ctx)
	}
	return nil
}

// EnablePlugin refreshes UI only after the enabled runtime initializes successfully.
func (m *Manager) EnablePlugin(ctx context.Context, pluginId string) error {
	if err := m.lifecycle.EnablePlugin(ctx, pluginId); err != nil {
		return err
	}
	if m.ui != nil {
		m.ui.ReloadChatResources(ctx, "mentions")
	}
	if strings.EqualFold(pluginId, AttentionPluginID) {
		PublishAttentionUnreadCount(ctx)
	}
	return nil
}

func (m *Manager) RestartHostForRuntime(ctx context.Context, runtime Runtime, skipPluginIDs []string, progressCallback UninstallProgressCallback) error {
	return m.lifecycle.RestartHostForRuntime(ctx, runtime, skipPluginIDs, progressCallback)
}

func (m *Manager) RefreshHostForRuntime(ctx context.Context, runtime Runtime) error {
	return m.lifecycle.RefreshHostForRuntime(ctx, runtime)
}

func (m *Manager) ParseMetadata(ctx context.Context, pluginDirectory string) (Metadata, error) {
	return m.lifecycle.ParseMetadata(ctx, pluginDirectory)
}

func (m *Manager) WaitForSystemPlugins() {
	m.lifecycle.WaitForSystemPlugins()
}

func (m *Manager) GetPluginInstances() []*Instance {
	return m.lifecycle.GetPluginInstances()
}

func (m *Manager) GetPluginInstanceById(pluginId string) *Instance {
	return m.lifecycle.GetPluginInstanceById(pluginId)
}

func (m *Manager) GetSystemPlugin(pluginId string) SystemPlugin {
	return m.lifecycle.GetSystemPlugin(pluginId)
}

func (m *Manager) IsHostStarted(ctx context.Context, runtime Runtime) bool {
	return m.lifecycle.IsHostStarted(ctx, runtime)
}

func (m *Manager) EnsureHostStarted(ctx context.Context, runtime Runtime) error {
	return m.lifecycle.EnsureHostStarted(ctx, runtime)
}

func (m *Manager) SuspendOfflinePlugins(ctx context.Context) {
	m.lifecycle.SuspendOfflinePlugins(ctx)
}

func (m *Manager) ResumeOfflinePlugins(ctx context.Context) error {
	return m.lifecycle.ResumeOfflinePlugins(ctx)
}

func (m *Manager) IgnoreSingleFileWatch(filePath string) {
	m.lifecycle.IgnoreSingleFileWatch(filePath)
}

func (m *Manager) ParseScriptMetadata(ctx context.Context, scriptPath string) (Metadata, error) {
	return m.lifecycle.ParseScriptMetadata(ctx, scriptPath)
}

func (m *Manager) ParseSingleFilePluginMetadata(ctx context.Context, filePath string) (Metadata, error) {
	return m.lifecycle.ParseSingleFilePluginMetadata(ctx, filePath)
}
