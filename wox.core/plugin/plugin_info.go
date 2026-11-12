package plugin

import (
	"context"
	"strings"
	"wox/common"
	"wox/network"
	"wox/setting"
)

type GetPluginInfoOption struct {
	PluginId string
}

// GetPluginInfoResult distinguishes persisted enablement from runtime readiness.
type GetPluginInfoResult struct {
	PluginId        string
	Installed       bool
	Enabled         bool
	Ready           bool
	Name            string
	Version         string
	Icon            *common.WoxImage `json:",omitempty"`
	TriggerKeywords []string
	ErrMsg          string `json:",omitempty"`
}

func (a *APIImpl) GetPluginInfo(ctx context.Context, option GetPluginInfoOption) GetPluginInfoResult {
	return GetPluginManager().getPluginInfo(ctx, option)
}

// GetLoadedPluginInfo reports the registered instance without discovering plugin files.
func (m *Manager) GetLoadedPluginInfo(ctx context.Context, pluginID string) GetPluginInfoResult {
	instance := m.GetPluginInstanceById(pluginID)
	if instance == nil {
		return GetPluginInfoResult{PluginId: pluginID, TriggerKeywords: []string{}}
	}
	result := pluginInfoFromMetadata(ctx, &instance.Metadata, instance.PluginDirectory, instance.GetTriggerKeywords())
	result.Enabled = !pluginInstanceDisabled(instance)
	result.Ready = result.Enabled && instance.pluginToolsCallable() &&
		(instance.Host == nil || (!network.IsOffline() && instance.Host.IsStarted(ctx)))
	return result
}

// getPluginInfo prefers the live instance and falls back to local metadata when loading failed.
func (m *Manager) getPluginInfo(ctx context.Context, option GetPluginInfoOption) GetPluginInfoResult {
	id := strings.TrimSpace(option.PluginId)
	if id == "" {
		return GetPluginInfoResult{TriggerKeywords: []string{}, ErrMsg: "plugin ID is required"}
	}
	if result := m.GetLoadedPluginInfo(ctx, id); result.Installed {
		return result
	}

	metadataList, discoveryErr := m.lifecycle.discoverUserPluginMetadata(ctx)
	for _, metadata := range metadataList {
		if !strings.EqualFold(metadata.Id, id) {
			continue
		}
		disabled, keywords, err := setting.ReadPluginRoutingSettings(metadata.Id)
		if len(keywords) == 0 {
			keywords = metadata.TriggerKeywords
		}
		result := pluginInfoFromMetadata(ctx, &metadata, metadata.Directory, keywords)
		if err != nil {
			result.ErrMsg = err.Error()
		} else {
			result.Enabled = !disabled
		}
		return result
	}
	result := GetPluginInfoResult{PluginId: id, TriggerKeywords: []string{}}
	if discoveryErr != nil {
		result.ErrMsg = discoveryErr.Error()
	}
	return result
}

// pluginInfoFromMetadata resolves relative icons against the owner and copies the routing snapshot.
func pluginInfoFromMetadata(ctx context.Context, metadata *Metadata, directory string, keywords []string) GetPluginInfoResult {
	result := GetPluginInfoResult{
		PluginId:        metadata.Id,
		Installed:       true,
		Name:            metadata.GetName(ctx),
		Version:         metadata.Version,
		TriggerKeywords: append([]string{}, keywords...),
	}
	if icon := metadata.GetIconOrDefault(directory, common.WoxImage{}); !icon.IsEmpty() {
		result.Icon = &icon
	}
	return result
}
