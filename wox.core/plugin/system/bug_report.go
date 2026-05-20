package system

import (
	"context"
	"fmt"
	"wox/common"
	"wox/diagnostic"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
	"wox/util"
	"wox/util/shell"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BugReportPlugin{})
}

type BugReportPlugin struct {
	api plugin.API
}

func (p *BugReportPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "b7f6f0f3-9d18-4f17-b74d-f28d19b1b541",
		Name:            "i18n:plugin_bug_report_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_bug_report_plugin_description",
		Icon:            common.PluginBugReportIcon.String(),
		Entry:           "",
		TriggerKeywords: []string{"bugreport"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
	}
}

func (p *BugReportPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

func (p *BugReportPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	enabled := diagnostic.GetManager().IsEnabled()
	title := "i18n:plugin_bug_report_title_off"
	subtitle := "i18n:plugin_bug_report_subtitle_off"
	if enabled {
		title = "i18n:plugin_bug_report_title_on"
		subtitle = "i18n:plugin_bug_report_subtitle_on"
	}

	result := plugin.QueryResult{
		Title:    title,
		SubTitle: subtitle,
		Icon:     common.PermissionIcon,
		// New feature: enabling bug aware mode has a visible restart impact, so
		// the primary result carries a short localized preview that explains the
		// user-facing purpose without exposing implementation details.
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeMarkdown,
			PreviewData: "i18n:plugin_bug_report_preview",
		},
		Actions: p.buildActions(enabled),
	}
	return plugin.NewQueryResponse([]plugin.QueryResult{result})
}

func (p *BugReportPlugin) buildActions(enabled bool) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{}
	if !enabled {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_bug_report_action_enable_restart",
			Icon:                   common.ExecuteRunIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.enableAndRestart(ctx)
			},
		})
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_bug_report_action_export_now",
			Icon:                   common.PluginInstalledIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.exportDiagnostics(ctx)
			},
		})
	} else {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_bug_report_action_export",
			Icon:                   common.PluginInstalledIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.exportDiagnostics(ctx)
			},
		})
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_bug_report_action_disable",
			Icon:                   common.TrashIcon,
			Hotkey:                 "ctrl+enter",
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				p.disable(ctx)
			},
		})
	}

	actions = append(actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_bug_report_action_open_logs",
		Icon:                   common.OpenContainingFolderIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			_ = shell.OpenFileInFolder(util.GetLocation().GetLogDirectory())
		},
	})
	return actions
}

func (p *BugReportPlugin) enableAndRestart(ctx context.Context) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	previousLogLevel := util.NormalizeLogLevel(woxSetting.LogLevel.Get())
	if _, err := diagnostic.GetManager().Enable(ctx, previousLogLevel); err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_enable_failed"), err.Error()))
		return
	}
	// New feature: bug aware mode trades log volume for crash observability, so
	// it explicitly switches to DEBUG and remembers the user's prior level for
	// disable-time restoration.
	woxSetting.LogLevel.Set(setting.LogLevelDebug)
	util.GetLogger().SetLevel(setting.LogLevelDebug)
	plugin.GetPluginManager().GetUI().UpdateDiagnosticStatus(ctx, true)
	if err := diagnostic.GetManager().StartSupervisorDetached(ctx, true); err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_supervisor_failed"), err.Error()))
		return
	}
	p.api.Notify(ctx, "i18n:plugin_bug_report_notify_enabled")
	ui.GetUIManager().ExitApp(ctx)
}

func (p *BugReportPlugin) disable(ctx context.Context) {
	state, err := diagnostic.GetManager().Disable(ctx)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_disable_failed"), err.Error()))
		return
	}
	if state.PreviousLogLevel != "" {
		setting.GetSettingManager().GetWoxSetting(ctx).LogLevel.Set(state.PreviousLogLevel)
		util.GetLogger().SetLevel(state.PreviousLogLevel)
	}
	plugin.GetPluginManager().GetUI().UpdateDiagnosticStatus(ctx, false)
	p.api.Notify(ctx, "i18n:plugin_bug_report_notify_disabled")
}

func (p *BugReportPlugin) exportDiagnostics(ctx context.Context) {
	exportPath, err := diagnostic.GetManager().Export(ctx)
	if err != nil {
		p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_export_failed"), err.Error()))
		return
	}
	_ = shell.OpenFileInFolder(exportPath)
	p.api.Notify(ctx, fmt.Sprintf(p.api.GetTranslation(ctx, "plugin_bug_report_notify_exported"), exportPath))
}
