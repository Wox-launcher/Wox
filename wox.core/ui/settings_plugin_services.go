package ui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/ui/contract"
	"wox/ui/dto"

	"github.com/jinzhu/copier"
	"github.com/samber/lo"
)

// Plugins returns one plugin catalog without exposing the HTTP DTO boundary.
func (s *CoreServices) Plugins(ctx context.Context, sessionID string, catalog contract.PluginCatalog) ([]contract.PluginCatalogItem, error) {
	ctx = uiServiceContext(ctx, sessionID)
	var (
		plugins []dto.PluginDto
		err     error
	)
	switch catalog {
	case contract.PluginCatalogInstalled:
		plugins, err = getInstalledPluginDTOs(ctx)
	case contract.PluginCatalogStore:
		plugins, err = getStorePluginDTOs(ctx)
	default:
		return nil, fmt.Errorf("unsupported plugin catalog %q", catalog)
	}
	if err != nil {
		return nil, err
	}

	result := make([]contract.PluginCatalogItem, len(plugins))
	for index, item := range plugins {
		result[index] = contract.PluginCatalogItem{
			ID: item.Id, Name: item.Name, Description: item.Description, Author: item.Author, Website: item.Website, Version: item.Version,
			Runtime: item.Runtime, Entry: item.Entry, PluginDirectory: item.PluginDirectory, Icon: item.Icon,
			ScreenshotURLs: append([]string(nil), item.ScreenshotUrls...), TriggerKeywords: append([]string(nil), item.TriggerKeywords...),
			Commands: append([]plugin.MetadataCommand(nil), item.Commands...), SupportedOS: append([]string(nil), item.SupportedOS...),
			Features: append([]plugin.MetadataFeature(nil), item.Features...), Glances: append([]plugin.MetadataGlance(nil), item.Glances...),
			IsSystem: item.IsSystem, IsDev: item.IsDev, IsInstalled: item.IsInstalled, IsDisable: item.IsDisable, IsUpgradable: item.IsUpgradable,
			SettingDefinitions: item.SettingDefinitions,
			Setting: contract.PluginSetting{
				Disabled: item.Setting.Disabled, TriggerKeywords: append([]string(nil), item.Setting.TriggerKeywords...), Settings: cloneStringMap(item.Setting.Settings),
			},
		}
	}
	return result, nil
}

// OperatePlugin performs one plugin lifecycle action against core-owned managers.
func (s *CoreServices) OperatePlugin(ctx context.Context, sessionID string, pluginID string, operation contract.PluginOperation) error {
	ctx = uiServiceContext(ctx, sessionID)
	switch operation {
	case contract.PluginOperationInstall:
		manifest, exists := lo.Find(plugin.GetStoreManager().GetStorePluginManifests(ctx), func(item plugin.StorePluginManifest) bool {
			return item.Id == pluginID
		})
		if !exists {
			return fmt.Errorf("plugin %q not found in the store", pluginID)
		}
		if err := plugin.GetStoreManager().Install(ctx, manifest); err != nil {
			return fmt.Errorf("install plugin %q: %w", manifest.GetName(ctx), err)
		}
		return nil
	case contract.PluginOperationUninstall:
		instance, exists := findPluginInstance(pluginID)
		if !exists {
			return fmt.Errorf("plugin %q is not installed", pluginID)
		}
		if err := plugin.GetStoreManager().Uninstall(ctx, instance, false); err != nil {
			return fmt.Errorf("uninstall plugin %q: %w", pluginID, err)
		}
		return nil
	case contract.PluginOperationEnable:
		return plugin.GetPluginManager().EnablePlugin(ctx, pluginID)
	case contract.PluginOperationDisable:
		return plugin.GetPluginManager().DisablePlugin(ctx, pluginID)
	default:
		return fmt.Errorf("unsupported plugin operation %q", operation)
	}
}

// UpdatePluginSettings persists a deterministic batch of changes for one installed plugin.
func (s *CoreServices) UpdatePluginSettings(ctx context.Context, sessionID string, pluginID string, values map[string]string) error {
	ctx = uiServiceContext(ctx, sessionID)
	instance, exists := findPluginInstance(pluginID)
	if !exists {
		return fmt.Errorf("plugin %q is not installed", pluginID)
	}

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := values[key]
		switch key {
		case "Disabled":
			var err error
			if value == "true" {
				err = plugin.GetPluginManager().DisablePlugin(ctx, pluginID)
			} else {
				err = plugin.GetPluginManager().EnablePlugin(ctx, pluginID)
			}
			if err != nil {
				return fmt.Errorf("update plugin setting %q: %w", key, err)
			}
		case "TriggerKeywords":
			instance.Setting.TriggerKeywords.Set(strings.Split(value, ","))
		default:
			isPlatformSpecific := false
			for _, settingDefinition := range instance.Metadata.SettingDefinitions {
				if settingDefinition.Value != nil && settingDefinition.Value.GetKey() == key {
					isPlatformSpecific = settingDefinition.IsPlatformSpecific
					break
				}
			}
			instance.API.SaveSetting(ctx, key, value, isPlatformSpecific)
		}
	}
	return nil
}

func findPluginInstance(pluginID string) (*plugin.Instance, bool) {
	return lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
		return item.Metadata.Id == pluginID
	})
}

// getInstalledPluginDTOs builds the legacy DTO catalog from current plugin instances.
func getInstalledPluginDTOs(ctx context.Context) ([]dto.PluginDto, error) {
	instances := plugin.GetPluginManager().GetPluginInstances()
	plugins := make([]dto.PluginDto, 0, len(instances))
	for _, instance := range instances {
		installedPlugin, err := convertPluginInstanceToDto(ctx, instance)
		if err != nil {
			return nil, err
		}
		plugins = append(plugins, installedPlugin)
	}
	return plugins, nil
}

// getStorePluginDTOs builds the legacy DTO catalog with installation state.
func getStorePluginDTOs(ctx context.Context) ([]dto.PluginDto, error) {
	manifests := plugin.GetStoreManager().GetStorePluginManifests(ctx)
	plugins := make([]dto.PluginDto, len(manifests))
	if err := copier.Copy(&plugins, &manifests); err != nil {
		return nil, err
	}

	for index, storePlugin := range plugins {
		instance, installed := lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
			return item.Metadata.Id == storePlugin.Id
		})
		if manifests[index].IconEmoji != "" {
			plugins[index].Icon = common.NewWoxImageEmoji(manifests[index].IconEmoji)
		} else if manifests[index].IconUrl != "" {
			plugins[index].Icon = common.NewWoxImageUrl(manifests[index].IconUrl)
		}
		plugins[index].IsInstalled = installed
		plugins[index].IsUpgradable = installed && plugin.IsVersionUpgradable(instance.Metadata.Version, manifests[index].Version)
		plugins[index].Name = manifests[index].GetName(ctx)
		plugins[index].NameEn = manifests[index].GetNameEn(ctx)
		plugins[index].Description = manifests[index].GetDescription(ctx)
		plugins[index].DescriptionEn = manifests[index].GetDescriptionEn(ctx)
		plugins[index] = convertPluginDto(ctx, plugins[index], instance)
	}
	return plugins, nil
}

// cloneStringMap isolates mutable plugin settings from core-owned maps.
func cloneStringMap(source map[string]string) map[string]string {
	if source == nil {
		return nil
	}
	result := make(map[string]string, len(source))
	for key, value := range source {
		result[key] = value
	}
	return result
}
