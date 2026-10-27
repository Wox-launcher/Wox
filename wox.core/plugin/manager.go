package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"wox/ai"
	"wox/analytics"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/network"
	"wox/setting"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/browser"
	"wox/util/notifier"
	"wox/util/selection"
	"wox/util/timetracking"
	"wox/util/window"

	"github.com/samber/lo"
	"github.com/wissance/stringFormatter"
)

var managerInstance *Manager
var managerOnce sync.Once
var logger *util.Log

const (
	previewDataMaxSize          = 1024
	globalQueryPluginScoreLimit = 200
	// rankAboveUsageScore sits above action history. Query pins are a separate display tier.
	rankAboveUsageScore int64 = 50000
	fileSearchPluginID        = common.FileSearchPluginID
)

// pluginQueryInput is the prepared input for one plugin query execution.
// It keeps metadata-derived UI state, the filtered plugin query, and optional
// requirement-blocking response together so queryForPlugin can read as stages.
type pluginQueryInput struct {
	// query is the plugin-facing query after env filtering.
	query Query
	// metadataLayout is the layout derived from plugin metadata before Plugin.Query runs.
	metadataLayout QueryLayout
	// queryContext is the backend-owned query classification returned to UI.
	queryContext QueryContext
	// blocked means query requirements produced a settings row instead of calling Plugin.Query.
	blocked bool
	// blockedResponse is returned directly when requirements prevent Plugin.Query.
	blockedResponse QueryResponse
}

type Manager struct {
	resultBindingRestores        sync.Map
	applyResultBindings          ResultBindingApplier
	runtimeTriggerRegistrationMu sync.Mutex
	lifecycle                    pluginLifecycle
	presentation                 pluginPresentation
	ui                           common.UI
	resultStore                  *queryResultStore
	queryScheduler               *queryScheduler
	autoQueryHistory             *autoQueryHistoryRecorder
	aiProviders                  *util.HashMap[string, ai.Provider]
	activeBrowserUrl             string // Active browser URL before Wox is activated.
}

const (
	systemActionPinInQueryID        = "__system_pin_in_query__"
	systemActionUnpinInQueryID      = "__system_unpin_in_query__"
	systemActionResetRankingID      = "__system_reset_ranking__"
	systemActionAddQueryAliasID     = "__system_add_query_alias__"
	systemActionOpenPluginSettingID = "__system_open_plugin_setting__"
	systemActionRemoveFromMRUID     = "__system_remove_from_mru__"
	systemActionSetResultHotkeyID   = "__system_set_result_hotkey__"
	systemActionSetResultAliasID    = "__system_set_result_alias__"

	queryAliasFormAliasKey = "alias"
	queryAliasFormQueryKey = "query"
)

func GetPluginManager() *Manager {
	managerOnce.Do(func() {
		managerInstance = newManager()
		logger = util.GetLogger()
	})
	return managerInstance
}

// newManager wires independent owners without giving them access to query orchestration.
func newManager() *Manager {
	m := &Manager{
		resultStore:    newQueryResultStore(),
		queryScheduler: newQueryScheduler(),
		aiProviders:    util.NewHashMap[string, ai.Provider](),
		autoQueryHistory: newAutoQueryHistoryRecorder(autoRecordQueryHistoryDelay, func(ctx context.Context, query common.PlainQuery) {
			setting.GetSettingManager().AddQueryHistory(ctx, query)
		}),
	}
	m.lifecycle = newPluginLifecycle()
	m.presentation = newPluginPresentation(m.lifecycle.GetPluginInstanceById, func() common.UI { return m.ui })
	m.lifecycle.releasePresentation = m.presentation.releasePlugin
	return m
}

func (m *Manager) Start(ctx context.Context, ui common.UI) error {
	m.ui = ui

	loadErr := m.lifecycle.loadPlugins(ctx)
	if loadErr != nil {
		return fmt.Errorf("failed to load plugins: %w", loadErr)
	}
	if util.IsThirdPartyPluginsDisabled() {
		return nil
	}

	m.lifecycle.startScriptPluginMonitoring(util.NewTraceContext())
	m.lifecycle.startSingleFilePluginMonitoring(util.NewTraceContext())

	// Start shared runtime host health monitoring
	m.lifecycle.startHostWatchdog(ctx)

	util.Go(ctx, "start store manager", func() {
		GetStoreManager().Start(util.NewTraceContext())
	})

	return nil
}

func (m *Manager) SetActiveBrowserUrl(url string) {
	m.activeBrowserUrl = url
}

func pluginInstanceDisabled(instance *Instance) bool {
	if instance == nil || instance.Setting == nil || instance.Setting.Disabled == nil {
		return false
	}
	return instance.Setting.Disabled.Get()
}

func (m *Manager) canOperateQuery(ctx context.Context, pluginInstance *Instance, query Query) bool {
	if network.IsOffline() && pluginInstance.Host != nil {
		return false
	}
	if pluginInstanceDisabled(pluginInstance) {
		return false
	}

	if query.HasScope() {
		if _, ok := query.Scope.Find(pluginInstance.Metadata.Id); !ok {
			return false
		}
		if query.Type == QueryTypeSelection {
			return pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQuerySelection)
		}
		return query.Type == QueryTypeInput
	}

	if query.Type == QueryTypeSelection {
		// Keyword-targeted and global selection both require the querySelection
		// feature. Matching a trigger keyword alone must not deliver a file
		// selection to a plugin that never declared that capability.
		if query.TriggerKeyword != "" && !lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword) {
			return false
		}
		return pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQuerySelection)
	}

	var validGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), "*") && query.TriggerKeyword == ""
	var validNonGlobalQuery = lo.Contains(pluginInstance.GetTriggerKeywords(), query.TriggerKeyword)
	if !validGlobalQuery && !validNonGlobalQuery {
		return false
	}

	return true
}

// applyScopeForPlugin returns a per-plugin query copy with Command/TriggerKeyword from Scope.
func (m *Manager) applyScopeForPlugin(query Query, pluginInstance *Instance) Query {
	if !query.HasScope() || pluginInstance == nil {
		return query
	}
	pluginQuery := query
	pluginQuery.Scope = query.Scope.Clone()
	if scoped, ok := query.Scope.Find(pluginInstance.Metadata.Id); ok {
		pluginQuery.Command = scoped.Command
	}
	pluginQuery.TriggerKeyword = pluginInstance.PrimaryTriggerKeyword()
	return pluginQuery
}

// buildMetadataBackedQueryLayout converts the static plugin metadata that used
// to be fetched through /query/metadata into the QueryResponse layout channel.
// Keeping this in the query pipeline removes the extra UI HTTP request while
// preserving command-scoped preview ratios and grid layout behavior.
func (m *Manager) buildMetadataBackedQueryLayout(ctx context.Context, pluginInstance *Instance, query Query) QueryLayout {
	layout := QueryLayout{}
	if pluginInstance == nil {
		return layout
	}

	iconImg, parseErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
	if parseErr == nil {
		convertedIcon := pluginInstance.ConvertIcon(ctx, iconImg)
		layout.Icon = &convertedIcon
	} else {
		logger.Error(ctx, fmt.Sprintf("failed to parse icon: %s", parseErr.Error()))
	}

	defaultWidthRatio := 0.4
	layout.ResultPreviewWidthRatio = &defaultWidthRatio

	featureParams, isResultPreviewWidthRatioEnabled, err := pluginInstance.Metadata.GetFeatureParamsForResultPreviewWidthRatioCommand(query.Command)
	if err == nil && isResultPreviewWidthRatioEnabled {
		// Command-scoped preview width still belongs to the plugin metadata
		// contract. Moving it into QueryResponse keeps zero-width preview-only
		// commands working without requiring the UI to race a side request.
		widthRatio := featureParams.WidthRatio
		layout.ResultPreviewWidthRatio = &widthRatio
	} else if err != nil && !errors.Is(err, ErrFeatureNotSupported) {
		logger.Error(ctx, fmt.Sprintf("failed to get feature params for result preview width ratio: %s", err.Error()))
	}

	featureParamsGridLayout, isGridLayoutEnabled, err := pluginInstance.Metadata.GetFeatureParamsForGridLayoutCommand(query.Command)
	if err == nil && isGridLayoutEnabled {
		layout.GridLayout = &featureParamsGridLayout
	} else if err != nil && !errors.Is(err, ErrFeatureNotSupported) {
		logger.Error(ctx, fmt.Sprintf("failed to get feature params for grid layout: %s", err.Error()))
	}

	return layout
}

func (m *Manager) mergeQueryLayouts(metadataLayout QueryLayout, responseLayout QueryLayout) QueryLayout {
	merged := metadataLayout

	if responseLayout.Icon != nil && !responseLayout.Icon.IsEmpty() {
		merged.Icon = responseLayout.Icon
	}
	if responseLayout.ResultPreviewWidthRatio != nil {
		// QueryResponse layout can override metadata defaults. A nil pointer
		// means unset, while a non-nil zero is an intentional preview-only ratio.
		merged.ResultPreviewWidthRatio = responseLayout.ResultPreviewWidthRatio
	}
	if responseLayout.GridLayout != nil {
		merged.GridLayout = responseLayout.GridLayout
	}
	if responseLayout.ChatMode {
		merged.ChatMode = true
	}
	if responseLayout.WindowWidth > 0 {
		merged.WindowWidth = responseLayout.WindowWidth
	}
	if responseLayout.WindowHeight > 0 {
		merged.WindowHeight = responseLayout.WindowHeight
	}

	return merged
}

func (m *Manager) queryForPlugin(ctx context.Context, pluginInstance *Instance, query Query) (response QueryResponse) {
	query = m.applyScopeForPlugin(query, pluginInstance)
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	queryForPluginStart := util.GetSystemTimestamp()
	queryForPluginTimingStart := time.Now()
	var startLogUs int64
	if tracker := timetracking.New("query_for_plugin_start"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		startLogStart := time.Now()
		tracker.Log(ctx)
		startLogUs = time.Since(startLogStart).Microseconds()
	}
	afterStartLog := time.Now()
	input := pluginQueryInput{
		query:        query,
		queryContext: BuildQueryContext(query, pluginInstance),
	}
	defer util.GoRecover(ctx, fmt.Sprintf("<%s> query panic", pluginInstance.GetName(ctx)), func(err error) {
		if tracker := timetracking.New("query_for_plugin_recovered"); tracker.Enabled() {
			tracker.SetRawString("queryId", query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-queryForPluginStart)
			tracker.Log(ctx)
		}
		response = m.buildFailedPluginQueryResponse(ctx, pluginInstance, input.query, input.metadataLayout, input.queryContext, err)
	})

	inputStart := util.GetSystemTimestamp()
	inputTimingStart := time.Now()
	preInputUs := inputTimingStart.Sub(afterStartLog).Microseconds()
	input = m.buildPluginQueryInput(ctx, pluginInstance, query)
	if tracker := timetracking.New("build_plugin_query_input"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetBool("blocked", input.blocked)
		tracker.SetInt64("startLogUs", startLogUs)
		tracker.SetInt64("preInputUs", preInputUs)
		tracker.SetInt64("buildUs", time.Since(inputTimingStart).Microseconds())
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-inputStart)
		tracker.Log(ctx)
	}
	if input.blocked {
		if tracker := timetracking.New("query_for_plugin_blocked"); tracker.Enabled() {
			tracker.SetRawString("queryId", query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt("resultCount", len(input.blockedResponse.Results))
			tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-queryForPluginStart)
			tracker.Log(ctx)
		}
		return input.blockedResponse
	}

	response, pluginQueryCost, recovered := m.executePluginQuery(ctx, pluginInstance, input.query, input.metadataLayout, input.queryContext)
	if recovered {
		if tracker := timetracking.New("query_for_plugin_plugin_recovered"); tracker.Enabled() {
			tracker.SetRawString("queryId", query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-queryForPluginStart)
			tracker.Log(ctx)
		}
		return response
	}

	finalizeStart := util.GetSystemTimestamp()
	finalizeTimingStart := time.Now()
	response = m.finalizePluginQueryResponse(ctx, pluginInstance, input.query, response, input.metadataLayout, input.queryContext, pluginQueryCost)
	if tracker := timetracking.New("query_for_plugin_finish"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt64("finalizeCostMs", util.GetSystemTimestamp()-finalizeStart)
		tracker.SetInt64("finalizeCostUs", time.Since(finalizeTimingStart).Microseconds())
		tracker.SetInt64("totalMs", util.GetSystemTimestamp()-queryForPluginStart)
		tracker.SetInt64("totalUs", time.Since(queryForPluginTimingStart).Microseconds())
		tracker.Log(ctx)
	}
	return response
}

