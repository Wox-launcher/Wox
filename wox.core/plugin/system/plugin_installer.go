package system

import (
	"context"
	"fmt"
	"strings"
	"wox/plugin"
	"wox/util"
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
		query.Selection.Type == util.SelectionTypeFile &&
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

	// create result for plugin installation
	results = append(results, plugin.QueryResult{
		Title:    fmt.Sprintf("Install plugin: %s", pluginMetadata.Name),
		SubTitle: fmt.Sprintf("Version: %s, Author: %s\nDescription: %s", pluginMetadata.Version, pluginMetadata.Author, pluginMetadata.Description),
		Icon:     plugin.WoxIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "Install",
				Icon:                   plugin.WoxIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					i.api.Notify(ctx, fmt.Sprintf("Installing plugin: %s", pluginMetadata.Name))
					installErr := plugin.GetStoreManager().InstallFromLocal(ctx, filePath)
					if installErr != nil {
						i.api.Notify(ctx, fmt.Sprintf("Failed to install plugin: %s", installErr.Error()))
					} else {
						i.api.Notify(ctx, fmt.Sprintf("Plugin installed: %s", pluginMetadata.Name))
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
				pluginMetadata.Version,
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
