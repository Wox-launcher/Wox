package system

import (
	"context"
	"strings"
	"wox/plugin"
	"wox/util"
	"wox/util/airdrop"
	"wox/util/clipboard"
	"wox/util/selection"
	"wox/util/shell"
)

var selectionIcon = plugin.PluginSelectionIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SelectionPlugin{})
}

type SelectionPlugin struct {
	api plugin.API
}

func (i *SelectionPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "d9e557ed-89bd-4b8b-bd64-2a7632cf3483",
		Name:          "Selection",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Wox default actions for selection query",
		Icon:          selectionIcon.String(),
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

func (i *SelectionPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API
}

func (i *SelectionPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	if query.Type != plugin.QueryTypeSelection {
		return []plugin.QueryResult{}
	}

	if query.Selection.Type == selection.SelectionTypeText {
		return i.queryForSelectionText(ctx, query.Selection.Text)
	}
	if query.Selection.Type == selection.SelectionTypeFile {
		return i.queryForSelectionFile(ctx, query.Selection.FilePaths)
	}

	return []plugin.QueryResult{}
}

func (i *SelectionPlugin) queryForSelectionText(ctx context.Context, text string) []plugin.QueryResult {
	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_copy"),
		Icon:  plugin.CopyIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_copy_to_clipboard"),
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(text)
				},
			},
		},
	})

	if util.IsFileExists(strings.TrimSpace(text)) {
		results = append(results, i.queryForFile(ctx, strings.TrimSpace(text))...)
	}

	return results
}

func (i *SelectionPlugin) queryForSelectionFile(ctx context.Context, filePaths []string) []plugin.QueryResult {
	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_copy_path"),
		Icon:  plugin.CopyIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_copy"),
				Icon: plugin.CopyIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(strings.Join(filePaths, "\n"))
				},
			},
		},
	})
	if len(filePaths) == 1 {
		results = append(results, i.queryForFile(ctx, filePaths[0])...)
	}

	if util.IsMacOS() {
		// share with airdrop
		results = append(results, plugin.QueryResult{
			Title: i.api.GetTranslation(ctx, "selection_share_with_airdrop"),
			Icon:  plugin.AirdropIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: i.api.GetTranslation(ctx, "selection_share"),
					Icon: plugin.AirdropIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						airdrop.Airdrop(filePaths)
					},
				},
			},
		})
	}

	return results
}

func (i *SelectionPlugin) queryForFile(ctx context.Context, filePath string) (results []plugin.QueryResult) {
	if !util.IsFileExists(filePath) {
		return
	}

	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_open_containing_folder"),
		Icon:  plugin.OpenContainingFolderIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_open_containing_folder"),
				Icon: plugin.OpenContainingFolderIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.OpenFileInFolder(filePath)
				},
			},
		},
	})

	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_preview"),
		Score: 1000,
		Icon:  plugin.PreviewIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_preview"),
				Icon: plugin.PreviewIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				},
			},
		},
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeFile,
			PreviewData: filePath,
			PreviewProperties: map[string]string{
				i.api.GetTranslation(ctx, "selection_created_at"):  util.GetFileCreatedAt(filePath),
				i.api.GetTranslation(ctx, "selection_modified_at"): util.GetFileModifiedAt(filePath),
				i.api.GetTranslation(ctx, "selection_size"):        util.GetFileSize(filePath),
			},
		},
	})

	return
}