func (m *Manager) buildPluginQueryInput(ctx context.Context, pluginInstance *Instance, query Query) pluginQueryInput {
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	tracker := timetracking.New("build_plugin_query_input_base")
	layoutStart := util.GetSystemTimestamp()
	layoutTimingStart := time.Now()
	metadataLayout := m.buildMetadataBackedQueryLayout(ctx, pluginInstance, query)
	layoutCost := util.GetSystemTimestamp() - layoutStart
	layoutCostUs := time.Since(layoutTimingStart).Microseconds()
	contextStart := util.GetSystemTimestamp()
	queryContext := BuildQueryContext(query, pluginInstance)
	contextCost := util.GetSystemTimestamp() - contextStart
	input := pluginQueryInput{
		query:          query,
		metadataLayout: metadataLayout,
		queryContext:   queryContext,
	}
	if tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetInt64("layoutMs", layoutCost)
		tracker.SetInt64("layoutUs", layoutCostUs)
		tracker.SetInt64("contextMs", contextCost)
		// One write avoids serializing every concurrent plugin three times before Query.
		defer tracker.Log(ctx)
	}

	// Query requirements are checked before calling Plugin.Query so plugins do not
	// need to duplicate settings-gate UI rows in every implementation.
	requirementStart := util.GetSystemTimestamp()
	if requirementResult, blocked := m.buildQueryRequirementSettingsResult(ctx, pluginInstance, query); blocked {
		input.blocked = true
		layout := previewOnlyQueryLayout(input.metadataLayout.Icon)
		input.blockedResponse = QueryResponse{
			Results: []QueryResult{m.PolishResult(ctx, pluginInstance, query, layout, requirementResult)},
			Layout:  layout,
			Context: input.queryContext,
		}
		if tracker.Enabled() {
			tracker.SetBool("blocked", true)
			tracker.SetInt64("requirementMs", util.GetSystemTimestamp()-requirementStart)
		}
		return input
	}
	if tracker.Enabled() {
		tracker.SetBool("blocked", false)
		tracker.SetInt64("requirementMs", util.GetSystemTimestamp()-requirementStart)
	}

	envStart := util.GetSystemTimestamp()
	input.query = m.buildPluginQueryEnv(ctx, pluginInstance, query)
	if tracker.Enabled() {
		tracker.SetInt64("envMs", util.GetSystemTimestamp()-envStart)
	}
	return input
}

func (m *Manager) buildPluginQueryEnv(ctx context.Context, pluginInstance *Instance, query Query) Query {
	// Query env is intentionally opt-in per plugin because active-window and
	// browser context can be expensive or sensitive. Keep only the fields the
	// plugin declared so the SDK contract stays narrow.
	currentEnv := query.Env
	query.Env = QueryEnv{}
	if !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureQueryEnv) {
		return query
	}

	queryEnvParams, err := pluginInstance.Metadata.GetFeatureParamsForQueryEnv()
	if err != nil {
		logger.Error(ctx, fmt.Sprintf("<%s> invalid query env config: %s", pluginInstance.GetName(ctx), err))
		return query
	}

	if queryEnvParams.RequireActiveWindowName {
		query.Env.ActiveWindowTitle = currentEnv.ActiveWindowTitle
	}
	if queryEnvParams.RequireActiveWindowPid {
		query.Env.ActiveWindowPid = currentEnv.ActiveWindowPid
	}
	if queryEnvParams.RequireActiveWindowId {
		query.Env.ActiveWindowId = currentEnv.ActiveWindowId
	}
	if queryEnvParams.RequireActiveWindowIcon {
		query.Env.ActiveWindowIcon = currentEnv.ActiveWindowIcon
	}
	if queryEnvParams.RequireActiveWindowIsOpenSaveDialog {
		query.Env.ActiveWindowIsOpenSaveDialog = currentEnv.ActiveWindowIsOpenSaveDialog
	}
	if queryEnvParams.RequireActiveWindowIsOpenSaveDialogSelectFolder {
		query.Env.ActiveWindowIsOpenSaveDialogSelectFolder = currentEnv.ActiveWindowIsOpenSaveDialogSelectFolder
		// Folder-only is a stricter open/save-dialog state. Preserve the parent
		// flag even when a plugin requests only the stricter field.
		query.Env.ActiveWindowIsOpenSaveDialog = currentEnv.ActiveWindowIsOpenSaveDialog
	}
	if queryEnvParams.RequireActiveBrowserUrl {
		query.Env.ActiveBrowserUrl = currentEnv.ActiveBrowserUrl
	}

	return query
}

func (m *Manager) executePluginQuery(ctx context.Context, pluginInstance *Instance, query Query, metadataLayout QueryLayout, queryContext QueryContext) (response QueryResponse, pluginQueryCost int64, recovered bool) {
	logger.Info(ctx, fmt.Sprintf("<%s> start query: %s", pluginInstance.GetName(ctx), query.RawQuery))
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	start := util.GetSystemTimestamp()
	if tracker := timetracking.New("plugin_query_start"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetRawString("rawQuery", query.RawQuery)
		tracker.Log(ctx)
	}
	defer util.GoRecover(ctx, fmt.Sprintf("<%s> query panic", pluginInstance.GetName(ctx)), func(err error) {
		recovered = true
		if tracker := timetracking.New("plugin_query_recovered"); tracker.Enabled() {
			tracker.SetRawString("queryId", query.Id)
			tracker.SetRawString("plugin", pluginLabel)
			tracker.SetInt64("elapsedMs", util.GetSystemTimestamp()-start)
			tracker.Log(ctx)
		}
		response = m.buildFailedPluginQueryResponse(ctx, pluginInstance, query, metadataLayout, queryContext, err)
	})
	response = pluginInstance.Plugin.Query(ctx, query)
	pluginQueryCost = util.GetSystemTimestamp() - start
	if tracker := timetracking.New("plugin_query_end"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt64("costMs", pluginQueryCost)
		tracker.Log(ctx)
	}
	return response, pluginQueryCost, false
}

func (m *Manager) finalizePluginQueryResponse(ctx context.Context, pluginInstance *Instance, query Query, response QueryResponse, metadataLayout QueryLayout, queryContext QueryContext, pluginQueryCost int64) QueryResponse {
	pluginLabel := queryDiagnosticPluginLabel(pluginInstance)
	finalizeStart := util.GetSystemTimestamp()
	finalizeTimingStart := time.Now()
	layoutStart := util.GetSystemTimestamp()
	layoutTimingStart := time.Now()
	response.Layout = m.mergeQueryLayouts(metadataLayout, response.Layout)
	layoutCost := util.GetSystemTimestamp() - layoutStart
	layoutCostUs := time.Since(layoutTimingStart).Microseconds()
	response.Context = queryContext
	resultsStart := util.GetSystemTimestamp()
	resultsTimingStart := time.Now()
	var totalDefaultActionsCost int64
	var totalDefaultActionsCostUs int64
	var totalPolishCost int64
	var totalPolishCostUs int64
	var totalRecordPluginElapsedCost int64
	var totalRecordPluginElapsedCostUs int64
	var maxResultCost int64
	var maxResultCostUs int64
	var maxResultIndex int
	var maxResultId string
	var maxResultTitle string
	var resultsAggregate timetracking.ResultsPolishAggregate
	collectResultTiming := util.IsDev()
	openPluginSettingActionStart := time.Now()
	openPluginSettingAction := m.newOpenPluginSettingAction(ctx, pluginInstance)
	openPluginSettingActionUs := time.Since(openPluginSettingActionStart).Microseconds()
	// Default and plugin actions often repeat the same icons across every result.
	actionIconCache := make(map[common.WoxImage]common.WoxImage)
	resultIconCache := make(map[common.WoxImage]common.WoxImage)
	for i := range response.Results {
		resultStart := util.GetSystemTimestamp()
		resultTimingStart := time.Now()
		defaultActionsStart := util.GetSystemTimestamp()
		defaultActionsTimingStart := time.Now()
		defaultActions := m.getDefaultActionsWithOpenPluginSettingAction(ctx, pluginInstance, query, response.Results[i], "", openPluginSettingAction)
		defaultActionsCost := util.GetSystemTimestamp() - defaultActionsStart
		defaultActionsCostUs := time.Since(defaultActionsTimingStart).Microseconds()
		totalDefaultActionsCost += defaultActionsCost
		totalDefaultActionsCostUs += defaultActionsCostUs
		response.Results[i].Actions = append(response.Results[i].Actions, defaultActions...)
		polishStart := util.GetSystemTimestamp()
		polishTimingStart := time.Now()
		var polishTiming timetracking.ResultPolishTiming
		response.Results[i], polishTiming = m.polishResult(ctx, pluginInstance, query, response.Layout, response.Results[i], collectResultTiming, actionIconCache, resultIconCache, "", aliasMatchNone)
		polishCost := util.GetSystemTimestamp() - polishStart
		polishCostUs := time.Since(polishTimingStart).Microseconds()
		totalPolishCost += polishCost
		totalPolishCostUs += polishCostUs
		recordPluginElapsedStart := util.GetSystemTimestamp()
		recordPluginElapsedTimingStart := time.Now()
		m.RecordQueryResultPluginQueryElapsed(query.SessionId, query.Id, response.Results[i].Id, pluginQueryCost)
		recordPluginElapsedCost := util.GetSystemTimestamp() - recordPluginElapsedStart
		recordPluginElapsedCostUs := time.Since(recordPluginElapsedTimingStart).Microseconds()
		totalRecordPluginElapsedCost += recordPluginElapsedCost
		totalRecordPluginElapsedCostUs += recordPluginElapsedCostUs
		resultCost := util.GetSystemTimestamp() - resultStart
		resultCostUs := time.Since(resultTimingStart).Microseconds()
		if resultCost > maxResultCost || (resultCost == maxResultCost && resultCostUs > maxResultCostUs) {
			maxResultCost = resultCost
			maxResultCostUs = resultCostUs
			maxResultIndex = i
			maxResultId = response.Results[i].Id
			maxResultTitle = response.Results[i].Title
		}
		if collectResultTiming {
			resultsAggregate.Add(timetracking.ResultIdentity{
				Index: i,
				Id:    response.Results[i].Id,
				Title: response.Results[i].Title,
			}, defaultActionsCostUs, recordPluginElapsedCostUs, resultCostUs, polishTiming)
		}
	}
	resultsCost := util.GetSystemTimestamp() - resultsStart
	resultsCostUs := time.Since(resultsTimingStart).Microseconds()

	filterStart := util.GetSystemTimestamp()
	filterTimingStart := time.Now()
	if query.Type == QueryTypeSelection && query.Search != "" {
		response.Results = lo.Filter(response.Results, func(item QueryResult, _ int) bool {
			return IsStringMatch(ctx, item.Title, query.Search)
		})
	}
	filterCost := util.GetSystemTimestamp() - filterStart
	filterCostUs := time.Since(filterTimingStart).Microseconds()
	finalizeCost := util.GetSystemTimestamp() - finalizeStart
	finalizeCostUs := time.Since(finalizeTimingStart).Microseconds()

	if pluginQueryCost >= 10 {
		logger.Debug(ctx, fmt.Sprintf("<%s> finish query, result count: %d, cost: %dms, query is slow", pluginInstance.GetName(ctx), len(response.Results), pluginQueryCost))
	} else {
		logger.Debug(ctx, fmt.Sprintf("<%s> finish query, result count: %d, cost: %dms", pluginInstance.GetName(ctx), len(response.Results), pluginQueryCost))
	}
	if collectResultTiming {
		resultsAggregate.Log(ctx, query.Id, pluginLabel)
	}
	if tracker := timetracking.New("finalize_done"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("plugin", pluginLabel)
		tracker.SetInt("resultCount", len(response.Results))
		tracker.SetInt64("pluginQueryMs", pluginQueryCost)
		tracker.SetInt64("layoutMs", layoutCost)
		tracker.SetInt64("layoutUs", layoutCostUs)
		tracker.SetInt64("openSettingActionUs", openPluginSettingActionUs)
		tracker.SetInt64("resultsMs", resultsCost)
		tracker.SetInt64("resultsUs", resultsCostUs)
		tracker.SetInt64("defaultActionsMs", totalDefaultActionsCost)
		tracker.SetInt64("defaultActionsUs", totalDefaultActionsCostUs)
		tracker.SetInt64("polishMs", totalPolishCost)
		tracker.SetInt64("polishUs", totalPolishCostUs)
		tracker.SetInt64("recordPluginElapsedMs", totalRecordPluginElapsedCost)
		tracker.SetInt64("recordPluginElapsedUs", totalRecordPluginElapsedCostUs)
		tracker.SetInt64("filterMs", filterCost)
		tracker.SetInt64("filterUs", filterCostUs)
		tracker.SetInt64("maxResultMs", maxResultCost)
		tracker.SetInt64("maxResultUs", maxResultCostUs)
		tracker.SetInt("maxResultIndex", maxResultIndex)
		tracker.SetRawString("maxResultId", maxResultId)
		tracker.SetString("maxResultTitle", maxResultTitle)
		tracker.SetInt64("totalMs", finalizeCost)
		tracker.SetInt64("totalUs", finalizeCostUs)
		tracker.Log(ctx)
	}

	return response
}

