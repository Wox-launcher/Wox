package system

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"wox/account"
	"wox/cloudsync"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
)

var (
	cloudSyncIcon     = common.PluginCloudSyncIcon
	cloudSyncPushIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="8" y="14" width="48" height="36" rx="10" fill="#2563eb"/><path fill="#dbeafe" d="M22 42h21a8 8 0 0 0 1.4-15.9A13 13 0 0 0 19.1 29A6.5 6.5 0 0 0 22 42"/><path fill="#1d4ed8" d="M31 40h4V30h5l-7-7l-7 7h5z"/></svg>`)
	cloudSyncPullIcon = common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 64 64"><rect x="8" y="14" width="48" height="36" rx="10" fill="#059669"/><path fill="#d1fae5" d="M22 42h21a8 8 0 0 0 1.4-15.9A13 13 0 0 0 19.1 29A6.5 6.5 0 0 0 22 42"/><path fill="#047857" d="M31 23h4v10h5l-7 7l-7-7h5z"/></svg>`)
)

const (
	cloudSyncHistoryResultLimit = 10
	cloudSyncStatusResultScore  = 1000
	cloudSyncHistoryGroupScore  = 900
	cloudSyncHistoryDetailScore = 800
	cloudSyncDefaultQuery       = "sync"
	cloudSyncHistoryCommand     = "history"
)

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
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
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
		return plugin.NewQueryResponse([]plugin.QueryResult{p.loginRequiredResult(ctx)})
	}
	if historyID, ok := parseCloudSyncHistoryDetailQuery(query.Search); ok {
		return plugin.NewQueryResponse(p.historyDetailResults(ctx, historyID))
	}

	result := plugin.QueryResult{
		Title:      p.statusTitle(accountStatus, status),
		SubTitle:   p.statusSubtitle(ctx, accountStatus, status),
		Icon:       cloudSyncIcon,
		Score:      cloudSyncStatusResultScore,
		GroupScore: cloudSyncStatusResultScore,
		Tails:      p.statusTails(ctx, accountStatus, status),
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

	results := []plugin.QueryResult{result}
	results = append(results, p.historyResults(ctx, query)...)
	return plugin.NewQueryResponse(results)
}

// resolveSearchableStatus only requires a signed-in sync account so free users can inspect local history.
func (p *CloudSyncPlugin) resolveSearchableStatus(ctx context.Context) (account.Status, cloudsync.ServiceStatus, bool) {
	accountService := account.GetService()
	if accountService == nil {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}
	accountStatus := accountService.Status(ctx)
	if !accountStatus.LoggedIn {
		return account.Status{}, cloudsync.ServiceStatus{}, false
	}

	service := cloudsync.GetService()
	if service == nil {
		return accountStatus, cloudsync.ServiceStatus{}, true
	}
	return accountStatus, service.Status(ctx), true
}

func (p *CloudSyncPlugin) loginRequiredResult(ctx context.Context) plugin.QueryResult {
	return plugin.QueryResult{
		Title:      p.tr(ctx, "plugin_cloudsync_login_required_title"),
		SubTitle:   p.tr(ctx, "plugin_cloudsync_login_required_subtitle"),
		Icon:       cloudSyncIcon,
		Score:      cloudSyncStatusResultScore,
		GroupScore: cloudSyncStatusResultScore,
	}
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

// statusSubtitle keeps the top status row focused on actionable sync state.
func (p *CloudSyncPlugin) statusSubtitle(ctx context.Context, accountStatus account.Status, status cloudsync.ServiceStatus) string {
	parts := []string{
		p.labelValue(ctx, "plugin_cloudsync_label_pending", strconv.Itoa(status.PendingCount)),
	}

	if !accountStatus.SyncEnabled {
		parts = append(parts, i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_detail_disabled"))
	}
	if status.State != nil && status.State.LastError != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_label_error", formatCloudSyncError(status.State.LastError)))
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

// historyResults appends recent local push/pull attempts without changing sync state or protocol behavior.
func (p *CloudSyncPlugin) historyResults(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	service := cloudsync.GetService()
	if service == nil || service.HistoryStore == nil {
		return nil
	}

	records, err := service.HistoryStore.ListRecent(ctx, cloudSyncHistoryResultLimit)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to load cloud sync history: %v", err))
		return nil
	}

	results := make([]plugin.QueryResult, 0, len(records))
	for index, record := range records {
		recordID := record.ID
		results = append(results, plugin.QueryResult{
			Id:         fmt.Sprintf("cloudsync-history-%d", record.ID),
			Title:      p.historyTitle(ctx, record),
			SubTitle:   p.historySubtitle(ctx, record),
			Icon:       p.historyIcon(record),
			Score:      cloudSyncHistoryGroupScore - int64(index),
			Group:      "i18n:plugin_cloudsync_history_group",
			GroupScore: cloudSyncHistoryGroupScore,
			Tails:      p.historyTails(ctx, record),
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "i18n:plugin_cloudsync_history_action_view_details",
					Icon:                   common.SearchIcon,
					IsDefault:              true,
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						p.changeQueryToHistoryDetails(ctx, query, recordID)
					},
				},
			},
		})
	}
	return results
}

