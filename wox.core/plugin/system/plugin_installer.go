package system

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/util/selection"

	"github.com/Masterminds/semver/v3"
	"github.com/samber/lo"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &PluginInstallerPlugin{})
}

type PluginInstallerPlugin struct {
	api plugin.API
}

func (i *PluginInstallerPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "1aee0f80-2bcd-489a-a2c6-81f9f2e54cad",
		Name:          "Wox Plugin Installer",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Install Wox plugins",
		Icon:          plugin.WoxIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQuerySelection,
			},
		},
	}
}

func (i *PluginInstallerPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *PluginInstallerPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Type == plugin.QueryTypeSelection &&
		query.Selection.Type == selection.SelectionTypeFile &&
		len(query.Selection.FilePaths) == 1 &&
		strings.HasSuffix(query.Selection.FilePaths[0], ".wox") {
		return i.queryForSelectionFile(ctx, query.Selection.FilePaths[0])
	}

	return []plugin.QueryResult{}
}

func (i *PluginInstallerPlugin) queryForSelectionFile(ctx context.Context, filePath string) []plugin.QueryResult {
	var results []plugin.QueryResult

	pluginMetadata, err := plugin.GetStoreManager().ParsePluginManifestFromLocal(ctx, filePath)
	if err != nil {
		i.api.Notify(ctx, fmt.Sprintf("Failed to parse plugin manifest: %s", err.Error()))
		return results
	}

	// Check if plugin is already installed
	installedPlugin, isInstalled := lo.Find(plugin.GetPluginManager().GetPluginInstances(), func(item *plugin.Instance) bool {
		return item.Metadata.Id == pluginMetadata.Id
	})

	// Determine action title and button text based on installation status and version comparison
	actionTitleKey := "plugin_installer_install"
	actionButtonName := "Install"

	if isInstalled {
		installedVersion, installedErr := semver.NewVersion(installedPlugin.Metadata.Version)
		currentVersion, currentErr := semver.NewVersion(pluginMetadata.Version)

		if installedErr == nil && currentErr == nil {
			if installedVersion.GreaterThan(currentVersion) {
				actionTitleKey = "plugin_installer_downgrade"
				actionButtonName = "Downgrade"
			} else if installedVersion.LessThan(currentVersion) {
				actionTitleKey = "plugin_installer_upgrade"
				actionButtonName = "Upgrade"
			} else {
				actionTitleKey = "plugin_installer_reinstall"
				actionButtonName = "Reinstall"
			}
		}
	}

	// Get translated action title
	actionTitle := i.api.GetTranslation(ctx, actionTitleKey)

	// create result for plugin installation
	results = append(results, plugin.QueryResult{
		Title:    fmt.Sprintf("%s: %s", actionTitle, pluginMetadata.Name),
		SubTitle: fmt.Sprintf("Version: %s, Author: %s\nDescription: %s", pluginMetadata.Version, pluginMetadata.Author, pluginMetadata.Description),
		Icon:     plugin.WoxIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   actionButtonName,
				Icon:                   plugin.WoxIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					i.api.Notify(ctx, fmt.Sprintf(i.api.GetTranslation(ctx, "plugin_installer_action_start"), actionButtonName, pluginMetadata.Name))
					installErr := plugin.GetStoreManager().InstallFromLocal(ctx, filePath)
					if installErr != nil {
						i.api.Notify(ctx, fmt.Sprintf(i.api.GetTranslation(ctx, "plugin_installer_action_failed"), strings.ToLower(actionButtonName), installErr.Error()))
					} else {
						actionVerbKey := fmt.Sprintf("plugin_installer_verb_%s_past", strings.ToLower(actionButtonName))
						actionVerb := i.api.GetTranslation(ctx, actionVerbKey)
						i.api.Notify(ctx, fmt.Sprintf(i.api.GetTranslation(ctx, "plugin_installer_action_success"), actionVerb, pluginMetadata.Name))
					}
				},
			},
		},
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: fmt.Sprintf(`# %s

## Basic Information
- **Version**: %s
- **Author**: %s
- **Website**: [%s](%s)

## Description
%s

## Technical Details
- **Runtime**: %s
- **Min Wox Version**: %s
- **Supported OS**: %s
- **Plugin ID**: %s

## Features
%s`,
				pluginMetadata.Name,
				func() string {
					versionText := pluginMetadata.Version
					if isInstalled {
						versionText += fmt.Sprintf(" (Installed: %s)", installedPlugin.Metadata.Version)
					}
					return versionText
				}(),
				pluginMetadata.Author,
				pluginMetadata.Website,
				pluginMetadata.Website,
				pluginMetadata.Description,
				pluginMetadata.Runtime,
				pluginMetadata.MinWoxVersion,
				strings.Join(pluginMetadata.SupportedOS, ", "),
				pluginMetadata.Id,
				func() string {
					if len(pluginMetadata.Features) == 0 {
						return "No special features"
					}
					var features []string
					for _, f := range pluginMetadata.Features {
						features = append(features, fmt.Sprintf("- %s", f.Name))
					}
					return strings.Join(features, "\n")
				}(),
			),
		},
		Score: 2000,
	})

	return results
}