func (m *Manager) buildFailedPluginQueryResponse(ctx context.Context, pluginInstance *Instance, query Query, metadataLayout QueryLayout, queryContext QueryContext, err error) QueryResponse {
	// Panic fallback keeps one failed plugin from breaking the whole query run.
	// The failed row is polished like normal results so actions, icons, and cache
	// behavior stay compatible with the existing result pipeline.
	failedResult := m.GetResultForFailedQuery(ctx, pluginInstance.Metadata, query, err)
	return QueryResponse{
		Results: []QueryResult{
			m.PolishResult(ctx, pluginInstance, query, metadataLayout, failedResult),
		},
		Layout:  metadataLayout,
		Context: queryContext,
	}
}

func (m *Manager) GetResultForFailedQuery(ctx context.Context, pluginMetadata Metadata, query Query, err error) QueryResult {
	overlayIcon := common.NewWoxImageEmoji("🚫")
	pluginIcon := common.ParseWoxImageOrDefault(pluginMetadata.Icon, overlayIcon)
	icon := pluginIcon.OverlayFullPercentage(overlayIcon, 0.6)
	pluginName := pluginMetadata.GetName(ctx)

	return QueryResult{
		Title:    fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_manager_query_failed"), pluginName),
		SubTitle: util.EllipsisEnd(err.Error(), 20),
		Icon:     icon,
		Preview: WoxPreview{
			PreviewType: WoxPreviewTypeText,
			PreviewData: err.Error(),
		},
	}
}

func (m *Manager) getDefaultActions(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult, sourceHash string) (defaultActions []QueryResultAction) {
	return m.getDefaultActionsWithOpenPluginSettingAction(ctx, pluginInstance, query, result, sourceHash, m.newOpenPluginSettingAction(ctx, pluginInstance))
}

func (m *Manager) getDefaultActionsWithOpenPluginSettingAction(ctx context.Context, pluginInstance *Instance, query Query, result QueryResult, sourceHash string, openPluginSettingAction QueryResultAction) (defaultActions []QueryResultAction) {
	// Pins and usage history share one identity. Translate first so an i18n title
	// hashes the same way calculateResultScore does after polish.
	scoreResult := result
	scoreResult.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	scoreResult.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	resultHash := resultIdentityHash(pluginInstance.Metadata.Id, scoreResult)
	settingManager := setting.GetSettingManager()
	// Declare both actions first
	var pinInQueryAction func(context.Context, ActionContext)
	var unpinInQueryAction func(context.Context, ActionContext)

	// Define pin in current query action. Refresh rebuilds scores so only this query moves the row up.
	pinInQueryAction = func(ctx context.Context, actionContext ActionContext) {
		settingManager.PinResult(ctx, resultHash, query.RawQuery)
		api := NewAPI(pluginInstance)
		api.Notify(ctx, "i18n:plugin_manager_pin_in_query_success")
		api.RefreshQuery(ctx, RefreshQueryParam{SelectedResultId: actionContext.ResultId})
	}

	// Define unpin from current query action.
	unpinInQueryAction = func(ctx context.Context, actionContext ActionContext) {
		settingManager.UnpinResult(ctx, resultHash, query.RawQuery)
		api := NewAPI(pluginInstance)
		api.Notify(ctx, "i18n:plugin_manager_unpin_in_query_success")
		api.RefreshQuery(ctx, RefreshQueryParam{SelectedResultId: actionContext.ResultId})
	}

	if settingManager.IsPinedResult(ctx, resultHash, query.RawQuery) {
		defaultActions = append(defaultActions, QueryResultAction{
			Id:                     systemActionUnpinInQueryID,
			Name:                   "i18n:plugin_manager_unpin_in_query",
			Icon:                   icons.Get(icons.ActionUnpin),
			IsSystemAction:         true,
			PreventHideAfterAction: true,
			Action:                 unpinInQueryAction,
		})
	} else {
		defaultActions = append(defaultActions, QueryResultAction{
			Id:                     systemActionPinInQueryID,
			Name:                   "i18n:plugin_manager_pin_in_query",
			Icon:                   icons.Get(icons.ActionPin),
			IsSystemAction:         true,
			PreventHideAfterAction: true,
			Action:                 pinInQueryAction,
		})
	}

	resetRankingAction := func(ctx context.Context, actionContext ActionContext) {
		settingManager.ClearActionedResults(ctx, resultHash)

		api := NewAPI(pluginInstance)
		api.Notify(ctx, "i18n:plugin_manager_reset_ranking_success")
		api.RefreshQuery(ctx, RefreshQueryParam{PreserveSelectedIndex: true})
	}
	actionedScore := m.calculateResultScore(ctx, pluginInstance.Metadata.Id, scoreResult, query)
	defaultActions = append(defaultActions, QueryResultAction{
		Id:                     systemActionResetRankingID,
		Name:                   "i18n:plugin_manager_reset_ranking",
		Icon:                   icons.Get(icons.ActionRevertRanking),
		Tail:                   actionedRankingScoreTail(actionedScore),
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Action:                 resetRankingAction,
	})

	if queryText, ok := newQueryAliasText(query, settingManager.GetWoxSetting(ctx).QueryAliases.Get()); ok {
		defaultActions = append(defaultActions, m.newAddQueryAliasAction(pluginInstance, queryText))
	}

	defaultActions = append(defaultActions, openPluginSettingAction)
	defaultActions = append(defaultActions, m.newResultBindingActions(ctx, pluginInstance, query, result, sourceHash)...)

	return defaultActions
}

// newQueryAliasText returns the query text that can become a new query alias.
// Only typed queries qualify, and a query that already is an alias target is
// skipped because aliasing it again would just duplicate the existing entry.
func newQueryAliasText(query Query, existingAliases []setting.QueryAlias) (string, bool) {
	if query.Type != QueryTypeInput {
		return "", false
	}
	queryText := strings.TrimSpace(query.RawQuery)
	if queryText == "" {
		return "", false
	}
	for _, alias := range existingAliases {
		if alias.Query == queryText {
			return "", false
		}
	}
	return queryText, true
}

// newAddQueryAliasAction saves the current query as a query alias from an inline
// action form. Naming an alias is a single-field edit, so it stays in the launcher
// instead of sending the user to the settings window.
func (m *Manager) newAddQueryAliasAction(pluginInstance *Instance, queryText string) QueryResultAction {
	return QueryResultAction{
		Id:                     systemActionAddQueryAliasID,
		Name:                   "i18n:plugin_manager_add_query_shortcut",
		Icon:                   icons.Get(icons.ActionQueryAlias),
		Type:                   QueryResultActionTypeForm,
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:        queryAliasFormAliasKey,
					Label:      "i18n:plugin_manager_add_query_shortcut_shortcut",
					Tooltip:    "i18n:plugin_manager_add_query_shortcut_shortcut_tooltip",
					Validators: []validator.PluginSettingValidator{{Type: validator.PluginSettingValidatorTypeNotEmpty, Value: &validator.PluginSettingValidatorNotEmpty{}}},
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          queryAliasFormQueryKey,
					Label:        "i18n:plugin_manager_add_query_shortcut_query",
					DefaultValue: queryText,
					Validators:   []validator.PluginSettingValidator{{Type: validator.PluginSettingValidatorTypeNotEmpty, Value: &validator.PluginSettingValidatorNotEmpty{}}},
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext FormActionContext) {
			api := NewAPI(pluginInstance)
			alias := setting.QueryAlias{
				Alias: strings.TrimSpace(actionContext.Values[queryAliasFormAliasKey]),
				Query: strings.TrimSpace(actionContext.Values[queryAliasFormQueryKey]),
			}
			if alias.Alias == "" || alias.Query == "" {
				return
			}

			aliasSetting := setting.GetSettingManager().GetWoxSetting(ctx).QueryAliases
			existing := aliasSetting.Get()
			for _, saved := range existing {
				if strings.EqualFold(saved.Alias, alias.Alias) {
					api.Notify(ctx, "i18n:plugin_manager_add_query_shortcut_duplicated")
					return
				}
			}

			if err := ValidateQueryAliasAgainstResultAliases(alias, currentResultBindings()); err != nil {
				api.Notify(ctx, err.Error())
				return
			}
			if err := aliasSetting.Set(append(append([]setting.QueryAlias(nil), existing...), alias)); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("save query alias: %v", err))
				api.Notify(ctx, "i18n:plugin_manager_result_binding_save_failed")
				return
			}
			// The settings window reads Wox settings once when it opens, so it must be
			// told about an alias added from the launcher.
			m.ui.ReloadSetting(ctx)
			api.Notify(ctx, "i18n:plugin_manager_add_query_shortcut_success")
		},
	}
}

func pluginIdentityIcon(pluginInstance *Instance) common.WoxImage {
	if pluginInstance == nil {
		return common.WoxImage{}
	}
	return pluginInstance.Metadata.GetIconOrDefault(pluginInstance.PluginDirectory, common.WoxImage{})
}

func (m *Manager) newOpenPluginSettingAction(ctx context.Context, pluginInstance *Instance) QueryResultAction {
	return QueryResultAction{
		Id:                     systemActionOpenPluginSettingID,
		Name:                   fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_sys_open_plugin_settings"), pluginInstance.GetName(ctx)),
		SearchAliases:          []string{fmt.Sprintf(i18n.GetI18nManager().TranslateWoxEnUs(ctx, "plugin_sys_open_plugin_settings"), pluginInstance.Metadata.GetNameEn(ctx))},
		Icon:                   icons.Get(icons.ActionSettings),
		TailIcon:               pluginIdentityIcon(pluginInstance),
		IsSystemAction:         true,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext ActionContext) {
			m.ui.OpenSettingWindow(ctx, common.SettingWindowContext{
				Path:  "/plugin/setting",
				Param: pluginInstance.Metadata.Id,
			})
		},
	}
}