// historyDetailResults renders the keys touched by one history row without exposing synced values.
func (p *CloudSyncPlugin) historyDetailResults(ctx context.Context, historyID uint) []plugin.QueryResult {
	service := cloudsync.GetService()
	if service == nil || service.HistoryStore == nil {
		return nil
	}

	record, err := service.HistoryStore.Get(ctx, historyID)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to load cloud sync history detail: %v", err))
		return []plugin.QueryResult{{
			Id:         fmt.Sprintf("cloudsync-history-%d-detail-error", historyID),
			Title:      p.tr(ctx, "plugin_cloudsync_history_detail_not_found"),
			Icon:       cloudSyncIcon,
			Score:      cloudSyncHistoryDetailScore,
			Group:      "i18n:plugin_cloudsync_history_detail_group",
			GroupScore: cloudSyncHistoryGroupScore,
		}}
	}

	if len(record.Details) == 0 {
		return []plugin.QueryResult{{
			Id:         fmt.Sprintf("cloudsync-history-%d-detail-empty", historyID),
			Title:      p.tr(ctx, "plugin_cloudsync_history_detail_empty"),
			SubTitle:   p.historySubtitle(ctx, *record),
			Icon:       p.historyIcon(*record),
			Score:      cloudSyncHistoryDetailScore,
			Group:      "i18n:plugin_cloudsync_history_detail_group",
			GroupScore: cloudSyncHistoryGroupScore,
		}}
	}

	results := make([]plugin.QueryResult, 0, len(record.Details))
	for index, detail := range record.Details {
		results = append(results, plugin.QueryResult{
			Id:         fmt.Sprintf("cloudsync-history-%d-detail-%d", historyID, index),
			Title:      p.historyDetailTitle(ctx, detail),
			SubTitle:   p.historyDetailSubtitle(ctx, detail),
			Icon:       p.historyIcon(*record),
			Score:      cloudSyncHistoryDetailScore - int64(index),
			Group:      "i18n:plugin_cloudsync_history_detail_group",
			GroupScore: cloudSyncHistoryGroupScore,
			Tails:      p.historyDetailTails(ctx, detail),
		})
	}
	return results
}

func (p *CloudSyncPlugin) changeQueryToHistoryDetails(ctx context.Context, query plugin.Query, historyID uint) {
	if p.api == nil {
		return
	}
	p.api.ChangeQuery(ctx, common.PlainQuery{
		QueryType: plugin.QueryTypeInput,
		QueryText: fmt.Sprintf("%s %s %d", cloudSyncQueryKeyword(query), cloudSyncHistoryCommand, historyID),
	})
}

// historyIcon gives push and pull rows distinct scan targets while keeping unknown operations on the base sync icon.
func (p *CloudSyncPlugin) historyIcon(record cloudsync.CloudSyncHistoryRecord) common.WoxImage {
	switch record.Operation {
	case cloudsync.CloudSyncProgressOperationPush:
		return cloudSyncPushIcon
	case cloudsync.CloudSyncProgressOperationPull:
		return cloudSyncPullIcon
	default:
		return cloudSyncIcon
	}
}

