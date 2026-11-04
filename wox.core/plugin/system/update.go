package system

import (
	"context"
	"encoding/json"
	"fmt"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/updater"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/shell"
)

var updateIcon = icons.Get(icons.ActionUpdate)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &UpdatePlugin{})
}

type UpdatePlugin struct {
	api plugin.API
}

type updatePreviewData struct {
	CurrentVersion    string `json:"currentVersion"`
	LatestVersion     string `json:"latestVersion"`
	ReleaseChannel    string `json:"releaseChannel"`
	ReleaseNotes      string `json:"releaseNotes"`
	DownloadUrl       string `json:"downloadUrl"`
	Status            string `json:"status"`
	HasUpdate         bool   `json:"hasUpdate"`
	Error             string `json:"error"`
	AutoUpdateEnabled bool   `json:"autoUpdateEnabled"`
}

func (p *UpdatePlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              common.UpdatePluginID,
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

func (p *UpdatePlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	info := updater.GetUpdateInfo()
	autoUpdateEnabled := true
	releaseChannel := string(setting.ReleaseChannelStable)
	if woxSetting := setting.GetSettingManager().GetWoxSetting(ctx); woxSetting != nil {
		autoUpdateEnabled = woxSetting.EnableAutoUpdate.Get()
		releaseChannel = string(setting.NormalizeReleaseChannel(string(woxSetting.ReleaseChannel.Get())))
	}
	channelVersions := p.getActionChannelVersions(ctx)

	preview := plugin.WoxPreview{
		PreviewType:    plugin.WoxPreviewTypeUpdate,
		PreviewData:    p.buildPreviewData(info, autoUpdateEnabled, releaseChannel),
		ScrollPosition: "",
	}

	// The update plugin renders one preview row. Keeping it as a local result
	// avoids the stale slice variable left by the QueryResponse migration while
	// still returning through NewQueryResponse for the shared plugin contract.
	result := plugin.QueryResult{
		Title:   "", // we don't need title in update plugin
		Icon:    updateIcon,
		Preview: preview,
		Actions: p.buildActions(ctx, info, autoUpdateEnabled, releaseChannel, channelVersions),
	}

	return plugin.NewQueryResponse([]plugin.QueryResult{result})
}

func (p *UpdatePlugin) buildPreviewData(info updater.UpdateInfo, autoUpdateEnabled bool, releaseChannel string) string {
	errText := ""
	if info.UpdateError != nil {
		errText = info.UpdateError.Error()
	}

	data := updatePreviewData{
		CurrentVersion:    info.CurrentVersion,
		LatestVersion:     info.LatestVersion,
		ReleaseChannel:    releaseChannel,
		ReleaseNotes:      info.ReleaseNotesForLang(string(i18n.GetI18nManager().GetCurrentLangCode())),
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

// getActionChannelVersions keeps action labels responsive when remote manifests are slow or unavailable.
func (p *UpdatePlugin) getActionChannelVersions(ctx context.Context) []updater.UpdateChannelVersion {
	channelVersionCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return updater.GetUpdateChannelVersions(channelVersionCtx)
}

func (p *UpdatePlugin) buildActions(ctx context.Context, info updater.UpdateInfo, autoUpdateEnabled bool, releaseChannel string, channelVersions []updater.UpdateChannelVersion) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{}

	if !autoUpdateEnabled {
		actions = append(actions,
			plugin.QueryResultAction{
				Name:                   "i18n:plugin_update_action_enable_auto_update",
				IsDefault:              true,
				Icon:                   icons.Get(icons.ActionCorrect),
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if woxSetting := setting.GetSettingManager().GetWoxSetting(ctx); woxSetting != nil {
						_ = woxSetting.EnableAutoUpdate.Set(true)
					}
					plugin.GetPluginManager().GetUI().ReloadSetting(ctx)
					p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_checking"), true, "")
					updater.CheckForUpdatesWithCallback(ctx, func(info updater.UpdateInfo) {
						p.refreshVisibleUpdatePreview(ctx, actionContext, info, true, releaseChannel, channelVersions)
						p.notifyUpdate(ctx, info)
					})
				},
			},
			p.buildSwitchReleaseChannelAction(ctx, autoUpdateEnabled, releaseChannel, channelVersions),
			plugin.QueryResultAction{
				Icon:                   icons.Get(icons.ActionSettings),
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
				Icon:                   icons.Get(icons.ActionInstall),
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
		Icon:                   icons.Get(icons.ActionUpdate),
		IsDefault:              info.Status != updater.UpdateStatusReady,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_checking"), true, "")
			updater.CheckForUpdatesWithCallback(ctx, func(info updater.UpdateInfo) {
				p.refreshVisibleUpdatePreview(ctx, actionContext, info, autoUpdateEnabled, releaseChannel, channelVersions)
				p.notifyUpdate(ctx, info)
			})
		},
	}
	actions = append(actions, checkAction)
	actions = append(actions, p.buildSwitchReleaseChannelAction(ctx, autoUpdateEnabled, releaseChannel, channelVersions))

	if info.DownloadUrl != "" {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_update_action_manual_download",
			Icon:                   icons.Get(icons.ActionInstall),
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
			Icon:                   icons.Get(icons.ActionInstall),
			IsDefault:              true,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if err := updater.ApplyUpdate(ctx, func(stage updater.ApplyUpdateStage) {
					p.notifyApplyProgress(ctx, stage)
				}); err != nil {
					p.showUpdateStatus(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_apply_failed"), err.Error()), false, err.Error())
				}
			},
		})
	}

	return actions
}