func (m *Manager) formatFileListPreview(ctx context.Context, filePaths []string) string {
	totalFiles := len(filePaths)
	if totalFiles == 0 {
		return i18n.GetI18nManager().TranslateWox(ctx, "selection_no_files_selected")
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_selected_files_count"), totalFiles))
	sb.WriteString("\n\n")

	maxDisplayFiles := 10
	for i, filePath := range filePaths {
		if i < maxDisplayFiles {
			sb.WriteString(fmt.Sprintf("- `%s`\n", filePath))
		} else {
			remainingFiles := totalFiles - maxDisplayFiles
			sb.WriteString("\n")
			sb.WriteString(fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_remaining_files_not_shown"), remainingFiles))
			break
		}
	}

	return sb.String()
}

func (m *Manager) buildSelectionFileListPreviewData(ctx context.Context, filePaths []string) string {
	items := make([]WoxPreviewListItem, 0, len(filePaths))
	for _, filePath := range filePaths {
		icon := common.ConvertIcon(ctx, common.NewWoxImageFileIcon(filePath), "")
		extension := strings.TrimPrefix(filepath.Ext(filePath), ".")
		typeLabel := strings.ToUpper(extension)
		if typeLabel == "" {
			typeLabel = "FILE"
		}

		items = append(items, WoxPreviewListItem{
			Icon:     &icon,
			Title:    filepath.Base(filePath),
			Subtitle: filepath.Dir(filePath),
			Tails:    []QueryResultTail{NewQueryResultTailText(typeLabel)},
		})
	}

	// Selection file previews now use the generic list contract. The previous
	// file-path-only payload could not express progress/status rows, so core maps
	// file selections into normal preview rows instead of keeping a file-specific
	// renderer alive.
	previewData, err := json.Marshal(WoxPreviewListData{Items: items})
	if err != nil {
		// Selection previews used to be markdown strings, which forced the UI to
		// render paths as generic text. Fall back to that stable legacy format if
		// JSON encoding ever fails so selection query preview still works.
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to marshal selection list preview data: %s", err.Error()))
		return m.formatFileListPreview(ctx, filePaths)
	}

	return string(previewData)
}

// applyDefaultSelectionPreview keeps the selection preview pane open when a
// plugin returns no preview content. Host SDKs often emit PreviewType=text with
// empty PreviewData, so a type-only check would still collapse the pane.
func (m *Manager) applyDefaultSelectionPreview(ctx context.Context, query Query, preview WoxPreview) WoxPreview {
	if query.Type != QueryTypeSelection || strings.TrimSpace(preview.PreviewData) != "" {
		return preview
	}

	switch query.Selection.Type {
	case selection.SelectionTypeText:
		if strings.TrimSpace(query.Selection.Text) == "" {
			return preview
		}
		return WoxPreview{
			PreviewType: WoxPreviewTypeText,
			PreviewData: query.Selection.Text,
		}
	case selection.SelectionTypeFile:
		if len(query.Selection.FilePaths) == 0 {
			return preview
		}
		return WoxPreview{
			// Selection-file preview is now expressed as generic list rows so
			// the same preview type can serve progress/status workflows too.
			PreviewType: WoxPreviewTypeList,
			PreviewData: m.buildSelectionFileListPreviewData(ctx, query.Selection.FilePaths),
			PreviewTags: []WoxPreviewTag{
				{Label: fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "selection_files_count_value"), len(query.Selection.FilePaths)), Tooltip: "i18n:selection_files_count"},
			},
		}
	default:
		return preview
	}
}

func (m *Manager) normalizeListPreviewData(ctx context.Context, pluginInstance *Instance, preview WoxPreview) WoxPreview {
	if preview.PreviewType != WoxPreviewTypeList || preview.PreviewData == "" {
		return preview
	}

	var data WoxPreviewListData
	if err := json.Unmarshal([]byte(preview.PreviewData), &data); err != nil {
		// Leave malformed payloads untouched so the UI can show its existing
		// preview error. Core only normalizes well-formed list rows.
		return preview
	}

	for itemIndex := range data.Items {
		item := &data.Items[itemIndex]
		item.Title = m.translatePlugin(ctx, pluginInstance, item.Title)
		item.Subtitle = m.translatePlugin(ctx, pluginInstance, item.Subtitle)

		if item.Icon != nil && !item.Icon.IsEmpty() {
			convertedIcon := pluginInstance.ConvertIcon(ctx, *item.Icon)
			item.Icon = &convertedIcon
		}

		for tailIndex := range item.Tails {
			tail := &item.Tails[tailIndex]
			if tail.Type == QueryResultTailTypeText {
				tail.Text = m.translatePlugin(ctx, pluginInstance, tail.Text)
			}
			if tail.Type == QueryResultTailTypeImage {
				tail.Image = pluginInstance.ConvertIcon(ctx, tail.Image)
			}
		}
	}

	// Re-encode after translation and icon normalization so preview rows render
	// with the same i18n and image behavior as top-level result rows.
	normalizedData, err := json.Marshal(data)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to marshal normalized list preview data: %s", err.Error()))
		return preview
	}

	preview.PreviewData = string(normalizedData)
	return preview
}

func (m *Manager) normalizePreviewMetadata(ctx context.Context, pluginInstance *Instance, preview WoxPreview) WoxPreview {
	// PreviewTags are the UI-facing metadata contract. Translate them in core so
	// UI only consumes one tag list and does not need to know about the
	// deprecated key/value PreviewProperties compatibility shape.
	for i := range preview.PreviewTags {
		preview.PreviewTags[i].Label = m.translatePlugin(ctx, pluginInstance, preview.PreviewTags[i].Label)
		if preview.PreviewTags[i].Tooltip != "" {
			preview.PreviewTags[i].Tooltip = m.translatePlugin(ctx, pluginInstance, preview.PreviewTags[i].Tooltip)
		}
	}

	if len(preview.PreviewProperties) == 0 {
		return preview
	}

	translatedProperties := make(map[string]string, len(preview.PreviewProperties))
	for key, value := range preview.PreviewProperties {
		// Legacy properties only supported i18n on the key. Keep values as final
		// display labels so older plugins retain their existing text semantics.
		translatedProperties[m.translatePlugin(ctx, pluginInstance, key)] = value
	}
	preview.PreviewProperties = translatedProperties

	if len(preview.PreviewTags) > 0 {
		return preview
	}

	preview.PreviewTags = make([]WoxPreviewTag, 0, len(preview.PreviewProperties))
	for key, value := range preview.PreviewProperties {
		preview.PreviewTags = append(preview.PreviewTags, WoxPreviewTag{
			Label:   value,
			Tooltip: key,
		})
	}
	return preview
}

// shouldHidePreviewForGlobalQuery keeps global search compact while preserving core-owned unblocker previews.
func shouldHidePreviewForGlobalQuery(query Query, preview WoxPreview) bool {
	if !query.IsGlobalQuery() {
		return false
	}

	return preview.PreviewType != WoxPreviewTypeQueryRequirementSettings &&
		preview.PreviewType != WoxPreviewTypeTriggerKeywordConflict
}

// shouldClearGroupForGlobalQuery keeps global search as a flat ranked list.
// File search is the exception: its many hits stay in one group below others.
func shouldClearGroupForGlobalQuery(query Query, pluginInstance *Instance) bool {
	if !query.IsGlobalQuery() {
		return false
	}
	if pluginInstance != nil && pluginInstance.Metadata.Id == fileSearchPluginID {
		return false
	}
	return true
}

// limitGlobalQueryPluginScore keeps plugin-provided global scores within Wox's shared ranking scale.
func limitGlobalQueryPluginScore(query Query, score int64) int64 {
	if !query.IsGlobalQuery() || score <= globalQueryPluginScoreLimit {
		return score
	}

	return globalQueryPluginScoreLimit
}

func resultIdentityHash(pluginId string, result QueryResult) setting.ResultHash {
	return setting.NewResultHashFromParts(pluginId, result.Title, result.SubTitle, result.IdentityKey)
}

func (m *Manager) calculateResultScore(ctx context.Context, pluginId string, result QueryResult, query Query) int64 {
	var score int64 = 0

	resultHash := resultIdentityHash(pluginId, result)
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	actionResults, ok := woxSetting.ActionedResults.Get().Load(resultHash)
	if !ok {
		return score
	}

	return calculateActionedResultScore(actionResults, query.RawQuery, query.IsGlobalQuery())
}

// calculateActionedResultScore limits global usage boosts to related searches.
func calculateActionedResultScore(actionResults []setting.ActionedResult, currentQuery string, global bool) int64 {
	var score int64
	currentQuery = strings.ToLower(strings.TrimSpace(currentQuery))
	// actioned score are based on actioned counts, the more actioned, the more score
	// also, action timestamp will be considered, the more recent actioned, the more score weight. If action is in recent 7 days, it will be considered as recent actioned and add score weight
	// we will use fibonacci sequence to calculate score, the more recent actioned, the more score: 5, 8, 13, 21, 34, 55, 89
	// that means, actions in day one, we will add weight 89, day two, we will add weight 55, day three, we will add weight 34, and so on
	// E.g. if actioned 3 times in day one, 2 times in day two, 1 time in day three, the score will be: 89*3 + 55*2 + 34*1 = 450

	for _, actionResult := range actionResults {
		historicalQuery := strings.ToLower(strings.TrimSpace(actionResult.Query))
		if global && currentQuery != "" && (historicalQuery == "" || !strings.Contains(historicalQuery, currentQuery) && !strings.Contains(currentQuery, historicalQuery)) {
			continue
		}
		var weight int64 = 2

		hours := (util.GetSystemTimestamp() - actionResult.Timestamp) / 1000 / 60 / 60
		if hours < 24*7 {
			fibonacciIndex := int(math.Ceil(float64(hours) / 24))
			if fibonacciIndex > 7 {
				fibonacciIndex = 7
			}
			if fibonacciIndex < 1 {
				fibonacciIndex = 1
			}
			fibonacci := []int64{5, 8, 13, 21, 34, 55, 89}
			score += fibonacci[7-fibonacciIndex]
		}

		// If the current query is within the historical selected actions, it indicates a stronger connection and increases the score.
		if currentQuery != "" && historicalQuery == currentQuery {
			score += 20
		}

		score += weight
	}

	return score
}

// actionedRankingScoreTail formats the usage-history boost that Reset ranking clears.
func actionedRankingScoreTail(score int64) string {
	if score <= 0 {
		return ""
	}
	return fmt.Sprintf("+%d", score)
}

// ReleasePreparedSearchText drops precomputed app, command, and plugin-name match text.
// Plugins rebuild it on the next query.
func (m *Manager) ReleasePreparedSearchText() {
	for _, instance := range m.GetPluginInstances() {
		if instance == nil || instance.Plugin == nil {
			continue
		}
		releaser, ok := instance.Plugin.(PreparedSearchReleaser)
		if !ok {
			continue
		}
		releaser.ReleasePreparedSearchText()
	}
}

func (m *Manager) getCachedLayoutForPluginQuery(ctx context.Context, pluginInstance *Instance, query Query) QueryLayout {
	if pluginInstance == nil {
		return QueryLayout{}
	}

	set, ok := m.resultStore.getQuery(query.SessionId, query.Id)
	if !ok {
		return m.buildMetadataBackedQueryLayout(ctx, pluginInstance, query)
	}

	var cachedLayout QueryLayout
	set.Results.Range(func(_ string, resultCache *QueryResultCache) bool {
		if resultCache == nil || resultCache.PluginInstance == nil {
			return true
		}
		if resultCache.PluginInstance.Metadata.Id != pluginInstance.Metadata.Id {
			return true
		}
		cachedLayout = resultCache.Layout
		return false
	})
	if cachedLayout.GridLayout != nil || cachedLayout.Icon != nil || cachedLayout.ResultPreviewWidthRatio != nil {
		return cachedLayout
	}

	// PushResults and late updates do not carry a new layout payload. Reusing the
	// cached QueryResponse layout keeps dynamically declared grid results at the
	// same icon size; metadata remains only as the legacy fallback when nothing
	// from the current query has been cached yet.
	return m.buildMetadataBackedQueryLayout(ctx, pluginInstance, query)
}

