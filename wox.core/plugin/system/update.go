package system

import (
	"context"
	"encoding/json"
	"fmt"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/updater"
	"wox/util/shell"
)

var updateIcon = common.UpdateIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &UpdatePlugin{})
}

type UpdatePlugin struct {
	api plugin.API
}

type updatePreviewData struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion"`
	ReleaseNotes      string `json:"releaseNotes"`
	DownloadUrl       string `json:"downloadUrl"`
	Status            string `json:"status"`
	HasUpdate         bool   `json:"hasUpdate"`
	Error             string `json:"error"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
}

func (p *UpdatePlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "2a9e9f06-6ff1-49c9-9f75-9db7f7a0b7b7",
		Name:            "i18n:plugin_update_plugin_name",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "i18n:plugin_update_plugin_description",
		Icon:            updateIcon.String(),
		TriggerKeywords: []string{"update", "upgrade"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
			{
				Name: plugin.MetadataFeatureResultPreviewWidthRatio,
				Params: map[string]any{
					"WidthRatio": 0.0,
				},
			},
		},
	}
}

func (p *UpdatePlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

func (p *UpdatePlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	info := updater.GetUpdateInfo()
	autoUpdateEnabled := true
	if woxSetting := setting.GetSettingManager().GetWoxSetting(ctx); woxSetting != nil {
		autoUpdateEnabled = woxSetting.EnableAutoUpdate.Get()
	}

	preview := plugin.WoxPreview{
		PreviewType:       plugin.WoxPreviewTypeUpdate,
		PreviewData:       p.buildPreviewData(info, autoUpdateEnabled),
		PreviewProperties: map[string]string{},
		ScrollPosition:    "",
	}

	result := plugin.QueryResult{
		Title:   "", // we don't need title in update plugin
		Icon:    updateIcon,
		Preview: preview,
		Actions: p.buildActions(ctx, info, autoUpdateEnabled),
	}

	return []plugin.QueryResult{result}
}

func (p *UpdatePlugin) buildPreviewData(info updater.UpdateInfo, autoUpdateEnabled bool) string {
	errText := ""
	if info.UpdateError != nil {
		errText = info.UpdateError.Error()
	}

	data := updatePreviewData{
		CurrentVersion:    info.CurrentVersion,
		LatestVersion:     info.LatestVersion,
		ReleaseNotes:      info.ReleaseNotes,
		DownloadUrl:       info.DownloadUrl,
		Status:            string(info.Status),
		HasUpdate:         info.HasUpdate,
		Error:             errText,
		AutoUpdateEnabled: autoUpdateEnabled,
	}

	b, err := json.Marshal(data)
	if err != nil {
		return "{}"
	}
	return string(b)
}

func (p *UpdatePlugin) buildActions(ctx context.Context, info updater.UpdateInfo, autoUpdateEnabled bool) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{}

	if !autoUpdateEnabled {
		actions = append(actions,
			plugin.QueryResultAction{
				Name:                   "i18n:plugin_update_action_enable_auto_update",
				IsDefault:              true,
				Icon:                   common.CorrectIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if woxSetting := setting.GetSettingManager().GetWoxSetting(ctx); woxSetting != nil {
						_ = woxSetting.EnableAutoUpdate.Set(true)
					}
					plugin.GetPluginManager().GetUI().ReloadSetting(ctx)
					p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_checking"))
					updater.CheckForUpdatesWithCallback(ctx, func(info updater.UpdateInfo) {
						p.notifyUpdate(ctx, info)
					})
				},
			},
			plugin.QueryResultAction{
				Icon:                   common.SettingIcon,
				Name:                   "i18n:plugin_update_action_open_settings",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{Path: "/general"})
				},
			},
		)
		if info.DownloadUrl != "" {
			actions = append(actions, plugin.QueryResultAction{
				Name:                   "i18n:plugin_update_action_manual_download",
				Icon:                   updateIcon,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					latest := updater.GetUpdateInfo()
					if latest.DownloadUrl != "" {
						_ = shell.Open(latest.DownloadUrl)
					}
				},
			})
		}
		return actions
	}

	checkAction := plugin.QueryResultAction{
		Name:                   "i18n:plugin_update_action_check",
		Icon:                   common.UpdateIcon,
		IsDefault:              info.Status != updater.UpdateStatusReady,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_checking"))
			updater.CheckForUpdatesWithCallback(ctx, func(info updater.UpdateInfo) {
				p.notifyUpdate(ctx, info)
			})
		},
	}
	actions = append(actions, checkAction)

	if info.DownloadUrl != "" {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_update_action_manual_download",
			Icon:                   updateIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				latest := updater.GetUpdateInfo()
				if latest.DownloadUrl != "" {
					_ = shell.Open(latest.DownloadUrl)
				}
			},
		})
	}

	if info.Status == updater.UpdateStatusReady && info.DownloadedPath != "" {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_update_action_apply",
			Icon:                   common.InstallIcon,
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := updater.ApplyUpdate(ctx); err != nil {
					plugin.GetPluginManager().GetUI().Notify(ctx, common.NotifyMsg{
						Text:           fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_apply_failed"), err.Error()),
						Icon:           updateIcon.String(),
						DisplaySeconds: 5,
					})
				}
			},
		})
	}

	return actions
}

func (p *UpdatePlugin) notifyUpdate(ctx context.Context, info updater.UpdateInfo) {
	if info.UpdateError != nil || info.Status == updater.UpdateStatusError {
		errText := ""
		if info.UpdateError != nil {
			errText = info.UpdateError.Error()
		}
		p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_error"), errText))
		return
	}

	if info.Status == updater.UpdateStatusDownloading {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_downloading"))
		return
	}

	if info.Status == updater.UpdateStatusReady {
		p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_ready"))
		return
	}

	if info.HasUpdate {
		p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_doctor_version_update_available"), info.CurrentVersion, info.LatestVersion))
		return
	}

	p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_up_to_date"))
}