// buildSwitchReleaseChannelAction keeps the update result as the one-click place for changing update channels.
func (p *UpdatePlugin) buildSwitchReleaseChannelAction(ctx context.Context, autoUpdateEnabled bool, releaseChannel string, channelVersions []updater.UpdateChannelVersion) plugin.QueryResultAction {
	currentChannel := setting.NormalizeReleaseChannel(releaseChannel)
	targetChannel := setting.ReleaseChannelStable
	if currentChannel == setting.ReleaseChannelStable {
		targetChannel = setting.ReleaseChannelBeta
	}

	return plugin.QueryResultAction{
		Id:                     fmt.Sprintf("switch_to_%s_channel", targetChannel),
		Name:                   p.switchReleaseChannelActionName(ctx, targetChannel, updateChannelLatestVersion(channelVersions, targetChannel)),
		Icon:                   icons.Get(icons.ActionStar),
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
			if woxSetting == nil {
				return
			}

			if err := woxSetting.ReleaseChannel.Set(targetChannel); err != nil {
				p.showUpdateStatus(ctx, err.Error(), false, err.Error())
				return
			}

			updater.ResetUpdateInfoForReleaseChannel(targetChannel)
			plugin.GetPluginManager().GetUI().ReloadSetting(ctx)
			p.refreshVisibleUpdatePreview(ctx, actionContext, updater.GetUpdateInfo(), autoUpdateEnabled, string(targetChannel), channelVersions)
			if !autoUpdateEnabled {
				return
			}

			p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_checking"), true, "")
			updater.CheckForUpdatesWithCallback(ctx, func(info updater.UpdateInfo) {
				p.refreshVisibleUpdatePreview(ctx, actionContext, info, autoUpdateEnabled, string(targetChannel), channelVersions)
				p.notifyUpdate(ctx, info)
			})
		},
	}
}

// switchReleaseChannelActionName formats the target channel action with a manifest version when it is available.
func (p *UpdatePlugin) switchReleaseChannelActionName(ctx context.Context, targetChannel setting.ReleaseChannel, latestVersion string) string {
	key := "plugin_update_action_switch_to_stable_channel"
	versionKey := "plugin_update_action_switch_to_stable_channel_with_version"
	if targetChannel == setting.ReleaseChannelBeta {
		key = "plugin_update_action_switch_to_beta_channel"
		versionKey = "plugin_update_action_switch_to_beta_channel_with_version"
	}

	if latestVersion == "" {
		return i18n.GetI18nManager().TranslateWox(ctx, key)
	}
	return fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, versionKey), latestVersion)
}