// NotifyDragOut delivers a terminal result-file-drag status to the owning plugin.
// The drag itself already finished; callbacks cannot block or cancel it.
func (m *Manager) NotifyDragOut(ctx context.Context, pluginId string, event DragOutEvent) {
	if m == nil || pluginId == "" {
		return
	}
	instance := m.getPluginInstance(pluginId)
	if instance == nil {
		return
	}
	callbacks := slices.Clone(instance.DragOutCallbacks)
	if len(callbacks) == 0 {
		return
	}
	event.Files = append([]string(nil), event.Files...)
	for _, callback := range callbacks {
		callback := callback
		util.Go(ctx, fmt.Sprintf("[%s] drag out callback", instance.GetName(ctx)), func() {
			callback(ctx, event)
		})
	}
}

// shouldWrapRemotePreview keeps previews that need their concrete type on the initial UI response inline.
func shouldWrapRemotePreview(preview WoxPreview) bool {
	if preview.IsEmpty() || len(preview.PreviewData) <= previewDataMaxSize {
		return false
	}
	switch preview.PreviewType {
	case WoxPreviewTypeRemote, WoxPreviewTypeQueryRequirementSettings, WoxPreviewTypeTriggerKeywordConflict, WoxPreviewTypeMedia, WoxPreviewTypeChat, WoxPreviewTypeTerminal:
		return false
	default:
		return true
	}
}

func (m *Manager) buildResultUI(resultCache *QueryResultCache, queryId string) QueryResultUI {
	uiResult := resultCache.Result
	// Core-owned interactive previews must keep their concrete preview type in the
	// result payload. If they were wrapped as remote previews, UI could not
	// choose the dedicated fullscreen/editing surface before loading the preview.
	if shouldWrapRemotePreview(uiResult.Preview) {
		uiResult.Preview = WoxPreview{
			PreviewType:   WoxPreviewTypeRemote,
			PreviewData:   fmt.Sprintf("/preview?sessionId=%s&queryId=%s&id=%s", resultCache.Query.SessionId, queryId, uiResult.Id),
			DefaultHidden: uiResult.Preview.DefaultHidden,
		}
	}
	resultUI := uiResult.ToUI()
	resultUI.QueryId = queryId
	return resultUI
}

// Equal scores must still produce a deterministic order because the result cache is backed by a map.
func compareQueryResultCachesForDisplay(a *QueryResultCache, b *QueryResultCache) int {
	if a.AliasMatchKind != b.AliasMatchKind {
		if a.AliasMatchKind > b.AliasMatchKind {
			return -1
		}
		return 1
	}
	// Pin is its own tier. Plugin scores, such as clipboard copy times, stay inside
	// that tier and only order pins against other pins.
	if aPinned, bPinned := queryResultPinned(a.Result), queryResultPinned(b.Result); aPinned != bPinned {
		if aPinned {
			return -1
		}
		return 1
	}
	switch {
	case a.Result.Score > b.Result.Score:
		return -1
	case a.Result.Score < b.Result.Score:
		return 1
	}

	if len(a.Result.Title) != len(b.Result.Title) {
		if len(a.Result.Title) < len(b.Result.Title) {
			return -1
		}
		return 1
	}
	if diff := strings.Compare(a.Result.Title, b.Result.Title); diff != 0 {
		return diff
	}
	if len(a.Result.SubTitle) != len(b.Result.SubTitle) {
		if len(a.Result.SubTitle) < len(b.Result.SubTitle) {
			return -1
		}
		return 1
	}
	if diff := strings.Compare(a.Result.SubTitle, b.Result.SubTitle); diff != 0 {
		return diff
	}

	pluginIdA := ""
	if a.PluginInstance != nil {
		pluginIdA = a.PluginInstance.Metadata.Id
	}
	pluginIdB := ""
	if b.PluginInstance != nil {
		pluginIdB = b.PluginInstance.Metadata.Id
	}
	if diff := strings.Compare(pluginIdA, pluginIdB); diff != 0 {
		return diff
	}

	return strings.Compare(a.Result.Id, b.Result.Id)
}

// BuildQueryResultsSnapshot builds a snapshot of all cached query results for the given session and query id.
// Query pins come first. Remaining results are grouped by name, and both groups and results within groups are sorted by score.
func (m *Manager) BuildQueryResultsSnapshot(sessionId string, queryId string) []QueryResultUI {
	return m.buildQueryResultsSnapshot(sessionId, queryId, nil)
}

// BuildQueryResultsSnapshotForResultIds builds a query snapshot limited to results accepted by the query runner.
func (m *Manager) BuildQueryResultsSnapshotForResultIds(sessionId string, queryId string, resultIds []string) []QueryResultUI {
	if len(resultIds) == 0 {
		return []QueryResultUI{}
	}

	includedResultIds := make(map[string]struct{}, len(resultIds))
	for _, resultId := range resultIds {
		if resultId == "" {
			continue
		}
		includedResultIds[resultId] = struct{}{}
	}
	if len(includedResultIds) == 0 {
		return []QueryResultUI{}
	}

	return m.buildQueryResultsSnapshot(sessionId, queryId, func(resultId string) bool {
		_, ok := includedResultIds[resultId]
		return ok
	})
}

func (m *Manager) buildQueryResultsSnapshot(sessionId string, queryId string, shouldIncludeResult func(string) bool) []QueryResultUI {
	if queryId == "" {
		return []QueryResultUI{}
	}
	// Bug fix: snapshot lookup must use the query id, not the session's latest
	// query. Multiple backend query pipelines can finish out of order, while
	// UI filters visibility by QueryId after the response is delivered.
	set, found := m.resultStore.getQuery(sessionId, queryId)
	if !found {
		return []QueryResultUI{}
	}

	var resultCaches []*QueryResultCache
	set.Results.Range(func(resultId string, resultCache *QueryResultCache) bool {
		if shouldIncludeResult != nil && !shouldIncludeResult(resultId) {
			return true
		}
		resultCaches = append(resultCaches, resultCache)
		return true
	})

	if len(resultCaches) == 0 {
		return []QueryResultUI{}
	}

	var aliasResults []*QueryResultCache
	var ordinaryResults []*QueryResultCache
	for _, resultCache := range resultCaches {
		if resultCache.AliasMatchKind != aliasMatchNone {
			aliasResults = append(aliasResults, resultCache)
			continue
		}
		ordinaryResults = append(ordinaryResults, resultCache)
	}
	sort.SliceStable(aliasResults, func(i, j int) bool {
		return compareQueryResultCachesForDisplay(aliasResults[i], aliasResults[j]) < 0
	})
	aliasResults = dedupeResultBindingCaches(aliasResults)
	ordinaryResults = dropResultCachesDuplicateOfAlias(aliasResults, ordinaryResults)

	resultCaches = ordinaryResults
	if len(resultCaches) == 0 && len(aliasResults) > 0 {
		finalResults := make([]QueryResultUI, 0, len(aliasResults))
		for _, resultCache := range aliasResults {
			finalResults = append(finalResults, m.buildResultUI(resultCache, queryId))
		}
		return finalResults
	}

	// Pins leave their group and stay above every group header. Otherwise a pinned
	// clipboard row remains inside Today, where a later copy's timestamp still
	// sorts first, and an older day group would keep the pin below Today.
	groupScores := map[string]int64{}
	var pinnedResults []*QueryResultCache
	groupedResults := make(map[string][]*QueryResultCache)
	for _, resultCache := range resultCaches {
		if queryResultPinned(resultCache.Result) {
			pinnedResults = append(pinnedResults, resultCache)
			continue
		}
		result := resultCache.Result
		if score, ok := groupScores[result.Group]; !ok || result.GroupScore > score {
			groupScores[result.Group] = result.GroupScore
		}
		groupedResults[result.Group] = append(groupedResults[result.Group], resultCache)
	}
	// Every pin is already in the same tier, so this keeps their original score order.
	sort.SliceStable(pinnedResults, func(i, j int) bool {
		return compareQueryResultCachesForDisplay(pinnedResults[i], pinnedResults[j]) < 0
	})

	var groups []string
	for group := range groupScores {
		groups = append(groups, group)
	}
	sort.SliceStable(groups, func(i, j int) bool {
		scoreA := groupScores[groups[i]]
		scoreB := groupScores[groups[j]]
		if scoreA == scoreB {
			return groups[i] < groups[j]
		}
		return scoreA > scoreB
	})

	for group := range groupedResults {
		groupResults := groupedResults[group]
		sort.Slice(groupResults, func(i, j int) bool {
			return compareQueryResultCachesForDisplay(groupResults[i], groupResults[j]) < 0
		})
		groupedResults[group] = groupResults
	}

	finalResults := make([]QueryResultUI, 0, len(aliasResults)+len(resultCaches)+len(groups))
	for _, resultCache := range aliasResults {
		finalResults = append(finalResults, m.buildResultUI(resultCache, queryId))
	}
	for _, resultCache := range pinnedResults {
		finalResults = append(finalResults, m.buildResultUI(resultCache, queryId))
	}
	for _, group := range groups {
		groupResults := groupedResults[group]
		if len(groupResults) == 0 {
			continue
		}
		if group != "" {
			finalResults = append(finalResults, QueryResultUI{
				QueryId:    queryId,
				Id:         fmt.Sprintf("group:%s:%s", queryId, group),
				Title:      group,
				SubTitle:   "",
				Icon:       common.WoxImage{},
				Preview:    WoxPreview{},
				Score:      groupScores[group],
				Group:      group,
				GroupScore: groupScores[group],
				Tails:      []QueryResultTail{},
				Actions:    []QueryResultActionUI{},
				IsGroup:    true,
			})
		}
		for _, resultCache := range groupResults {
			finalResults = append(finalResults, m.buildResultUI(resultCache, queryId))
		}
	}

	return finalResults
}

func (m *Manager) serializeContextData(contextData map[string]string) string {
	if len(contextData) == 0 {
		return ""
	}
	data, err := json.Marshal(contextData)
	if err != nil {
		return ""
	}
	return string(data)
}

func (m *Manager) Query(ctx context.Context, query Query) QueryExecution {
	queryStart := util.GetSystemTimestamp()
	if m.autoQueryHistory != nil {
		m.autoQueryHistory.beginQuery(query)
	}
	if tracker := timetracking.New("manager_query_enter"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("query", query.String())
		tracker.Log(ctx)
	}

	cacheStart := util.GetSystemTimestamp()
	m.resultStore.startQuery(query)
	ctx = m.resultStore.withQueryResultLifetime(ctx, query.SessionId, query.Id)
	if tracker := timetracking.New("start_session_query_cache"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-cacheStart)
		tracker.Log(ctx)
	}

	jobs, immediateCount, debouncedCount := m.buildQueryPlan(ctx, query)
	firstFlushDeadlineMs := m.queryScheduler.getQueryFirstFlushDeadlineMs(jobs, util.GetSystemTimestamp()-queryStart)
	execution := m.queryScheduler.start(ctx, query, jobs, firstFlushDeadlineMs, queryExecutionCallbacks{
		queryPlugin:    m.queryForPlugin,
		restoreAliases: m.restoreResultAliases,
		onPluginResponse: func(ctx context.Context, query Query, response QueryResponse) {
			if m.autoQueryHistory != nil {
				m.autoQueryHistory.schedule(ctx, query, response)
			}
		},
	})
	if tracker := timetracking.New("manager_query_exit"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("query", query.String())
		tracker.SetInt("scheduled", len(jobs))
		tracker.SetInt("immediate", immediateCount)
		tracker.SetInt("debounced", debouncedCount)
		tracker.SetInt64("firstFlushDeadlineMs", firstFlushDeadlineMs)
		tracker.SetInt64("costMs", util.GetSystemTimestamp()-queryStart)
		tracker.Log(ctx)
	}

	return execution
}

