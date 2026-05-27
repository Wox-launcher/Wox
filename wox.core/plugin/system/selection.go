package system

import (
	"context"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system/explorer"
	"wox/setting/definition"
	"wox/util"
	"wox/util/airdrop"
	"wox/util/clipboard"
	"wox/util/keyboard"
	"wox/util/selection"
	"wox/util/shell"

	"github.com/google/uuid"
)

var selectionIcon = common.PluginSelectionIcon

// selectionCommandPreview is the command name that, when used in a selection file
// query, causes the plugin to return only the file preview result instead of
// the full set of actions (copy path, open folder, preview, etc.).
const selectionCommandPreview = "preview"

const (
	enableSpaceQuickLookSettingKey = "enableSpaceQuickLook"
	// The trailing space makes Wox parse "preview" as a command instead of a
	// search term for selection queries.
	selectionQuickLookQueryText = "selection " + selectionCommandPreview + " "
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &SelectionPlugin{})
}

type SelectionPlugin struct {
	api            plugin.API
	quickLookMu    sync.Mutex
	quickLookState *selectionSpaceQuickLookState
}

// selectionSpaceQuickLookState owns the platform monitor subscriptions and the
// small key-state machine needed to avoid repeated or accidental Space previews.
type selectionSpaceQuickLookState struct {
	plugin        *SelectionPlugin
	explorerSub   explorer.ExplorerRawKeySubscription
	dialogSub     explorer.ExplorerRawKeySubscription
	mu            sync.Mutex
	spaceDown     bool
	spaceConsumed bool
	invalidUntil  time.Time
}

func (i *SelectionPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "d9e557ed-89bd-4b8b-bd64-2a7632cf3483",
		Name:          "i18n:plugin_selection_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_selection_plugin_description",
		Icon:          selectionIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
			"selection",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Commands: []plugin.MetadataCommand{
			{
				Command:     selectionCommandPreview,
				Description: "i18n:plugin_selection_command_preview",
			},
		},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          enableSpaceQuickLookSettingKey,
					Label:        "i18n:plugin_selection_setting_enable_space_quick_look",
					Tooltip:      "i18n:plugin_selection_setting_enable_space_quick_look_tips",
					DefaultValue: "false",
				},
				DisabledInPlatforms: []util.Platform{util.PlatformMacOS, util.PlatformLinux},
				IsPlatformSpecific:  true,
			},
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureQuerySelection,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]any{
					// The preview command is intended to behave like Quick Look. A plugin-wide
					// WidthRatio 0 would also hide the result list for normal selection queries,
					// so the command-scoped ratio keeps only "selection preview" preview-only.
					"WidthRatio": 0.0,
					"Commands":   []string{selectionCommandPreview},
				},
			},
		},
	}
}

func (i *SelectionPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	i.api = initParams.API

	i.updateSpaceQuickLookListener(ctx, i.api.GetSetting(ctx, enableSpaceQuickLookSettingKey) == "true")
	i.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == enableSpaceQuickLookSettingKey {
			i.updateSpaceQuickLookListener(callbackCtx, value == "true")
		}
	})
	i.api.OnUnload(ctx, func(ctx context.Context) {
		i.stopSpaceQuickLookListener()
	})
}

// updateSpaceQuickLookListener keeps the platform monitor subscriptions aligned
// with the platform-specific plugin setting.
func (i *SelectionPlugin) updateSpaceQuickLookListener(ctx context.Context, enabled bool) {
	if !enabled || !util.IsWindows() {
		i.stopSpaceQuickLookListener()
		return
	}

	i.quickLookMu.Lock()
	if i.quickLookState != nil {
		i.quickLookMu.Unlock()
		return
	}

	state := &selectionSpaceQuickLookState{plugin: i}
	explorerSub, explorerErr := explorer.AddExplorerRawKeyListener(state.handleRawKey)
	if explorerErr != nil {
		i.quickLookMu.Unlock()
		i.api.Log(ctx, plugin.LogLevelWarning, "Failed to enable Space Quick Look explorer listener: "+explorerErr.Error())
		return
	}

	dialogSub, dialogErr := explorer.AddExplorerOpenSaveRawKeyListener(state.handleRawKey)
	if dialogErr != nil {
		if explorerSub != nil {
			_ = explorerSub.Close()
		}
		i.quickLookMu.Unlock()
		i.api.Log(ctx, plugin.LogLevelWarning, "Failed to enable Space Quick Look dialog listener: "+dialogErr.Error())
		return
	}

	state.explorerSub = explorerSub
	state.dialogSub = dialogSub
	i.quickLookState = state
	i.quickLookMu.Unlock()
}