// updateChannelLatestVersion finds the latest version already loaded for a release channel.
func updateChannelLatestVersion(channelVersions []updater.UpdateChannelVersion, releaseChannel setting.ReleaseChannel) string {
	targetChannel := setting.NormalizeReleaseChannel(string(releaseChannel))
	for _, channelVersion := range channelVersions {
		if setting.NormalizeReleaseChannel(channelVersion.Channel) == targetChannel {
			return channelVersion.LatestVersion
		}
	}
	return ""
}

// refreshVisibleUpdatePreview keeps the current update preview in sync while a manual check/download action runs.
func (p *UpdatePlugin) refreshVisibleUpdatePreview(ctx context.Context, actionContext plugin.ActionContext, info updater.UpdateInfo, autoUpdateEnabled bool, releaseChannel string, channelVersions []updater.UpdateChannelVersion) {
	if actionContext.ResultId == "" {
		return
	}

	updatable := p.api.GetUpdatableResult(ctx, actionContext.ResultId)
	if updatable == nil {
		return
	}

	effectiveReleaseChannel := releaseChannel
	if info.ReleaseChannel != "" {
		effectiveReleaseChannel = info.ReleaseChannel
	}
	preview := plugin.WoxPreview{
		PreviewType:    plugin.WoxPreviewTypeUpdate,
		PreviewData:    p.buildPreviewData(info, autoUpdateEnabled, effectiveReleaseChannel),
		ScrollPosition: "",
	}
	actions := p.buildActions(ctx, info, autoUpdateEnabled, effectiveReleaseChannel, channelVersions)
	updatable.Preview = &preview
	updatable.Actions = &actions
	p.api.UpdateResult(ctx, *updatable)
}

func (p *UpdatePlugin) notifyApplyProgress(ctx context.Context, stage updater.ApplyUpdateStage) {
	switch stage {
	case updater.ApplyUpdateStagePreparing:
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_apply_preparing"), true, "")
	case updater.ApplyUpdateStageExtracting:
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_apply_extracting"), true, "")
	case updater.ApplyUpdateStageReplacing:
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_apply_replacing"), true, "")
	case updater.ApplyUpdateStageRestarting:
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_apply_restarting"), true, "")
	}
}

func (p *UpdatePlugin) notifyUpdate(ctx context.Context, info updater.UpdateInfo) {
	if info.UpdateError != nil || info.Status == updater.UpdateStatusError {
		errText := ""
		if info.UpdateError != nil {
			errText = info.UpdateError.Error()
		}
		p.showUpdateStatus(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_error"), errText), false, errText)
		return
	}

	if info.Status == updater.UpdateStatusDownloading {
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_downloading"), true, "")
		return
	}

	if info.Status == updater.UpdateStatusReady {
		p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_ready"), false, "")
		return
	}

	if info.HasUpdate {
		p.showUpdateStatus(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_available"), info.CurrentVersion, info.LatestVersion), false, "")
		return
	}

	p.showUpdateStatus(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_update_notify_up_to_date"), false, "")
}

// showUpdateStatus keeps progress and its final outcome in the update query context.
func (p *UpdatePlugin) showUpdateStatus(ctx context.Context, message string, inProgress bool, errorText string) {
	status := plugin.ToolbarMsg{Id: "wox-update-status", Title: message, Indeterminate: inProgress, IsPersist: inProgress || errorText != ""}
	if errorText != "" {
		status.Actions = []plugin.ToolbarMsgAction{{Name: "i18n:ui_toolbar_copy_error", Icon: icons.Get(icons.ActionCopy), PreventHideAfterAction: true, Action: func(actionCtx context.Context, _ plugin.ToolbarMsgActionContext) {
			if err := clipboard.WriteText(errorText); err != nil {
				util.GetLogger().Error(actionCtx, "copy update error: "+err.Error())
			}
		}}}
	}
	p.api.ShowToolbarMsg(ctx, status)
}