// buildQueryPlan resolves eligibility once so every runnable plugin starts from the same scheduling boundary.
func (m *Manager) buildQueryPlan(ctx context.Context, query Query) (jobs []queryPluginJob, immediateCount int, debouncedCount int) {
	planStart := time.Now()
	maxCheckUs := int64(0)
	maxCheckPlugin := ""

	appendJob := func(pluginInstance *Instance) {
		checkStart := time.Now()
		canOperate := m.canOperateQuery(ctx, pluginInstance, query)
		checkUs := time.Since(checkStart).Microseconds()
		if checkUs > maxCheckUs {
			maxCheckUs = checkUs
			maxCheckPlugin = queryDiagnosticPluginLabel(pluginInstance)
		}
		if !canOperate {
			return
		}

		supportsDebounce := pluginInstance.Metadata.IsSupportFeature(MetadataFeatureDebounce)
		job := queryPluginJob{pluginInstance: pluginInstance}
		if supportsDebounce {
			debounceParams, err := pluginInstance.Metadata.GetFeatureParamsForDebounce()
			if err != nil {
				logger.Error(ctx, fmt.Sprintf("[%s] %s, query directly", pluginInstance.GetName(ctx), err))
			} else {
				job.debounced = debounceParams.IntervalMs > 0
				job.intervalMs = debounceParams.IntervalMs
			}
		}

		if interval, overridden := pluginInstance.queryDebounceOverride(m.applyScopeForPlugin(query, pluginInstance)); overridden {
			job.debounced = interval > 0
			job.intervalMs = interval
		}
		job.blocksFallback = !job.debounced

		if job.debounced {
			debouncedCount++
		} else {
			immediateCount++
		}
		jobs = append(jobs, job)
	}

	checkedCount := 0
	if query.HasScope() {
		deduped := query.Scope.Deduplicate()
		jobs = make([]queryPluginJob, 0, len(deduped.Plugins))
		checkedCount = len(deduped.Plugins)
		for _, item := range deduped.Plugins {
			pluginInstance := m.getPluginInstance(item.PluginID)
			if pluginInstance == nil {
				continue
			}
			appendJob(pluginInstance)
		}
	} else {
		instances := m.pluginInstancesSnapshot()
		jobs = make([]queryPluginJob, 0, len(instances)+1)
		checkedCount = len(instances)
		for _, pluginInstance := range instances {
			appendJob(pluginInstance)
		}
	}

	if query.IsGlobalQuery() && strings.TrimSpace(query.Search) != "" && m.hasResultAliases() {
		jobs = append([]queryPluginJob{{aliasRestore: true}}, jobs...)
		immediateCount++
	}

	if tracker := timetracking.New("query_plan"); tracker.Enabled() {
		tracker.SetRawString("queryId", query.Id)
		tracker.SetRawString("query", query.String())
		tracker.SetInt("checked", checkedCount)
		tracker.SetInt("scheduled", len(jobs))
		tracker.SetInt("immediate", immediateCount)
		tracker.SetInt("debounced", debouncedCount)
		tracker.SetInt64("maxCheckUs", maxCheckUs)
		tracker.SetRawString("maxCheckPlugin", maxCheckPlugin)
		tracker.SetInt64("totalUs", time.Since(planStart).Microseconds())
		tracker.Log(ctx)
	}

	return jobs, immediateCount, debouncedCount
}

func (m *Manager) QuerySilent(ctx context.Context, query Query) bool {
	// Silent hotkeys own temporary sessions without a window to release their callbacks.
	defer m.resultStore.clearSession(query.SessionId)
	var startTimestamp = util.GetSystemTimestamp()
	var results []QueryResultUI
	execution := m.Query(ctx, query)
	for {
		select {
		case r := <-execution.Results:
			results = append(results, r.Results...)
		case <-execution.Done:
			logger.Info(ctx, fmt.Sprintf("silent query done, total results: %d, cost %d ms", len(results), util.GetSystemTimestamp()-startTimestamp))

			// execute default action if only one result
			woxIcon, _ := icons.Get(icons.BrandWox).ToImage()
			if len(results) == 1 {
				result := results[0]
				for _, action := range result.Actions {
					if action.IsDefault {
						actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, query.SessionId), query.Id)
						executeErr := m.ExecuteAction(actionCtx, query.SessionId, query.Id, result.Id, action.Id)
						if executeErr != nil {
							logger.Error(ctx, fmt.Sprintf("silent query execute failed: %s", executeErr.Error()))
							notifier.Notify(woxIcon, fmt.Sprintf("Silent query execute failed: %s", executeErr.Error()))
							return false
						}

						return true
					}
				}
			} else {
				notifier.Notify(woxIcon, fmt.Sprintf("Silent query failed, there shouldbe only one result, but got %d", len(results)))
			}

			return false
		case <-time.After(time.Minute):
			logger.Error(ctx, "silent query timeout")
			return false
		}
	}
}

func (m *Manager) QueryFallback(ctx context.Context, query Query, queryPlugin *Instance) (response QueryResponseUI) {
	response.Context = BuildQueryContext(query, queryPlugin)
	if queryPlugin != nil {
		// Fallback command rows are still part of the same plugin query surface.
		// Attach metadata-backed layout here so early fallback does not erase
		// the QueryResponse layout sent before the plugin result finishes.
		response.Layout = m.buildMetadataBackedQueryLayout(ctx, queryPlugin, query)
	}

	var queryResults []QueryResult
	if query.IsGlobalQuery() {
		for _, pluginInstance := range m.pluginInstancesSnapshot() {
			// Disabled plugins stay registered so they can be enabled later, but Init is
			// skipped. Their API is nil, and QueryFallback would panic and discard every
			// other fallback result for this query.
			if pluginInstanceDisabled(pluginInstance) {
				continue
			}
			if v, ok := pluginInstance.Plugin.(FallbackSearcher); ok {
				fallbackResults := v.QueryFallback(ctx, query)
				for _, fallbackResult := range fallbackResults {
					fallbackResult.isFallback = true
					polishedFallbackResult := m.PolishResult(ctx, pluginInstance, query, QueryLayout{}, fallbackResult)
					queryResults = append(queryResults, polishedFallbackResult)
				}
				continue
			}
		}
	} else {
		if query.Command != "" {
			return response
		}
		if queryPlugin == nil {
			return response
		}

		// search query commands
		commands := lo.Filter(queryPlugin.GetQueryCommands(), func(item MetadataCommand, index int) bool {
			return strings.Contains(item.Command, query.Search) || query.Search == ""
		})
		queryResults = lo.Map(commands, func(item MetadataCommand, index int) QueryResult {
			return QueryResult{
				Title:    item.Command,
				SubTitle: string(item.Description),
				Icon:     common.ParseWoxImageOrDefault(queryPlugin.Metadata.Icon, common.NewWoxImageEmoji("🔍")),
				Actions: []QueryResultAction{
					{
						Name:                   "Execute",
						PreventHideAfterAction: true,
						Action: func(ctx context.Context, actionContext ActionContext) {
							m.ui.ChangeQuery(ctx, common.PlainQuery{
								QueryType: QueryTypeInput,
								QueryText: fmt.Sprintf("%s %s ", query.TriggerKeyword, item.Command),
							})
						},
					},
				},
			}
		})
		for i := range queryResults {
			queryResults[i] = m.PolishResult(ctx, queryPlugin, query, response.Layout, queryResults[i])
		}
	}

	queryResultsUI := lo.Map(queryResults, func(item QueryResult, index int) QueryResultUI {
		return item.ToUI()
	})
	response.Results = append(response.Results, queryResultsUI...)
	return response
}

func queryDiagnosticPluginLabel(pluginInstance *Instance) string {
	if pluginInstance == nil {
		return "<nil>"
	}
	name := string(pluginInstance.Metadata.Name)
	if name == "" {
		name = pluginInstance.Metadata.Id
	}
	return fmt.Sprintf("%s(%s)", name, pluginInstance.Metadata.Id)
}

func (m *Manager) translatePlugin(ctx context.Context, pluginInstance *Instance, key string) string {
	if !strings.HasPrefix(key, "i18n:") {
		return key
	}

	return pluginInstance.Metadata.translate(ctx, common.I18nString(key))
}

func (m *Manager) GetUI() common.UI {
	return m.ui
}

// AttachUI sets the UI used by plugin APIs without loading plugins or hosts.
func (m *Manager) AttachUI(ui common.UI) {
	m.ui = ui
}

// WaitPluginInit waits until the named plugin finishes its current init cycle.
func (m *Manager) WaitPluginInit(ctx context.Context, pluginID string) error {
	instance := m.GetPluginInstanceById(pluginID)
	if instance == nil {
		return fmt.Errorf("plugin %s is not loaded", pluginID)
	}
	if err := instance.WaitInit(ctx); err != nil {
		return err
	}
	initialized, initErr := instance.initStatus()
	if initialized {
		return nil
	}
	if initErr != nil {
		return initErr
	}
	return fmt.Errorf("plugin %s did not initialize", pluginID)
}

func (m *Manager) NewQuery(ctx context.Context, plainQuery common.PlainQuery) (Query, *Instance, error) {

	plainQuery.QueryHint = plainQuery.QueryHint.NormalizeForQuery(plainQuery.QueryType, plainQuery.QueryText)
	refinements := plainQuery.QueryRefinements
	if refinements == nil {
		// Query refinements are optional in older UI requests. Normalize nil to
		// an empty map so external hosts always receive an object, not JSON null.
		refinements = map[string]string{}
	}
	contextData := plainQuery.ContextData
	if contextData == nil {
		contextData = common.ContextData{}
	}
	scope := plainQuery.QueryScope.NormalizeForRouting()

	attachEnv := func(query *Query) {
		query.QueryHint = plainQuery.QueryHint.Clone()
		activeWindowSnapshot := m.GetUI().GetActiveWindowSnapshot(ctx)
		query.QueryVariables = activeWindowSnapshot.QueryVariables
		query.Env.ActiveWindowTitle = activeWindowSnapshot.Name
		query.Env.ActiveWindowPid = activeWindowSnapshot.Pid
		query.Env.ActiveWindowId = activeWindowSnapshot.WindowId
		query.Env.ActiveWindowIcon = activeWindowSnapshot.Icon
		query.Env.ActiveWindowIsOpenSaveDialog = activeWindowSnapshot.IsOpenSaveDialog
		query.Env.ActiveWindowIsOpenSaveDialogSelectFolder = activeWindowSnapshot.IsOpenSaveDialogSelectFolder
		query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
	}

	resolveScopedOwner := func(query Query) *Instance {
		owner := ScopeOwnerPlugin(query, m.getPluginInstance)
		if owner == nil || !m.canOperateQuery(ctx, owner, query) {
			return nil
		}
		return owner
	}

	if !scope.IsEmpty() {
		query := Query{
			Id:          plainQuery.QueryId,
			SessionId:   util.GetContextSessionId(ctx),
			Type:        plainQuery.QueryType,
			RawQuery:    plainQuery.QueryText,
			Search:      plainQuery.QueryText,
			Selection:   plainQuery.QuerySelection,
			Refinements: refinements,
			ContextData: contextData,
			Scope:       scope,
		}
		if query.Type == "" {
			query.Type = QueryTypeInput
		}
		attachEnv(&query)
		owner := resolveScopedOwner(query)
		return query, owner, nil
	}

	if plainQuery.QueryType == QueryTypeInput {
		newQuery := plainQuery.QueryText
		woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
		if len(woxSetting.QueryAliases.Get()) > 0 {
			originQuery := plainQuery.QueryText
			expandedQuery := m.expandQueryAlias(ctx, plainQuery.QueryText, woxSetting.QueryAliases.Get())
			if originQuery != expandedQuery {
				logger.Info(ctx, fmt.Sprintf("expand query alias: %s -> %s", originQuery, expandedQuery))
				newQuery = expandedQuery
			}
		}
		query, instance := newQueryInputWithPlugins(newQuery, GetPluginManager().GetPluginInstances())
		query.Id = plainQuery.QueryId
		query.SessionId = util.GetContextSessionId(ctx)
		query.Refinements = refinements
		query.ContextData = contextData
		if conflictErr := m.newTriggerKeywordConflictErrorIfNeeded(ctx, query); conflictErr != nil {
			return query, nil, conflictErr
		}
		attachEnv(&query)
		return query, instance, nil
	}

	if plainQuery.QueryType == QueryTypeSelection {
		// selection query also supports query text for plugins to parse trigger keyword and command
		parsed, instance := newQueryInputWithPlugins(plainQuery.QueryText, GetPluginManager().GetPluginInstances())

		query := Query{
			Id:             plainQuery.QueryId,
			Type:           QueryTypeSelection,   // override: this is a selection query, not input
			RawQuery:       plainQuery.QueryText, // keep the original unmodified text
			TriggerKeyword: parsed.TriggerKeyword,
			Command:        parsed.Command,
			Search:         parsed.Search,
			Selection:      plainQuery.QuerySelection,
			Refinements:    refinements,
			ContextData:    contextData,
		}
		query.SessionId = util.GetContextSessionId(ctx)
		if conflictErr := m.newTriggerKeywordConflictErrorIfNeeded(ctx, query); conflictErr != nil {
			return query, nil, conflictErr
		}
		attachEnv(&query)
		return query, instance, nil
	}

	return Query{}, nil, errors.New("invalid query type")
}