// stopSpaceQuickLookListener removes any active Space Quick Look monitor
// subscriptions owned by the Selection plugin.
func (i *SelectionPlugin) stopSpaceQuickLookListener() {
	i.quickLookMu.Lock()
	state := i.quickLookState
	i.quickLookState = nil
	i.quickLookMu.Unlock()

	if state != nil {
		state.close()
	}
}

func (s *selectionSpaceQuickLookState) close() {
	if s.explorerSub != nil {
		_ = s.explorerSub.Close()
	}
	if s.dialogSub != nil {
		_ = s.dialogSub.Close()
	}
}

// handleRawKey applies QuickLook-style Space filtering before triggering the
// existing Selection preview command.
func (s *selectionSpaceQuickLookState) handleRawKey(event keyboard.RawKeyEvent) bool {
	if event.Type == keyboard.EventTypeKeyUp {
		if event.Key == keyboard.KeySpace {
			s.mu.Lock()
			s.spaceDown = false
			s.spaceConsumed = false
			s.mu.Unlock()
		}
		return false
	}

	if event.Type != keyboard.EventTypeKeyDown {
		return false
	}

	if event.Key != keyboard.KeySpace {
		s.recordInvalidKeyIfNeeded(event)
		return false
	}

	s.mu.Lock()
	if s.spaceDown {
		consume := s.spaceConsumed
		s.mu.Unlock()
		return consume
	}

	s.spaceDown = true
	if event.Modifiers != 0 {
		s.spaceConsumed = false
		s.mu.Unlock()
		return false
	}
	if time.Now().Before(s.invalidUntil) {
		s.spaceConsumed = false
		s.mu.Unlock()
		return false
	}
	if s.plugin.api.IsVisible(context.Background()) {
		s.spaceConsumed = false
		s.mu.Unlock()
		return false
	}
	s.spaceConsumed = true
	s.mu.Unlock()

	util.Go(context.Background(), "selection space quick look", func() {
		s.plugin.triggerSpaceQuickLook()
	})
	return true
}

// recordInvalidKeyIfNeeded suppresses Space briefly after normal typing or
// command keys so typing in Explorer cannot accidentally open preview.
func (s *selectionSpaceQuickLookState) recordInvalidKeyIfNeeded(event keyboard.RawKeyEvent) {
	if isSpaceQuickLookNavigationKey(event.Key) || isSpaceQuickLookModifierKey(event.Key) {
		return
	}

	s.mu.Lock()
	s.invalidUntil = time.Now().Add(time.Second)
	s.mu.Unlock()
}

func isSpaceQuickLookNavigationKey(key keyboard.Key) bool {
	switch key {
	case keyboard.KeyLeft, keyboard.KeyRight, keyboard.KeyUp, keyboard.KeyDown,
		keyboard.KeyReturn, keyboard.KeyEscape, keyboard.KeyF5, keyboard.KeyF11:
		return true
	default:
		return false
	}
}

func isSpaceQuickLookModifierKey(key keyboard.Key) bool {
	switch key {
	case keyboard.KeyCtrl, keyboard.KeyShift, keyboard.KeyAlt, keyboard.KeySuper:
		return true
	default:
		return false
	}
}