// historyTitle maps one local history row to a short scan-friendly result title.
func (p *CloudSyncPlugin) historyTitle(ctx context.Context, record cloudsync.CloudSyncHistoryRecord) string {
	switch {
	case record.Operation == cloudsync.CloudSyncProgressOperationPush && record.Status == cloudsync.CloudSyncHistoryStatusSucceeded:
		return p.tr(ctx, "plugin_cloudsync_history_push_succeeded")
	case record.Operation == cloudsync.CloudSyncProgressOperationPush && record.Status == cloudsync.CloudSyncHistoryStatusPartialSucceeded:
		return p.tr(ctx, "plugin_cloudsync_history_push_partial_succeeded")
	case record.Operation == cloudsync.CloudSyncProgressOperationPush && record.Status == cloudsync.CloudSyncHistoryStatusFailed:
		return p.tr(ctx, "plugin_cloudsync_history_push_failed")
	case record.Operation == cloudsync.CloudSyncProgressOperationPull && record.Status == cloudsync.CloudSyncHistoryStatusSucceeded:
		return p.tr(ctx, "plugin_cloudsync_history_pull_succeeded")
	case record.Operation == cloudsync.CloudSyncProgressOperationPull && record.Status == cloudsync.CloudSyncHistoryStatusPartialSucceeded:
		return p.tr(ctx, "plugin_cloudsync_history_pull_partial_succeeded")
	case record.Operation == cloudsync.CloudSyncProgressOperationPull && record.Status == cloudsync.CloudSyncHistoryStatusFailed:
		return p.tr(ctx, "plugin_cloudsync_history_pull_failed")
	case record.Status == cloudsync.CloudSyncHistoryStatusFailed:
		return p.tr(ctx, "plugin_cloudsync_history_sync_failed")
	default:
		return p.tr(ctx, "plugin_cloudsync_history_sync_succeeded")
	}
}

// historySubtitle summarizes diagnostic fields without exposing synced keys or plaintext values.
func (p *CloudSyncPlugin) historySubtitle(ctx context.Context, record cloudsync.CloudSyncHistoryRecord) string {
	parts := []string{}
	if record.Reason != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_source", p.historyReasonLabel(ctx, record.Reason)))
	}
	parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_count", strconv.Itoa(record.ItemCount)))
	if types := p.formatHistoryEntityCounts(ctx, record.EntityCounts); types != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_types", types))
	}
	parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_duration", formatHistoryDuration(record.DurationMs)))
	if record.Status != cloudsync.CloudSyncHistoryStatusSucceeded && record.Error != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_label_error", formatCloudSyncError(record.Error)))
	}

	return strings.Join(parts, " | ")
}

// historyTails mirrors history status as a semantic chip in the launcher row.
func (p *CloudSyncPlugin) historyTails(ctx context.Context, record cloudsync.CloudSyncHistoryRecord) []plugin.QueryResultTail {
	tails := []plugin.QueryResultTail{plugin.NewQueryResultTailText(p.formatTimestamp(ctx, historyTimestamp(record)))}
	if record.Status == cloudsync.CloudSyncHistoryStatusFailed {
		return append(tails, plugin.NewQueryResultTailTextWithCategory(p.tr(ctx, "plugin_cloudsync_history_tail_failed"), plugin.QueryResultTailTextCategoryDanger))
	}
	if record.Status == cloudsync.CloudSyncHistoryStatusPartialSucceeded {
		return append(tails, plugin.NewQueryResultTailTextWithCategory(p.tr(ctx, "plugin_cloudsync_history_tail_partial_succeeded"), plugin.QueryResultTailTextCategoryWarning))
	}
	return append(tails, plugin.NewQueryResultTailTextWithCategory(p.tr(ctx, "plugin_cloudsync_history_tail_succeeded"), plugin.QueryResultTailTextCategorySuccess))
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
		if accountService := account.GetService(); accountService != nil {
			accountStatus := accountService.Status(ctx)
			if accountStatus.SyncEnabled && accountStatus.Plan == "pro" {
				service.StartManager(ctx)
			}
		}
		service.Manager.Pull(ctx, "manual")
		service.Manager.PushPending(ctx, "manual")

		state, err := cloudsync.LoadCloudSyncState(ctx)
		if err != nil {
			p.notifyText(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_notify_failed"), formatCloudSyncError(err.Error())))
		} else if state.LastError != "" {
			p.notifyText(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_notify_failed"), formatCloudSyncError(state.LastError)))
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

// formatTimestamp keeps empty sync timestamps user-facing instead of showing the Unix epoch.
func (p *CloudSyncPlugin) formatTimestamp(ctx context.Context, timestamp int64) string {
	if timestamp == 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "plugin_cloudsync_never")
	}
	return util.FormatTimestamp(timestamp)
}

// formatHistoryEntityCounts renders entity-type counts with stable labels and ordering.
func (p *CloudSyncPlugin) formatHistoryEntityCounts(ctx context.Context, counts map[string]int) string {
	if len(counts) == 0 {
		return ""
	}

	parts := []string{}
	for _, entityType := range sortedCloudSyncHistoryEntityTypes(counts) {
		parts = append(parts, fmt.Sprintf("%s %d", p.historyEntityLabel(ctx, entityType), counts[entityType]))
	}
	return strings.Join(parts, ", ")
}

// historyEntityLabel keeps raw sync entity identifiers out of the normal user-facing path.
func (p *CloudSyncPlugin) historyEntityLabel(ctx context.Context, entityType string) string {
	switch entityType {
	case cloudsync.EntityWoxSetting:
		return p.tr(ctx, "plugin_cloudsync_history_entity_wox_setting")
	case cloudsync.EntityPluginSetting:
		return p.tr(ctx, "plugin_cloudsync_history_entity_plugin_setting")
	case cloudsync.EntityInstalledPlugin:
		return p.tr(ctx, "plugin_cloudsync_history_entity_installed_plugin")
	case cloudsync.EntityInstalledTheme:
		return p.tr(ctx, "plugin_cloudsync_history_entity_installed_theme")
	default:
		return entityType
	}
}

func (p *CloudSyncPlugin) historyDetailSubtitle(ctx context.Context, detail cloudsync.CloudSyncHistoryRecordDetail) string {
	parts := []string{p.historyEntityLabel(ctx, detail.EntityType)}
	if detail.PluginID != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_plugin", p.historyPluginLabel(ctx, detail.PluginID)))
	}
	if detail.Op != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_operation", detail.Op))
	}
	if detail.Status != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_history_label_status", p.historyDetailStatusLabel(ctx, detail.Status)))
	}
	if detail.Error != "" {
		parts = append(parts, p.labelValue(ctx, "plugin_cloudsync_label_error", formatCloudSyncError(detail.Error)))
	}
	return strings.Join(parts, " | ")
}