func (m *Manager) getActiveBrowserUrl(ctx context.Context) string {
	activeWindowSnapshot := m.GetUI().GetActiveWindowSnapshot(ctx)
	if !browser.IsBrowserWindowName(activeWindowSnapshot.Name) {
		return ""
	}

	return m.activeBrowserUrl
}

func (m *Manager) getActiveFileExplorerPath(ctx context.Context) string {
	// Supported on Windows (File Explorer) and macOS (Finder)
	if runtime.GOOS != "windows" && runtime.GOOS != "darwin" {
		return ""
	}

	// Use platform-specific implementation via util/window
	return window.GetActiveFileExplorerPath()
}

func (m *Manager) expandQueryAlias(ctx context.Context, query string, queryAliases []setting.QueryAlias) (newQuery string) {
	newQuery = query

	// Expand the longest alias first so "wix" wins over "wi".
	slices.SortFunc(queryAliases, func(i, j setting.QueryAlias) int {
		return len(j.Alias) - len(i.Alias)
	})

	for _, alias := range queryAliases {
		if alias.Disabled {
			continue
		}

		// Query aliases rewrite only the first token. Plain prefix matching made
		// short aliases such as "th" rewrite normal queries like "theme xx", so
		// the alias must end at the query boundary while still supporting "th args".
		if query == alias.Alias || strings.HasPrefix(query, alias.Alias+" ") {
			if !alias.HasPlaceholder() {
				newQuery = strings.Replace(query, alias.Alias, alias.Query, 1)
				break
			} else {
				queryWithoutAlias := strings.Replace(query, alias.Alias, "", 1)
				queryWithoutAlias = strings.TrimLeft(queryWithoutAlias, " ")
				parameters := strings.Split(queryWithoutAlias, " ")
				placeholderCount := alias.PlaceholderCount()
				var paramsCount = 0

				var params []any
				var nonPrams string
				for _, param := range parameters {
					if paramsCount < placeholderCount {
						paramsCount++
						params = append(params, param)
					} else {
						nonPrams += " " + param
					}
				}
				newQuery = stringFormatter.Format(alias.Query, params...) + nonPrams
				break
			}
		}
	}

	return newQuery
}

func (m *Manager) ExecuteAction(ctx context.Context, sessionId string, queryId string, resultId string, actionId string) error {
	resultCache, found := m.resultStore.findInSession(sessionId, queryId, resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (execute action): %s", resultId)
	}

	if network.IsOffline() && resultCache.PluginInstance.Host != nil {
		return network.ErrOffline
	}
	// Find the action in cache
	var actionCache *QueryResultAction
	for i := range resultCache.Result.Actions {
		if resultCache.Result.Actions[i].Id == actionId {
			actionCache = &resultCache.Result.Actions[i]
			break
		}
	}
	if actionCache == nil {
		return fmt.Errorf("action not found for result id: %s, action id: %s", resultId, actionId)
	}

	// Check if action callback is nil
	if actionCache.Action == nil {
		return fmt.Errorf("action callback is nil for result id: %s, action id: %s", resultId, actionId)
	}

	meta := resultCache.PluginInstance.Metadata
	analytics.TrackActionExecuted(ctx, meta.Id, resultCache.PluginInstance.GetName(ctx))

	actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, resultCache.Query.SessionId), resultCache.Query.Id)
	actionCache.Action(actionCtx, ActionContext{
		ResultId:       resultId,
		ResultActionId: actionId,
		ContextData:    actionCache.ContextData,
	})

	util.Go(actionCtx, fmt.Sprintf("[%s] post execute action", resultCache.PluginInstance.GetName(actionCtx)), func() {
		m.postExecuteAction(actionCtx, resultCache, actionId, actionCache.ContextData)
	})

	return nil
}

func (m *Manager) SubmitFormAction(ctx context.Context, sessionId string, queryId string, resultId string, actionId string, values map[string]string) error {
	resultCache, found := m.resultStore.findInSession(sessionId, queryId, resultId)
	if !found {
		return fmt.Errorf("result cache not found for result id (submit form action): %s", resultId)
	}

	if network.IsOffline() && resultCache.PluginInstance.Host != nil {
		return network.ErrOffline
	}

	var actionCache *QueryResultAction
	for i := range resultCache.Result.Actions {
		if resultCache.Result.Actions[i].Id == actionId && resultCache.Result.Actions[i].Type == QueryResultActionTypeForm {
			actionCache = &resultCache.Result.Actions[i]
			break
		}
	}
	if actionCache == nil {
		return fmt.Errorf("form action not found for result id: %s, action id: %s", resultId, actionId)
	}

	if actionCache.OnSubmit == nil {
		return fmt.Errorf("form action callback is nil for result id: %s, action id: %s", resultId, actionId)
	}

	meta := resultCache.PluginInstance.Metadata
	analytics.TrackActionExecuted(ctx, meta.Id, resultCache.PluginInstance.GetName(ctx))

	actionCtx := util.WithQueryIdContext(util.WithSessionContext(ctx, resultCache.Query.SessionId), resultCache.Query.Id)
	actionCache.OnSubmit(actionCtx, FormActionContext{
		ActionContext: ActionContext{
			ResultId:       resultId,
			ResultActionId: actionId,
			ContextData:    actionCache.ContextData,
		},
		Values: values,
	})

	util.Go(actionCtx, fmt.Sprintf("[%s] post execute action", resultCache.PluginInstance.GetName(actionCtx)), func() {
		m.postExecuteAction(actionCtx, resultCache, actionId, actionCache.ContextData)
	})

	return nil
}

func (m *Manager) postExecuteAction(ctx context.Context, resultCache *QueryResultCache, actionId string, contextData map[string]string) {
	meta := resultCache.PluginInstance.Metadata
	if isResultBindingMaintenanceAction(actionId) {
		return
	}

	// Ranking history describes how often a result is used, so maintenance actions
	// that only change Wox configuration must not boost the result they were run from.
	scoreHash := resultIdentityHash(meta.Id, resultCache.Result)
	setting.GetSettingManager().AddActionedResultByHash(ctx, scoreHash, resultCache.Query.RawQuery)

	// Add to MRU if plugin supports it
	if meta.IsSupportFeature(MetadataFeatureMRU) {
		mruItem := setting.MRUItem{
			PluginID:    meta.Id,
			Title:       resultCache.Result.Title,
			SubTitle:    resultCache.Result.SubTitle,
			Icon:        resultCache.Result.Icon,
			ContextData: contextData,
		}
		mruItem.Hash = resolveResultSourceHash(resultCache.PluginInstance, resultCache.Query, resultCache.Result, resultCache.SourceMRUHash)
		if err := setting.GetSettingManager().AddMRUItem(ctx, mruItem); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to add MRU item: %s", err.Error()))
		}
	}

	// Add to query history only if query is not empty (skip empty queries like MRU)
	if resultCache.Query.RawQuery != "" {
		setting.GetSettingManager().AddQueryHistory(ctx, plainQueryForHistory(resultCache.Query))
	}
}

func (m *Manager) GetResultPreview(ctx context.Context, sessionId string, queryId string, resultId string) (WoxPreview, error) {
	resultCache, found := m.resultStore.findInSession(sessionId, queryId, resultId)
	if !found {
		return WoxPreview{}, fmt.Errorf("result cache not found for result id (get preview): %s", resultId)
	}

	// if preview text is too long, ellipsis it, otherwise UI maybe freeze when render
	preview := resultCache.Result.Preview
	if preview.PreviewType == WoxPreviewTypeText {
		preview.PreviewData = util.EllipsisMiddle(preview.PreviewData, 2000)
		// translate preview data if preview type is text
		preview.PreviewData = m.translatePlugin(ctx, resultCache.PluginInstance, preview.PreviewData)
	}

	preview = m.normalizePreviewMetadata(ctx, resultCache.PluginInstance, preview)
	return preview, nil
}

func (m *Manager) ReplaceQueryVariable(ctx context.Context, queryText string) common.PlainQuery {
	// Track whether {wox:selected_file} was resolved so we can promote the query to
	// QueryTypeSelection. Plugins that handle file selections expect a Selection context,
	// not raw file paths embedded in a text query.
	var resolvedFileSelection *selection.Selection

	values := ResolveTextQueryVariables(ctx, queryText)

	// Replace selected file variable. When resolved, capture the selection so the caller
	// can promote the query to QueryTypeSelection instead of embedding paths as plain text.
	// Also strip the placeholder from queryText so the remaining text (e.g. a trigger keyword)
	// is still passed as QueryText and can be used for plugin routing in NewQuery.
	if strings.Contains(queryText, QueryVariableSelectedFile) {
		selected, selectedErr := selection.GetSelected(ctx)
		if selectedErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to get selected file: %s", selectedErr.Error()))
		} else {
			if selected.Type == selection.SelectionTypeFile {
				resolvedFileSelection = &selected
				values[QueryVariableSelectedFile] = ""
			} else {
				logger.Error(ctx, fmt.Sprintf("selected data is not file, type: %s", selected.Type))
			}
		}
	}

	if strings.Contains(queryText, QueryVariableActiveBrowserUrl) {
		activeBrowserUrl := m.activeBrowserUrl
		values[QueryVariableActiveBrowserUrl] = activeBrowserUrl
	}

	// Replace file explorer path variable if present
	if strings.Contains(queryText, QueryVariableFileExplorerPath) {
		startTime := time.Now()
		explorerPath := m.getActiveFileExplorerPath(ctx)
		values[QueryVariableFileExplorerPath] = explorerPath
		logger.Debug(ctx, fmt.Sprintf("replaced file explorer path variable in %d ms", time.Since(startTime).Milliseconds()))
	}

	// Substitute the original template once so captured text cannot introduce new variables.
	var replacements []string
	for token, value := range values {
		replacements = append(replacements, token, value)
	}
	queryText = strings.NewReplacer(replacements...).Replace(queryText)

	// If {wox:selected_file} was successfully resolved, promote to QueryTypeSelection
	// so that selection-aware plugins receive a proper file selection context rather
	// than raw path strings in a text query.
	// QueryText carries the remainder of the template string (e.g. a trigger keyword like "files")
	// so that NewQuery can parse it and route the query to the right plugin.
	if resolvedFileSelection != nil {
		return common.PlainQuery{
			QueryType:      QueryTypeSelection,
			QueryText:      queryText,
			QuerySelection: *resolvedFileSelection,
		}
	}

	return common.PlainQuery{
		QueryType: QueryTypeInput,
		QueryText: queryText,
	}
}