// triggerSpaceQuickLook opens the existing preview-only Selection query for a
// single selected file.
func (i *SelectionPlugin) triggerSpaceQuickLook() {
	ctx := util.NewTraceContext()
	ctx = util.WithCoreSessionContext(ctx)
	ctx = util.WithShowSourceContext(ctx, string(common.ShowSourceSelection))

	selected, err := selection.GetSelected(ctx)
	if err != nil {
		return
	}
	if selected.Type != selection.SelectionTypeFile || len(selected.FilePaths) != 1 {
		return
	}
	if !util.IsFileExists(selected.FilePaths[0]) {
		return
	}

	plugin.GetPluginManager().GetUI().ChangeQuery(ctx, common.PlainQuery{
		QueryId:        uuid.NewString(),
		QueryType:      plugin.QueryTypeSelection,
		QueryText:      selectionQuickLookQueryText,
		QuerySelection: selected,
	})
	plugin.GetPluginManager().GetUI().ShowApp(ctx, common.ShowContext{
		HideQueryBox: true,
		HideToolbar:  true,
		ShowSource:   common.ShowSourceSelection,
	})
}

func (i *SelectionPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Type != plugin.QueryTypeSelection {
		return plugin.QueryResponse{}
	}

	if query.Selection.Type == selection.SelectionTypeText {
		return plugin.NewQueryResponse(i.queryForSelectionText(ctx, query.Selection.Text))
	}
	if query.Selection.Type == selection.SelectionTypeFile {
		return plugin.NewQueryResponse(i.queryForSelectionFile(ctx, query, query.Selection.FilePaths))
	}

	return plugin.QueryResponse{}
}

func (i *SelectionPlugin) queryForSelectionText(ctx context.Context, text string) []plugin.QueryResult {
	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_copy"),
		Icon:  common.CopyIcon,
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

func (i *SelectionPlugin) queryForSelectionFile(ctx context.Context, query plugin.Query, filePaths []string) []plugin.QueryResult {
	// When the preview command is specified, skip all other actions and only
	// return the preview result for a single selected file. This allows users
	// to quickly open a file preview without seeing copy/open-folder options.
	if query.Command == selectionCommandPreview {
		if len(filePaths) == 1 {
			return i.queryForFilePreviewOnly(ctx, filePaths[0])
		}
		return []plugin.QueryResult{}
	}

	var results []plugin.QueryResult
	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_copy_path"),
		Icon:  common.CopyIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_copy"),
				Icon: common.CopyIcon,
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
			Icon:  common.AirdropIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: i.api.GetTranslation(ctx, "selection_share"),
					Icon: common.AirdropIcon,
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
		Icon:  common.OpenContainingFolderIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_open_containing_folder"),
				Icon: common.OpenContainingFolderIcon,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.OpenFileInFolder(filePath)
				},
			},
		},
	})

	results = append(results, plugin.QueryResult{
		Title: i.api.GetTranslation(ctx, "selection_preview"),
		Score: 1000,
		Icon:  common.PreviewIcon,
		Actions: []plugin.QueryResultAction{
			{
				Name: i.api.GetTranslation(ctx, "selection_preview"),
				Icon: common.PreviewIcon,
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

// queryForFilePreviewOnly returns only the preview result for a single file,
// used when the preview command is active to skip copy/open-folder actions.
func (i *SelectionPlugin) queryForFilePreviewOnly(ctx context.Context, filePath string) []plugin.QueryResult {
	if !util.IsFileExists(filePath) {
		return []plugin.QueryResult{}
	}

	return []plugin.QueryResult{
		{
			Title: i.api.GetTranslation(ctx, "selection_preview"),
			Score: 1000,
			Icon:  common.PreviewIcon,
			Actions: []plugin.QueryResultAction{
				{
					Name: i.api.GetTranslation(ctx, "selection_preview"),
					Icon: common.PreviewIcon,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					},
				},
			},
			Preview: plugin.WoxPreview{
				PreviewType: plugin.WoxPreviewTypeFile,
				PreviewData: filePath,
			},
		},
	}
}
