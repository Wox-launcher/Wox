package plugin

import (
	"context"
	"fmt"
	"sort"
	"time"

	"wox/common"
	"wox/util"
	"wox/util/timetracking"

	"github.com/google/uuid"
)

func (m *Manager) PolishResult(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, result QueryResult) QueryResult {
	return m.polishResultWithSource(ctx, pluginInstance, query, layout, result, "", aliasMatchNone)
}

// polishResultWithSource publishes restore identity together with the cached result.
func (m *Manager) polishResultWithSource(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, result QueryResult, sourceHash string, match aliasMatchKind) QueryResult {
	result, timing := m.polishResult(ctx, pluginInstance, query, layout, result, false, nil, nil, sourceHash, match)
	m.logPolishResultTiming(ctx, query, queryDiagnosticPluginLabel(pluginInstance), result, timing)
	return result
}

func (m *Manager) polishResult(ctx context.Context, pluginInstance *Instance, query Query, layout QueryLayout, result QueryResult, collectIconTiming bool, actionIconCache, resultIconCache map[common.WoxImage]common.WoxImage, sourceHash string, match aliasMatchKind) (QueryResult, timetracking.ResultPolishTiming) {
	polishStart := util.GetSystemTimestamp()
	polishTimingStart := time.Now()
	var IconConversion timetracking.IconConversionTimingSummary
	var IconConversionSet bool
	var iconRecorder func(timetracking.IconConversionDiagnostics, timetracking.IconConversionTimingSummary)
	if collectIconTiming {
		iconRecorder = func(_ timetracking.IconConversionDiagnostics, timing timetracking.IconConversionTimingSummary) {
			IconConversion = timing
			IconConversionSet = true
		}
	}
	actionSetupStart := util.GetSystemTimestamp()
	actionSetupTimingStart := time.Now()
	actionDefaultsStart := util.GetSystemTimestamp()
	actionDefaultsTimingStart := time.Now()
	// set default id
	if result.Id == "" {
		result.Id = uuid.NewString()
	}
	normalizeResultActionDefaults(result.Actions)
	ActionDefaultsCost := util.GetSystemTimestamp() - actionDefaultsStart
	ActionDefaultsCostUs := time.Since(actionDefaultsTimingStart).Microseconds()
	actionIconStart := util.GetSystemTimestamp()
	actionIconTimingStart := time.Now()
	ActionIconCount := convertActionIcons(ctx, result.Actions, pluginInstance, actionIconCache)
	ActionIconCost := util.GetSystemTimestamp() - actionIconStart
	ActionIconCostUs := time.Since(actionIconTimingStart).Microseconds()
	actionCallbackStart := util.GetSystemTimestamp()
	actionCallbackTimingStart := time.Now()
	m.attachExternalActionCallbacks(pluginInstance, result.Actions)
	ActionCallbackCost := util.GetSystemTimestamp() - actionCallbackStart
	ActionCallbackCostUs := time.Since(actionCallbackTimingStart).Microseconds()
	ActionSetupCost := util.GetSystemTimestamp() - actionSetupStart
	ActionSetupCostUs := time.Since(actionSetupTimingStart).Microseconds()

	visualStart := util.GetSystemTimestamp()
	visualTimingStart := time.Now()
	// Result icons are converted for the visual surface selected by metadata. The old
	// hard-coded list size made grid icons blurry because the UI had to scale 40px
	// raster caches into much larger cells.
	ResultIconType := result.Icon.ImageType
	resultIconStart := util.GetSystemTimestamp()
	resultIconTimingStart := time.Now()
	sourceIcon := result.Icon
	if converted, found := resultIconCache[sourceIcon]; found {
		result.Icon = converted
	} else {
		result.Icon = m.convertResultIconWithRecorder(ctx, pluginInstance, query, layout, result.Id, result.Title, sourceIcon, iconRecorder)
		// This batch has one plugin/layout/size. Share ordinary conversions only:
		// lazy icons carry per-result tokens and must retain distinct ownership.
		if resultIconCache != nil && len(resultIconCache) < 128 && result.Icon.ImageType != common.WoxImageTypeLazyLoad {
			resultIconCache[sourceIcon] = result.Icon
		}
	}
	ResultIconCost := util.GetSystemTimestamp() - resultIconStart
	ResultIconCostUs := time.Since(resultIconTimingStart).Microseconds()
	ConvertedResultIconType := result.Icon.ImageType
	dragStart := util.GetSystemTimestamp()
	dragTimingStart := time.Now()
	result.DragData = normalizeQueryResultDragData(result.DragData)
	DragCost := util.GetSystemTimestamp() - dragStart
	DragCostUs := time.Since(dragTimingStart).Microseconds()
	tailIconStart := util.GetSystemTimestamp()
	tailIconTimingStart := time.Now()
	TailImageCount := convertResultTailImages(ctx, pluginInstance, result.Tails)
	TailIconCost := util.GetSystemTimestamp() - tailIconStart
	TailIconCostUs := time.Since(tailIconTimingStart).Microseconds()
	VisualCost := util.GetSystemTimestamp() - visualStart
	VisualCostUs := time.Since(visualTimingStart).Microseconds()

	previewStart := util.GetSystemTimestamp()
	previewTimingStart := time.Now()
	previewDefaultStart := util.GetSystemTimestamp()
	previewDefaultTimingStart := time.Now()
	result.Preview = m.applyDefaultSelectionPreview(ctx, query, result.Preview)
	PreviewDefaultCost := util.GetSystemTimestamp() - previewDefaultStart
	PreviewDefaultCostUs := time.Since(previewDefaultTimingStart).Microseconds()
	previewNormalizeStart := util.GetSystemTimestamp()
	previewNormalizeTimingStart := time.Now()
	result.Preview = m.normalizeListPreviewData(ctx, pluginInstance, result.Preview)
	PreviewNormalizeCost := util.GetSystemTimestamp() - previewNormalizeStart
	PreviewNormalizeCostUs := time.Since(previewNormalizeTimingStart).Microseconds()
	PreviewCost := util.GetSystemTimestamp() - previewStart
	PreviewCostUs := time.Since(previewTimingStart).Microseconds()

	translationStart := util.GetSystemTimestamp()
	translationTimingStart := time.Now()
	translationTextStart := util.GetSystemTimestamp()
	translationTextTimingStart := time.Now()
	// translate title
	result.Title = m.translatePlugin(ctx, pluginInstance, result.Title)
	// translate subtitle
	result.SubTitle = m.translatePlugin(ctx, pluginInstance, result.SubTitle)
	TranslationTextCost := util.GetSystemTimestamp() - translationTextStart
	TranslationTextCostUs := time.Since(translationTextTimingStart).Microseconds()
	translationTailStart := util.GetSystemTimestamp()
	translationTailTimingStart := time.Now()
	m.translateResultTails(ctx, pluginInstance, result.Tails)
	TranslationTailCost := util.GetSystemTimestamp() - translationTailStart
	TranslationTailCostUs := time.Since(translationTailTimingStart).Microseconds()
	translationPreviewMetaStart := util.GetSystemTimestamp()
	translationPreviewMetaTimingStart := time.Now()
	result.Preview = m.normalizePreviewMetadata(ctx, pluginInstance, result.Preview)
	TranslationPreviewMetaCost := util.GetSystemTimestamp() - translationPreviewMetaStart
	TranslationPreviewMetaCostUs := time.Since(translationPreviewMetaTimingStart).Microseconds()
	translationActionStart := util.GetSystemTimestamp()
	translationActionTimingStart := time.Now()
	m.translateResultActions(ctx, pluginInstance, result.Actions)
	TranslationActionCost := util.GetSystemTimestamp() - translationActionStart
	TranslationActionCostUs := time.Since(translationActionTimingStart).Microseconds()
	translationPreviewDataStart := util.GetSystemTimestamp()
	translationPreviewDataTimingStart := time.Now()
	// translate preview data if preview type is text
	if result.Preview.PreviewType == WoxPreviewTypeText || result.Preview.PreviewType == WoxPreviewTypeMarkdown {
		result.Preview.PreviewData = m.translatePlugin(ctx, pluginInstance, result.Preview.PreviewData)
	}
	TranslationPreviewDataCost := util.GetSystemTimestamp() - translationPreviewDataStart
	TranslationPreviewDataCostUs := time.Since(translationPreviewDataTimingStart).Microseconds()
	translationGroupStart := util.GetSystemTimestamp()
	translationGroupTimingStart := time.Now()
	// translate group name
	result.Group = m.translatePlugin(ctx, pluginInstance, result.Group)
	TranslationGroupCost := util.GetSystemTimestamp() - translationGroupStart
	TranslationGroupCostUs := time.Since(translationGroupTimingStart).Microseconds()
	TranslationCost := util.GetSystemTimestamp() - translationStart
	TranslationCostUs := time.Since(translationTimingStart).Microseconds()

	actionNormalizeStart := util.GetSystemTimestamp()
	actionNormalizeTimingStart := time.Now()
	actionDefaultStart := util.GetSystemTimestamp()
	actionDefaultTimingStart := time.Now()
	ensureDefaultResultAction(result.Actions)
	ActionDefaultCost := util.GetSystemTimestamp() - actionDefaultStart
	ActionDefaultCostUs := time.Since(actionDefaultTimingStart).Microseconds()

	actionSortStart := util.GetSystemTimestamp()
	actionSortTimingStart := time.Now()
	//move default action to first one of the actions
	sort.Slice(result.Actions, func(i, j int) bool {
		return result.Actions[i].IsDefault
	})
	ActionSortCost := util.GetSystemTimestamp() - actionSortStart
	ActionSortCostUs := time.Since(actionSortTimingStart).Microseconds()

	hotkeyNormalizeStart := util.GetSystemTimestamp()
	hotkeyNormalizeTimingStart := time.Now()
	normalizeResultActionHotkeys(result.Actions)
	HotkeyNormalizeCost := util.GetSystemTimestamp() - hotkeyNormalizeStart
	HotkeyNormalizeCostUs := time.Since(hotkeyNormalizeTimingStart).Microseconds()
	ActionNormalizeCost := util.GetSystemTimestamp() - actionNormalizeStart
	ActionNormalizeCostUs := time.Since(actionNormalizeTimingStart).Microseconds()

	previewRemoteStart := util.GetSystemTimestamp()
	previewRemoteTimingStart := time.Now()
	previewGlobalStart := util.GetSystemTimestamp()
	previewGlobalTimingStart := time.Now()
	// Global search stays compact: hide previews, and flatten groups except file search.
	if shouldHidePreviewForGlobalQuery(query, result.Preview) {
		result.Preview = WoxPreview{}
	}
	if shouldClearGroupForGlobalQuery(query, pluginInstance) {
		result.Group = ""
		result.GroupScore = 0
	}
	PreviewGlobalCost := util.GetSystemTimestamp() - previewGlobalStart
	PreviewGlobalCostUs := time.Since(previewGlobalTimingStart).Microseconds()

	// store preview for ui invoke later
	// because preview may contain some heavy data (E.g. image or large text),
	// we will store preview in cache and only send preview to ui when user select the result
	var originalPreview = result.Preview
	previewWrapStart := util.GetSystemTimestamp()
	previewWrapTimingStart := time.Now()
	// Core-owned interactive previews intentionally bypass remote wrapping so the UI
	// can detect the type before deciding whether grid previews are allowed.
	if shouldWrapRemotePreview(result.Preview) {
		result.Preview = WoxPreview{
			PreviewType:   WoxPreviewTypeRemote,
			PreviewData:   fmt.Sprintf("/preview?sessionId=%s&queryId=%s&id=%s", query.SessionId, query.Id, result.Id),
			DefaultHidden: originalPreview.DefaultHidden,
		}
	}
	PreviewWrapCost := util.GetSystemTimestamp() - previewWrapStart
	PreviewWrapCostUs := time.Since(previewWrapTimingStart).Microseconds()
	PreviewRemoteCost := util.GetSystemTimestamp() - previewRemoteStart
	PreviewRemoteCostUs := time.Since(previewRemoteTimingStart).Microseconds()

	scoreStart := util.GetSystemTimestamp()
	scoreTimingStart := time.Now()
	result.Score = limitGlobalQueryPluginScore(query, result.Score)
	scoreFeatureStart := util.GetSystemTimestamp()
	scoreFeatureTimingStart := time.Now()
	// ignoreAutoScore is a plugin-context control; global search still needs actioned-result ranking across providers.
	ignoreAutoScore := !query.IsGlobalQuery() && pluginInstance.Metadata.IsSupportFeature(MetadataFeatureIgnoreAutoScore)
	isMRUQuery := query.Env.IsMRU
	if isMRUQuery {
		// MRU restore owns ranking; plugin/action/favorite scores would leak normal query ranking into the MRU page.
		ignoreAutoScore = true
	}
	ScoreFeatureCost := util.GetSystemTimestamp() - scoreFeatureStart
	ScoreFeatureCostUs := time.Since(scoreFeatureTimingStart).Microseconds()
	autoScoreStart := util.GetSystemTimestamp()
	autoScoreTimingStart := time.Now()
	if !ignoreAutoScore {
		score := m.calculateResultScore(ctx, pluginInstance.Metadata.Id, result, query)
		if score > 0 {
			logger.Debug(ctx, fmt.Sprintf("<%s> result(%s) add score: %d", pluginInstance.GetName(ctx), result.Title, score))
			result.Score += score
		}
	}
	// System plugins use this to keep a navigation row, such as "..", first in global search.
	if result.RankAboveUsage && !isMRUQuery {
		result.Score += rankAboveUsageScore
	}
	AutoScoreCost := util.GetSystemTimestamp() - autoScoreStart
	AutoScoreCostUs := time.Since(autoScoreTimingStart).Microseconds()
	favoriteStart := util.GetSystemTimestamp()
	favoriteTimingStart := time.Now()
	result.Tails = m.appendPinnedResultTail(ctx, pluginInstance, query, result, result.Tails)
	FavoriteCost := util.GetSystemTimestamp() - favoriteStart
	FavoriteCostUs := time.Since(favoriteTimingStart).Microseconds()

	devScoreTailStart := util.GetSystemTimestamp()
	devScoreTailTimingStart := time.Now()
	if result.isFallback {
		// Apply after usage and navigation boosts so late ordinary results sort ahead
		// of provisional fallback rows in both the UI response and cached snapshots.
		result.Score = -1
	}
	result.Tails = m.appendDevScoreTail(ctx, result.Tails, result.Score)
	sourceHash = resolveResultSourceHash(pluginInstance, query, result, sourceHash)
	m.applyResultBindingTitleTags(ctx, pluginInstance, query, &result, sourceHash)
	DevScoreTailCost := util.GetSystemTimestamp() - devScoreTailStart
	DevScoreTailCostUs := time.Since(devScoreTailTimingStart).Microseconds()
	ScoreCost := util.GetSystemTimestamp() - scoreStart
	ScoreCostUs := time.Since(scoreTimingStart).Microseconds()

	cacheStart := util.GetSystemTimestamp()
	cacheTimingStart := time.Now()
	// Create cache at the end
	resultCopy := result
	// Because we may have replaced preview with remote preview
	// we need to restore the original preview in the cache
	resultCopy.Preview = originalPreview
	m.resultStore.storeResult(ctx, pluginInstance, query, layout, resultCopy, sourceHash, match)
	CacheCost := util.GetSystemTimestamp() - cacheStart
	CacheCostUs := time.Since(cacheTimingStart).Microseconds()
	polishCost := util.GetSystemTimestamp() - polishStart
	polishCostUs := time.Since(polishTimingStart).Microseconds()
	return result, timetracking.ResultPolishTiming{
		TotalCost:                    polishCost,
		TotalCostUs:                  polishCostUs,
		ActionSetupCost:              ActionSetupCost,
		ActionSetupCostUs:            ActionSetupCostUs,
		ActionDefaultsCost:           ActionDefaultsCost,
		ActionDefaultsCostUs:         ActionDefaultsCostUs,
		ActionIconCost:               ActionIconCost,
		ActionIconCostUs:             ActionIconCostUs,
		ActionCallbackCost:           ActionCallbackCost,
		ActionCallbackCostUs:         ActionCallbackCostUs,
		ActionIconCount:              ActionIconCount,
		VisualCost:                   VisualCost,
		VisualCostUs:                 VisualCostUs,
		ResultIconCost:               ResultIconCost,
		ResultIconCostUs:             ResultIconCostUs,
		ResultIconType:               ResultIconType,
		ConvertedResultIconType:      ConvertedResultIconType,
		IconConversion:               IconConversion,
		IconConversionSet:            IconConversionSet,
		DragCost:                     DragCost,
		DragCostUs:                   DragCostUs,
		TailIconCost:                 TailIconCost,
		TailIconCostUs:               TailIconCostUs,
		TailImageCount:               TailImageCount,
		PreviewCost:                  PreviewCost,
		PreviewCostUs:                PreviewCostUs,
		PreviewDefaultCost:           PreviewDefaultCost,
		PreviewDefaultCostUs:         PreviewDefaultCostUs,
		PreviewNormalizeCost:         PreviewNormalizeCost,
		PreviewNormalizeCostUs:       PreviewNormalizeCostUs,
		TranslationCost:              TranslationCost,
		TranslationCostUs:            TranslationCostUs,
		TranslationTextCost:          TranslationTextCost,
		TranslationTextCostUs:        TranslationTextCostUs,
		TranslationTailCost:          TranslationTailCost,
		TranslationTailCostUs:        TranslationTailCostUs,
		TranslationPreviewMetaCost:   TranslationPreviewMetaCost,
		TranslationPreviewMetaCostUs: TranslationPreviewMetaCostUs,
		TranslationActionCost:        TranslationActionCost,
		TranslationActionCostUs:      TranslationActionCostUs,
		TranslationPreviewDataCost:   TranslationPreviewDataCost,
		TranslationPreviewDataCostUs: TranslationPreviewDataCostUs,
		TranslationGroupCost:         TranslationGroupCost,
		TranslationGroupCostUs:       TranslationGroupCostUs,
		ActionNormalizeCost:          ActionNormalizeCost,
		ActionNormalizeCostUs:        ActionNormalizeCostUs,
		ActionDefaultCost:            ActionDefaultCost,
		ActionDefaultCostUs:          ActionDefaultCostUs,
		ActionSortCost:               ActionSortCost,
		ActionSortCostUs:             ActionSortCostUs,
		HotkeyNormalizeCost:          HotkeyNormalizeCost,
		HotkeyNormalizeCostUs:        HotkeyNormalizeCostUs,
		PreviewRemoteCost:            PreviewRemoteCost,
		PreviewRemoteCostUs:          PreviewRemoteCostUs,
		PreviewGlobalCost:            PreviewGlobalCost,
		PreviewGlobalCostUs:          PreviewGlobalCostUs,
		PreviewWrapCost:              PreviewWrapCost,
		PreviewWrapCostUs:            PreviewWrapCostUs,
		ScoreCost:                    ScoreCost,
		ScoreCostUs:                  ScoreCostUs,
		ScoreFeatureCost:             ScoreFeatureCost,
		ScoreFeatureCostUs:           ScoreFeatureCostUs,
		AutoScoreCost:                AutoScoreCost,
		AutoScoreCostUs:              AutoScoreCostUs,
		FavoriteCost:                 FavoriteCost,
		FavoriteCostUs:               FavoriteCostUs,
		DevScoreTailCost:             DevScoreTailCost,
		DevScoreTailCostUs:           DevScoreTailCostUs,
		CacheCost:                    CacheCost,
		CacheCostUs:                  CacheCostUs,
		PreviewDataLen:               len(originalPreview.PreviewData),
	}
}