func (m *Manager) RuntimeStatusForRuntime(ctx context.Context, runtime Runtime) (RuntimeHostStatus, bool) {
	// Feature: callers that present install/load failures can reuse the same
	// structured runtime diagnosis as /runtime/status instead of parsing wrapped
	// startup errors that are meant for logs.
	return RuntimeStatusForRegisteredHost(ctx, runtime)
}

func (m *Manager) IsTriggerKeywordAIChat(ctx context.Context, triggerKeyword string) bool {
	aiChatPluginInstance := m.GetAIChatPluginInstance(ctx)
	if aiChatPluginInstance == nil {
		return false
	}

	return lo.Contains(aiChatPluginInstance.GetTriggerKeywords(), triggerKeyword)
}

func (m *Manager) GetAIChatPluginInstance(ctx context.Context) *Instance {
	aiChatPlugin := m.GetPluginInstances()
	aiChatPluginInstance, exist := lo.Find(aiChatPlugin, func(item *Instance) bool {
		return item.Metadata.Id == common.AIChatPluginID
	})
	if exist {
		return aiChatPluginInstance
	}

	return nil
}

func (m *Manager) GetAIChatPluginChater(ctx context.Context) common.AIChater {
	aiChatPluginInstance := m.GetAIChatPluginInstance(ctx)
	if aiChatPluginInstance == nil {
		return nil
	}

	chater, ok := aiChatPluginInstance.Plugin.(common.AIChater)
	if ok {
		return chater
	}

	return nil
}

func (m *Manager) GetAIProvider(ctx context.Context, provider common.ProviderName, alias string) (ai.Provider, error) {
	key := string(provider)
	if alias != "" {
		key = fmt.Sprintf("%s_%s", provider, alias)
	}
	if v, exist := m.aiProviders.Load(key); exist {
		return v, nil
	}

	//check if provider has setting
	aiProviderSettings := setting.GetSettingManager().GetWoxSetting(ctx).AIProviders.Get()
	providerSetting, providerSettingExist := lo.Find(aiProviderSettings, func(item setting.AIProvider) bool {
		return item.Name == provider && item.Alias == alias
	})
	if !providerSettingExist {
		return nil, fmt.Errorf("ai provider setting not found: %s (alias=%s)", provider, alias)
	}

	newProvider, newProviderErr := ai.NewProvider(ctx, providerSetting)
	if newProviderErr != nil {
		return nil, newProviderErr
	}
	m.aiProviders.Store(key, newProvider)
	return newProvider, nil
}

func (m *Manager) ExecutePluginDeeplink(ctx context.Context, pluginId string, arguments map[string]string) {
	pluginInstance, exist := lo.Find(m.pluginInstancesSnapshot(), func(item *Instance) bool {
		return item.Metadata.Id == pluginId
	})
	if !exist {
		logger.Error(ctx, fmt.Sprintf("plugin not found: %s", pluginId))
		return
	}

	logger.Info(ctx, fmt.Sprintf("execute deeplink for plugin: %s, callbacks: %d", pluginInstance.GetName(ctx), len(pluginInstance.DeepLinkCallbacks)))

	for _, callback := range pluginInstance.DeepLinkCallbacks {
		util.Go(ctx, fmt.Sprintf("[%s] execute deeplink callback", pluginInstance.GetName(ctx)), func() {
			callback(ctx, arguments)
		})
	}
}

// mruRestoreTimeout bounds one start-page MRU restore. The list is not drawn
// until the current restore returns, so a plugin that fetches or otherwise
// waits is skipped and the next item is used.
const mruRestoreTimeout = 300 * time.Millisecond

func (m *Manager) QueryMRU(ctx context.Context, sessionId string, queryId string) []QueryResultUI {
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, sessionId), queryId)
	activeWindowSnapshot := m.GetUI().GetActiveWindowSnapshot(ctx)
	query := Query{
		Id:        queryId,
		SessionId: sessionId,
		Type:      QueryTypeInput,
	}
	query.Env.ActiveWindowTitle = activeWindowSnapshot.Name
	query.Env.ActiveWindowPid = activeWindowSnapshot.Pid
	query.Env.ActiveWindowId = activeWindowSnapshot.WindowId
	query.Env.ActiveWindowIcon = activeWindowSnapshot.Icon
	query.Env.ActiveWindowIsOpenSaveDialog = activeWindowSnapshot.IsOpenSaveDialog
	query.Env.ActiveWindowIsOpenSaveDialogSelectFolder = activeWindowSnapshot.IsOpenSaveDialogSelectFolder
	query.Env.ActiveBrowserUrl = m.getActiveBrowserUrl(ctx)
	query.Env.IsMRU = true
	m.resultStore.startQuery(query)
	ctx = m.resultStore.withQueryResultLifetime(ctx, query.SessionId, query.Id)

	mruItems, err := setting.GetSettingManager().GetMRUItems(ctx, 10)
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to get MRU items: %s", err.Error()))
		return []QueryResultUI{}
	}

	// Deduplicate restored MRU results by (pluginId, title, subTitle, contextData)
	seen := make(map[string]bool)

	var results []QueryResultUI
	for _, item := range mruItems {
		util.GetLogger().Debug(ctx, fmt.Sprintf("start to restore mru item: %s", item.Title))
		pluginInstance := m.getPluginInstance(item.PluginID)
		if pluginInstance == nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("plugin not found, skip restore mru item: %s", item.Title))
			continue
		}
		if !pluginInstance.Metadata.IsSupportFeature(MetadataFeatureMRU) {
			util.GetLogger().Debug(ctx, fmt.Sprintf("plugin does not support mru, skip restore mru item: %s", item.Title))
			continue
		}

		pluginQuery := m.buildPluginQueryEnv(ctx, pluginInstance, query)
		if restored := m.restoreMRUItem(ctx, pluginInstance, item, pluginQuery.Env); restored != nil {
			util.GetLogger().Debug(ctx, fmt.Sprintf("mru item restored: %s", item.Title))

			// Build a stable dedupe key using restored values, which are language-independent for Go plugins
			key := fmt.Sprintf("%s|%s|%s|%s", item.PluginID, restored.Title, restored.SubTitle, m.serializeContextData(item.ContextData))
			if seen[key] {
				util.GetLogger().Debug(ctx, fmt.Sprintf("duplicate mru item, skip restore mru item: %s", item.Title))
				continue
			}
			seen[key] = true

			// Keep the core-owned plugin settings action consistent with regular query results.
			restored.Actions = append(restored.Actions, m.newOpenPluginSettingAction(ctx, pluginInstance))
			restored.Actions = append(restored.Actions, m.newResultBindingActions(ctx, pluginInstance, query, *restored, item.Hash)...)

			// Keep the homepage visible so users can continue managing its items.
			removeMRUAction := QueryResultAction{
				Id:                     systemActionRemoveFromMRUID,
				Name:                   i18n.GetI18nManager().TranslateWox(ctx, "mru_remove_action"),
				Icon:                   icons.Get(icons.ActionDelete),
				IsSystemAction:         true,
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext ActionContext) {
					err := setting.GetSettingManager().RemoveMRUItem(ctx, item.Hash)
					if err != nil {
						util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove MRU item: %s", err.Error()))
					} else {
						util.GetLogger().Info(ctx, fmt.Sprintf("removed MRU item: %s - %s", item.Title, item.SubTitle))
						m.GetUI().RefreshQuery(ctx, common.RefreshQueryOptions{PreserveSelectedIndex: true})
					}
				},
			}

			restored.Actions = append(restored.Actions, removeMRUAction)
			restored.Score = item.Score

			polishedResult := m.polishResultWithSource(ctx, pluginInstance, query, QueryLayout{}, *restored, item.Hash, aliasMatchNone)
			results = append(results, polishedResult.ToUI())
		}
	}

	if len(results) == 0 {
		return results
	}

	return m.BuildQueryResultsSnapshot(query.SessionId, query.Id)
}

// BuildScopeIcons resolves core-owned query-box icons from QueryScope order.
// Missing or unloadable plugins are skipped so a partial allowlist still shows icons.
func (m *Manager) BuildScopeIcons(ctx context.Context, query Query) []common.WoxImage {
	if !query.HasScope() {
		return nil
	}
	deduped := query.Scope.Deduplicate()
	icons := make([]common.WoxImage, 0, len(deduped.Plugins))
	for _, item := range deduped.Plugins {
		pluginInstance := m.getPluginInstance(item.PluginID)
		if pluginInstance == nil {
			continue
		}
		iconImg, parseErr := common.ParseWoxImage(pluginInstance.Metadata.Icon)
		if parseErr != nil {
			logger.Error(ctx, fmt.Sprintf("failed to parse scope icon for %s: %s", pluginInstance.Metadata.Id, parseErr.Error()))
			continue
		}
		icons = append(icons, pluginInstance.ConvertIcon(ctx, iconImg))
		if len(icons) >= 3 {
			break
		}
	}
	return icons
}

// getPluginInstance finds a plugin instance by ID
func (m *Manager) getPluginInstance(pluginID string) *Instance {
	pluginInstance, found := lo.Find(m.pluginInstancesSnapshot(), func(item *Instance) bool {
		return item.Metadata.Id == pluginID
	})
	if found {
		return pluginInstance
	}
	return nil
}

// restoreMRUItem waits at most mruRestoreTimeout. A restore that is still
// running is cancelled and logged, and the caller continues with the next item.
func (m *Manager) restoreMRUItem(ctx context.Context, pluginInstance *Instance, item setting.MRUItem, env QueryEnv) *QueryResult {
	if ctx.Err() != nil || pluginInstance == nil {
		return nil
	}
	restoreCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	done := make(chan *QueryResult, 1)
	util.Go(restoreCtx, "restore mru item", func() {
		done <- m.restoreFromMRU(restoreCtx, pluginInstance, item, env)
	})
	timer := time.NewTimer(mruRestoreTimeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return nil
	case <-timer.C:
		cancel()
		util.GetLogger().Warn(ctx, fmt.Sprintf("MRU restore exceeded %s, skip to next item: plugin=%s title=%s", mruRestoreTimeout, pluginInstance.GetName(ctx), item.Title))
		return nil
	case restored := <-done:
		return restored
	}
}

// restoreFromMRU attempts to restore a QueryResult from MRU data
func (m *Manager) restoreFromMRU(ctx context.Context, pluginInstance *Instance, item setting.MRUItem, env QueryEnv) *QueryResult {
	// For Go plugins, call MRU restore callbacks directly
	if len(pluginInstance.MRURestoreCallbacks) > 0 {
		mruData := MRUData{
			PluginID:    item.PluginID,
			Title:       item.Title,
			SubTitle:    item.SubTitle,
			Icon:        item.Icon,
			ContextData: item.ContextData,
			Env:         env,
			LastUsed:    item.LastUsed,
			UseCount:    item.UseCount,
		}

		// Call the first (and typically only) MRU restore callback
		if restored, err := pluginInstance.MRURestoreCallbacks[0](ctx, mruData); err == nil {
			return restored
		} else {
			util.GetLogger().Debug(ctx, fmt.Sprintf("MRU restore failed for plugin %s: %s", pluginInstance.GetName(ctx), err.Error()))
		}
	}

	// For external plugins (Python/Node.js), MRU support will be implemented later
	// Currently only Go plugins support MRU functionality
	if pluginInstance.Host != nil {
		util.GetLogger().Debug(ctx, fmt.Sprintf("External plugin MRU restore not yet implemented for plugin %s", pluginInstance.GetName(ctx)))
	}

	return nil
}