func (p *CloudSyncPlugin) historyDetailStatusLabel(ctx context.Context, status string) string {
	switch status {
	case cloudsync.CloudSyncHistoryStatusSucceeded:
		return p.tr(ctx, "plugin_cloudsync_history_tail_succeeded")
	case cloudsync.CloudSyncHistoryStatusFailed:
		return p.tr(ctx, "plugin_cloudsync_history_tail_failed")
	default:
		return status
	}
}

// historyDetailTails mirrors the per-record outcome instead of the aggregate history status.
func (p *CloudSyncPlugin) historyDetailTails(ctx context.Context, detail cloudsync.CloudSyncHistoryRecordDetail) []plugin.QueryResultTail {
	if detail.Status == cloudsync.CloudSyncHistoryStatusFailed {
		return []plugin.QueryResultTail{plugin.NewQueryResultTailTextWithCategory(p.tr(ctx, "plugin_cloudsync_history_tail_failed"), plugin.QueryResultTailTextCategoryDanger)}
	}
	return []plugin.QueryResultTail{plugin.NewQueryResultTailTextWithCategory(p.tr(ctx, "plugin_cloudsync_history_tail_succeeded"), plugin.QueryResultTailTextCategorySuccess)}
}

// historyPluginLabel resolves IDs from both installed plugins and store manifests so failed install rows stay readable.
func (p *CloudSyncPlugin) historyPluginLabel(ctx context.Context, pluginID string) string {
	if instance := plugin.GetPluginManager().GetPluginInstanceById(pluginID); instance != nil {
		if name := strings.TrimSpace(instance.GetName(ctx)); name != "" {
			return name
		}
	}
	if manifest, err := plugin.GetStoreManager().GetStorePluginManifestById(ctx, pluginID); err == nil {
		if name := strings.TrimSpace(manifest.GetName(ctx)); name != "" {
			return name
		}
	}
	return pluginID
}

// historyReasonLabel maps internal trigger reasons to labels a normal user can understand.
func (p *CloudSyncPlugin) historyReasonLabel(ctx context.Context, reason string) string {
	switch reason {
	case "manual":
		return p.tr(ctx, "plugin_cloudsync_history_reason_manual")
	case "startup":
		return p.tr(ctx, "plugin_cloudsync_history_reason_startup")
	case "startup-missing-keys":
		return p.tr(ctx, "plugin_cloudsync_history_reason_startup_missing_keys")
	case "periodic-pull":
		return p.tr(ctx, "plugin_cloudsync_history_reason_periodic_pull")
	case "periodic-push":
		return p.tr(ctx, "plugin_cloudsync_history_reason_periodic_push")
	case "first":
		return p.tr(ctx, "plugin_cloudsync_history_reason_first")
	case "tick":
		return p.tr(ctx, "plugin_cloudsync_history_reason_tick")
	case "bootstrap":
		return p.tr(ctx, "plugin_cloudsync_history_reason_bootstrap")
	case "done":
		return p.tr(ctx, "plugin_cloudsync_history_reason_shutdown")
	default:
		return reason
	}
}