// logPolishResultTiming preserves the existing slow-result diagnostic for paths that are not already aggregated by query finalize.
func (m *Manager) logPolishResultTiming(ctx context.Context, query Query, pluginLabel string, result QueryResult, timing timetracking.ResultPolishTiming) {
	if !util.IsDev() {
		return
	}
	if timing.TotalCost < 1 && timing.TotalCostUs < 500 {
		return
	}

	tracker := timetracking.New("polish_result_done")
	if !tracker.Enabled() {
		return
	}
	tracker.SetRawString("queryId", query.Id)
	tracker.SetRawString("plugin", pluginLabel)
	tracker.SetRawString("resultId", result.Id)
	tracker.SetString("title", result.Title)
	tracker.SetInt64("totalMs", timing.TotalCost)
	tracker.SetInt64("totalUs", timing.TotalCostUs)
	tracker.SetInt64("actionSetupMs", timing.ActionSetupCost)
	tracker.SetInt64("actionSetupUs", timing.ActionSetupCostUs)
	tracker.SetInt64("actionDefaultsMs", timing.ActionDefaultsCost)
	tracker.SetInt64("actionDefaultsUs", timing.ActionDefaultsCostUs)
	tracker.SetInt64("actionIconMs", timing.ActionIconCost)
	tracker.SetInt64("actionIconUs", timing.ActionIconCostUs)
	tracker.SetInt64("actionCallbackMs", timing.ActionCallbackCost)
	tracker.SetInt64("actionCallbackUs", timing.ActionCallbackCostUs)
	tracker.SetInt("actionIconCount", timing.ActionIconCount)
	tracker.SetInt64("visualMs", timing.VisualCost)
	tracker.SetInt64("visualUs", timing.VisualCostUs)
	tracker.SetInt64("resultIconMs", timing.ResultIconCost)
	tracker.SetInt64("resultIconUs", timing.ResultIconCostUs)
	tracker.SetRawString("resultIconType", timing.ResultIconType)
	tracker.SetRawString("convertedResultIconType", timing.ConvertedResultIconType)
	tracker.SetInt64("dragMs", timing.DragCost)
	tracker.SetInt64("dragUs", timing.DragCostUs)
	tracker.SetInt64("tailIconMs", timing.TailIconCost)
	tracker.SetInt64("tailIconUs", timing.TailIconCostUs)
	tracker.SetInt("tailImageCount", timing.TailImageCount)
	tracker.SetInt64("previewMs", timing.PreviewCost)
	tracker.SetInt64("previewUs", timing.PreviewCostUs)
	tracker.SetInt64("previewDefaultMs", timing.PreviewDefaultCost)
	tracker.SetInt64("previewDefaultUs", timing.PreviewDefaultCostUs)
	tracker.SetInt64("previewNormalizeMs", timing.PreviewNormalizeCost)
	tracker.SetInt64("previewNormalizeUs", timing.PreviewNormalizeCostUs)
	tracker.SetInt64("translationMs", timing.TranslationCost)
	tracker.SetInt64("translationUs", timing.TranslationCostUs)
	tracker.SetInt64("translationTextMs", timing.TranslationTextCost)
	tracker.SetInt64("translationTextUs", timing.TranslationTextCostUs)
	tracker.SetInt64("translationTailMs", timing.TranslationTailCost)
	tracker.SetInt64("translationTailUs", timing.TranslationTailCostUs)
	tracker.SetInt64("translationPreviewMetaMs", timing.TranslationPreviewMetaCost)
	tracker.SetInt64("translationPreviewMetaUs", timing.TranslationPreviewMetaCostUs)
	tracker.SetInt64("translationActionMs", timing.TranslationActionCost)
	tracker.SetInt64("translationActionUs", timing.TranslationActionCostUs)
	tracker.SetInt64("translationPreviewDataMs", timing.TranslationPreviewDataCost)
	tracker.SetInt64("translationPreviewDataUs", timing.TranslationPreviewDataCostUs)
	tracker.SetInt64("translationGroupMs", timing.TranslationGroupCost)
	tracker.SetInt64("translationGroupUs", timing.TranslationGroupCostUs)
	tracker.SetInt64("actionNormalizeMs", timing.ActionNormalizeCost)
	tracker.SetInt64("actionNormalizeUs", timing.ActionNormalizeCostUs)
	tracker.SetInt64("actionDefaultMs", timing.ActionDefaultCost)
	tracker.SetInt64("actionDefaultUs", timing.ActionDefaultCostUs)
	tracker.SetInt64("actionSortMs", timing.ActionSortCost)
	tracker.SetInt64("actionSortUs", timing.ActionSortCostUs)
	tracker.SetInt64("hotkeyNormalizeMs", timing.HotkeyNormalizeCost)
	tracker.SetInt64("hotkeyNormalizeUs", timing.HotkeyNormalizeCostUs)
	tracker.SetInt64("previewRemoteMs", timing.PreviewRemoteCost)
	tracker.SetInt64("previewRemoteUs", timing.PreviewRemoteCostUs)
	tracker.SetInt64("previewGlobalMs", timing.PreviewGlobalCost)
	tracker.SetInt64("previewGlobalUs", timing.PreviewGlobalCostUs)
	tracker.SetInt64("previewWrapMs", timing.PreviewWrapCost)
	tracker.SetInt64("previewWrapUs", timing.PreviewWrapCostUs)
	tracker.SetInt64("scoreMs", timing.ScoreCost)
	tracker.SetInt64("scoreUs", timing.ScoreCostUs)
	tracker.SetInt64("scoreFeatureMs", timing.ScoreFeatureCost)
	tracker.SetInt64("scoreFeatureUs", timing.ScoreFeatureCostUs)
	tracker.SetInt64("autoScoreMs", timing.AutoScoreCost)
	tracker.SetInt64("autoScoreUs", timing.AutoScoreCostUs)
	tracker.SetInt64("favoriteMs", timing.FavoriteCost)
	tracker.SetInt64("favoriteUs", timing.FavoriteCostUs)
	tracker.SetInt64("devScoreTailMs", timing.DevScoreTailCost)
	tracker.SetInt64("devScoreTailUs", timing.DevScoreTailCostUs)
	tracker.SetInt64("cacheMs", timing.CacheCost)
	tracker.SetInt64("cacheUs", timing.CacheCostUs)
	tracker.SetInt("actions", len(result.Actions))
	tracker.SetInt("tails", len(result.Tails))
	tracker.SetInt("previewDataLen", timing.PreviewDataLen)
	tracker.Log(ctx)
}

func (m *Manager) getResultIconSizeForQuery(pluginInstance *Instance, query Query, layout QueryLayout) int {
	if pluginInstance == nil {
		return common.ResultListIconSize
	}

	// QueryResponse layout is now the preferred grid source. The previous check
	// only read deprecated metadata, which made plugins that migrated to
	// QueryResponse render grid images at list size before UI placed them
	// into grid cells.
	if layout.GridLayout != nil {
		return common.ResultGridIconSize
	}

	// Legacy metadata remains a compatibility fallback for older plugins that
	// still declare gridLayout in plugin.json while they migrate to QueryResponse.
	if _, isGridLayout, err := pluginInstance.Metadata.GetFeatureParamsForGridLayoutCommand(query.Command); err == nil && isGridLayout {
		return common.ResultGridIconSize
	}

	return common.ResultListIconSize
}
