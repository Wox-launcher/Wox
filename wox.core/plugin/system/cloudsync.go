package system

import (
	"context"
	"fmt"
	"strings"
	"wox/account"
	"wox/cloudsync"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
)

var cloudSyncIcon = common.PluginCloudSyncIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &CloudSyncPlugin{})
}

type CloudSyncPlugin struct {
	api plugin.API
}

// GetMetadata declares the local query surface for cloud sync status and manual sync.
func (p *CloudSyncPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "8867b6af-dc86-45d5-bcc3-32966adcee27",
		Name:          "i18n:plugin_cloudsync_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_cloudsync_plugin_description",
		Icon:          cloudSyncIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"cloudsync",
			"sync",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (p *CloudSyncPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

// Query only exposes sync state after the account and encryption prerequisites are satisfied.
func (p *CloudSyncPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	accountStatus, status, ok := p.resolveSearchableStatus(ctx)
	if !ok {
		return plugin.QueryResponse{}
	}

	result := plugin.QueryResult{
		Title:    p.statusTitle(accountStatus, status),
		SubTitle: p.statusSubtitle(ctx, accountStatus, status),
		Icon:     cloudSyncIcon,
		Tails:    p.statusTails(ctx, accountStatus, status),
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "i18n:plugin_cloudsync_action_sync_now",
				Icon:                   common.UpdateIcon,
				IsDefault:              true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					p.syncNow(ctx)
				},
			},
		},
	}

	return plugin.NewQueryResponse([]plugin.QueryResult{result})
}

// resolveSearchableStatus enforces the product gate before this system plugin returns any result.
func (p *CloudSyncPlugin) resolveSearchableStatus(ctx context.Context) (account.Status, cloudsync.ServiceStatus, bool) {
	accountService := account.GetService()
	if accountService == nil {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}
	accountStatus := accountService.Status(ctx)
	if !accountStatus.LoggedIn || !accountStatus.SyncEligible {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}

	service := cloudsync.GetService()
	if service == nil || service.KeyManager == nil || service.Manager == nil {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}
	status := service.Status(ctx)
	if !status.KeyStatus.Available {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}
	return accountStatus, status, true
}

// statusTitle maps the persisted sync state to a single scan-friendly result title.
func (p *CloudSyncPlugin) statusTitle(accountStatus account.Status, status cloudsync.ServiceStatus) string {
	if !accountStatus.SyncEnabled {
		return "i18n:plugin_cloudsync_status_disabled"
	}
	if status.State != nil && status.State.LastError != "" {
		return "i18n:plugin_cloudsync_status_error"
	}
	if status.State != nil && status.State.BackoffUntil > util.GetSystemTimestamp() {
		return "i18n:plugin_cloudsync_status_retrying"
	}
	if status.State != nil && !status.State.Bootstrapped {
		return "i18n:plugin_cloudsync_status_initializing"
	}
	return "i18n:plugin_cloudsync_status_active"
}

// statusSubtitle includes the important local timestamps without requiring a dedicated preview.
func (p *CloudSyncPlugin) statusSubtitle(ctx context.Context, accountStatus account.Status, status cloudsync.ServiceStatus) string {
	parts := []string{
		p.labelValue(ctx, "plugin_cloudsync_label_account", accountStatus.Email),
		p.labelValue(ctx, "plugin_cloudsync_label_last_sync", p.formatTimestamp(ctx, p.lastSyncTimestamp(status.State))),
	}

	if !accountStatus.SyncEnabled {
		parts = append(parts, i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_detail_disabled"))
	}
	if status.State != nil && status.State.LastError != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_label_error", status.State.LastError))
	}
	if status.State != nil && status.State.BackoffUntil > util.GetSystemTimestamp() {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_label_retry_after", util.FormatTimestamp(status.State.BackoffUntil)))
	}

	return strings.Join(parts, " | ")
}

// statusTails surfaces the most actionable state for quick scanning and filtering.
func (p *CloudSyncPlugin) statusTails(ctx context.Context, accountStatus account.Status, status cloudsync.ServiceStatus) []plugin.QueryResultTail {
	if status.State != nil && status.State.LastError != "" {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailTextWithCategory(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_tail_error"), plugin.QueryResultTailTextCategoryDanger)}
	}
	if status.State != nil && status.State.BackoffUntil > util.GetSystemTimestamp() {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailTextWithCategory(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_tail_retrying"), plugin.QueryResultTailTextCategoryWarning)}
	}
	if !accountStatus.SyncEnabled {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailText(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_tail_disabled"))}
	}
	return []plugin.QueryResultTail{plugin.NewQueryResultTailTextWithCategory(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_tail_active"), plugin.QueryResultTailTextCategorySuccess)}
}

// syncNow performs one bidirectional manual sync and refreshes the current launcher row when done.
func (p *CloudSyncPlugin) syncNow(ctx context.Context) {
	service := cloudsync.GetService()
	if service == nil || service.Manager == nil {
		p.notify(ctx, "plugin_cloudsync_notify_not_configured")
		return
	}

	p.notify(ctx, "plugin_cloudsync_notify_syncing")
	util.Go(ctx, "cloud sync manual sync", func() {
		if accountService := account.GetService(); accountService != nil && accountService.Status(ctx).SyncEnabled {
			service.StartManager(ctx)
		}
		service.Manager.Pull(ctx, "manual")
		service.Manager.PushPending(ctx, "manual")

		state, err := cloudsync.LoadCloudSyncState(ctx)
		if err != nil {
			p.notifyText(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_notify_failed"), err.Error()))
		} else if state.LastError != "" {
			p.notifyText(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_notify_failed"), state.LastError))
		} else {
			p.notify(ctx, "plugin_cloudsync_notify_done")
		}
		if p.api != nil {
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}
	})
}

func (p *CloudSyncPlugin) labelValue(ctx context.Context, labelKey string, value string) string {
	return fmt.Sprintf("%s: %s", i18n.GetI18nManager().TranslateWox(ctx, labelKey), value)
}

// lastSyncTimestamp reports the latest completed sync activity regardless of sync direction.
func (p *CloudSyncPlugin) lastSyncTimestamp(state *cloudsync.CloudSyncStateView) int64 {
	if state == nil {
		return 0
	}
	if state.LastPullTs > state.LastPushTs {
		return state.LastPullTs
	}
	return state.LastPushTs
}

// formatTimestamp keeps empty sync timestamps user-facing instead of showing the Unix epoch.
func (p *CloudSyncPlugin) formatTimestamp(ctx context.Context, timestamp int64) string {
	if timestamp == 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_never")
	}
	return util.FormatTimestamp(timestamp)
}

func (p *CloudSyncPlugin) notify(ctx context.Context, key string) {
	p.notifyText(ctx, i18n.GetI18nManager().TranslateWox(ctx, key))
}

func (p *CloudSyncPlugin) notifyText(ctx context.Context, text string) {
	if p.api != nil {
		p.api.Notify(ctx, text)
	}
}