func (p *CloudSyncPlugin) tr(ctx context.Context, key string) string {
	return i18n.GetI18nManager().TranslateWox(ctx, key)
}

func (p *CloudSyncPlugin) notify(ctx context.Context, key string) {
	p.notifyText(ctx, i18n.GetI18nManager().TranslateWox(ctx, key))
}

func (p *CloudSyncPlugin) notifyText(ctx context.Context, text string) {
	if p.api != nil {
		p.api.Notify(ctx, text)
	}
}

// sortedCloudSyncHistoryEntityTypes keeps the common entity types in product order and unknown types deterministic.
func sortedCloudSyncHistoryEntityTypes(counts map[string]int) []string {
	ordered := []string{
		cloudsync.EntityWoxSetting,
		cloudsync.EntityPluginSetting,
		cloudsync.EntityInstalledPlugin,
		cloudsync.EntityInstalledTheme,
	}
	seen := map[string]struct{}{}
	result := []string{}
	for _, entityType := range ordered {
		if counts[entityType] > 0 {
			result = append(result, entityType)
			seen[entityType] = struct{}{}
		}
	}

	other := []string{}
	for entityType, count := range counts {
		if count <= 0 {
			continue
		}
		if _, ok := seen[entityType]; ok {
			continue
		}
		other = append(other, entityType)
	}
	sort.Strings(other)
	return append(result, other...)
}

// historyTimestamp prefers completion time because it matches when the user sees the outcome.
func historyTimestamp(record cloudsync.CloudSyncHistoryRecord) int64 {
	if record.FinishedAt > 0 {
		return record.FinishedAt
	}
	return record.StartedAt
}

func parseCloudSyncHistoryDetailQuery(search string) (uint, bool) {
	parts := strings.Fields(search)
	if len(parts) != 2 || parts[0] != cloudSyncHistoryCommand {
		return 0, false
	}
	id, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil || id == 0 {
		return 0, false
	}
	return uint(id), true
}

func cloudSyncQueryKeyword(query plugin.Query) string {
	if query.TriggerKeyword != "" {
		return query.TriggerKeyword
	}
	return cloudSyncDefaultQuery
}

func (p *CloudSyncPlugin) historyDetailTitle(ctx context.Context, detail cloudsync.CloudSyncHistoryRecordDetail) string {
	if detail.EntityType == cloudsync.EntityInstalledPlugin && detail.Key != "" {
		return p.historyPluginLabel(ctx, detail.Key)
	}
	if detail.Key != "" {
		return detail.Key
	}
	if detail.PluginID != "" {
		return detail.PluginID
	}
	return detail.EntityType
}

// formatHistoryDuration keeps short sync attempts readable without locale-specific formatting.
func formatHistoryDuration(durationMs int64) string {
	if durationMs < 1000 {
		return fmt.Sprintf("%dms", durationMs)
	}
	if durationMs%1000 == 0 {
		return fmt.Sprintf("%ds", durationMs/1000)
	}
	return fmt.Sprintf("%.1fs", float64(durationMs)/1000)
}

// truncateCloudSyncHistoryError keeps long backend errors from crowding the launcher row.
func truncateCloudSyncHistoryError(message string) string {
	const maxErrorLength = 120
	runes := []rune(message)
	if len(runes) <= maxErrorLength {
		return message
	}
	return string(runes[:maxErrorLength]) + "..."
}

func formatCloudSyncError(message string) string {
	return truncateCloudSyncHistoryError(stripCloudSyncErrorDecorations(message))
}

func stripCloudSyncErrorDecorations(message string) string {
	result := strings.TrimSpace(message)
	for {
		parts := strings.SplitN(result, ": ", 2)
		if len(parts) != 2 {
			return result
		}
		head := strings.TrimSpace(parts[0])
		if !isCloudSyncErrorCode(head) && !isCloudSyncErrorWrapper(head) {
			return result
		}
		result = strings.TrimSpace(parts[1])
	}
}

func isCloudSyncErrorWrapper(value string) bool {
	switch value {
	case "cloud sync push failed", "cloud sync pull failed", "cloud sync record key list failed", "cloud sync device update failed", "cloud sync device join failed":
		return true
	default:
		return false
	}
}

func isCloudSyncErrorCode(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			continue
		}
		return false
	}
	return true
}
