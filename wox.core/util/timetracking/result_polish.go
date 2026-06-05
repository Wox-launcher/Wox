package timetracking

import "context"

// ResultIdentity is the business-free identity used by timing aggregates.
type ResultIdentity struct {
	Index int
	Id    string
	Title string
}

// MaxTiming keeps the slowest result for one timing dimension.
type MaxTiming struct {
	CostUs int64
	Result ResultIdentity
}

// Record updates the max entry when costUs is slower than the current value.
func (m *MaxTiming) Record(costUs int64, result ResultIdentity) {
	if costUs <= m.CostUs {
		return
	}

	m.CostUs = costUs
	m.Result = result
}

// ResultPolishTiming keeps per-result polish timings in memory so query diagnostics can log one aggregate line after the measured loop.
type ResultPolishTiming struct {
	TotalCost                    int64
	TotalCostUs                  int64
	ActionSetupCost              int64
	ActionSetupCostUs            int64
	ActionDefaultsCost           int64
	ActionDefaultsCostUs         int64
	ActionIconCost               int64
	ActionIconCostUs             int64
	ActionCallbackCost           int64
	ActionCallbackCostUs         int64
	ActionIconCount              int
	VisualCost                   int64
	VisualCostUs                 int64
	ResultIconCost               int64
	ResultIconCostUs             int64
	ResultIconType               string
	ConvertedResultIconType      string
	IconConversion               IconConversionTimingSummary
	IconConversionSet            bool
	DragCost                     int64
	DragCostUs                   int64
	TailIconCost                 int64
	TailIconCostUs               int64
	TailImageCount               int
	PreviewCost                  int64
	PreviewCostUs                int64
	PreviewDefaultCost           int64
	PreviewDefaultCostUs         int64
	PreviewNormalizeCost         int64
	PreviewNormalizeCostUs       int64
	TranslationCost              int64
	TranslationCostUs            int64
	TranslationTextCost          int64
	TranslationTextCostUs        int64
	TranslationTailCost          int64
	TranslationTailCostUs        int64
	TranslationPreviewMetaCost   int64
	TranslationPreviewMetaCostUs int64
	TranslationActionCost        int64
	TranslationActionCostUs      int64
	TranslationPreviewDataCost   int64
	TranslationPreviewDataCostUs int64
	TranslationGroupCost         int64
	TranslationGroupCostUs       int64
	ActionNormalizeCost          int64
	ActionNormalizeCostUs        int64
	ActionDefaultCost            int64
	ActionDefaultCostUs          int64
	ActionSortCost               int64
	ActionSortCostUs             int64
	HotkeyNormalizeCost          int64
	HotkeyNormalizeCostUs        int64
	PreviewRemoteCost            int64
	PreviewRemoteCostUs          int64
	PreviewGlobalCost            int64
	PreviewGlobalCostUs          int64
	PreviewWrapCost              int64
	PreviewWrapCostUs            int64
	ScoreCost                    int64
	ScoreCostUs                  int64
	ScoreFeatureCost             int64
	ScoreFeatureCostUs           int64
	AutoScoreCost                int64
	AutoScoreCostUs              int64
	FavoriteCost                 int64
	FavoriteCostUs               int64
	DevScoreTailCost             int64
	DevScoreTailCostUs           int64
	CacheCost                    int64
	CacheCostUs                  int64
	PreviewDataLen               int
}

// ResultsPolishAggregate summarizes result polish details without writing one debug line per result inside the hot path.
type ResultsPolishAggregate struct {
	Count                     int
	DefaultActionsUs          int64
	RecordPluginElapsedUs     int64
	ResultTotalUs             int64
	PolishUs                  int64
	ActionSetupUs             int64
	ActionDefaultsUs          int64
	ActionIconUs              int64
	ActionCallbackUs          int64
	ActionIconCount           int
	VisualUs                  int64
	ResultIconUs              int64
	DragUs                    int64
	TailIconUs                int64
	TailImageCount            int
	PreviewUs                 int64
	PreviewDefaultUs          int64
	PreviewNormalizeUs        int64
	TranslationUs             int64
	TranslationTextUs         int64
	TranslationTailUs         int64
	TranslationPreviewMetaUs  int64
	TranslationActionUs       int64
	TranslationPreviewDataUs  int64
	TranslationGroupUs        int64
	ActionNormalizeUs         int64
	ActionDefaultUs           int64
	ActionSortUs              int64
	HotkeyNormalizeUs         int64
	PreviewRemoteUs           int64
	PreviewGlobalUs           int64
	PreviewWrapUs             int64
	ScoreUs                   int64
	ScoreFeatureUs            int64
	AutoScoreUs               int64
	FavoriteUs                int64
	DevScoreTailUs            int64
	CacheUs                   int64
	PreviewDataLen            int
	IconConvertCount          int
	IconConvertUs             int64
	IconFileIconUs            int64
	IconRelativeUs            int64
	IconSvgCheckUs            int64
	IconCacheUs               int64
	IconLazyCheckUs           int64
	IconCropUs                int64
	IconResizeUs              int64
	IconCacheHits             int
	IconAlreadyResizedHits    int
	IconLazyCount             int
	MaxTotal                  MaxTiming
	MaxDefaultActions         MaxTiming
	MaxPolish                 MaxTiming
	MaxResultIcon             MaxTiming
	MaxIconConvert            MaxTiming
	MaxScore                  MaxTiming
	MaxCache                  MaxTiming
	MaxTranslationPreviewMeta MaxTiming
}

// Add accumulates one polished result into the aggregate.
func (a *ResultsPolishAggregate) Add(result ResultIdentity, defaultActionsUs int64, recordPluginElapsedUs int64, resultTotalUs int64, polish ResultPolishTiming) {
	a.Count++
	a.DefaultActionsUs += defaultActionsUs
	a.RecordPluginElapsedUs += recordPluginElapsedUs
	a.ResultTotalUs += resultTotalUs
	a.PolishUs += polish.TotalCostUs
	a.ActionSetupUs += polish.ActionSetupCostUs
	a.ActionDefaultsUs += polish.ActionDefaultsCostUs
	a.ActionIconUs += polish.ActionIconCostUs
	a.ActionCallbackUs += polish.ActionCallbackCostUs
	a.ActionIconCount += polish.ActionIconCount
	a.VisualUs += polish.VisualCostUs
	a.ResultIconUs += polish.ResultIconCostUs
	a.DragUs += polish.DragCostUs
	a.TailIconUs += polish.TailIconCostUs
	a.TailImageCount += polish.TailImageCount
	a.PreviewUs += polish.PreviewCostUs
	a.PreviewDefaultUs += polish.PreviewDefaultCostUs
	a.PreviewNormalizeUs += polish.PreviewNormalizeCostUs
	a.TranslationUs += polish.TranslationCostUs
	a.TranslationTextUs += polish.TranslationTextCostUs
	a.TranslationTailUs += polish.TranslationTailCostUs
	a.TranslationPreviewMetaUs += polish.TranslationPreviewMetaCostUs
	a.TranslationActionUs += polish.TranslationActionCostUs
	a.TranslationPreviewDataUs += polish.TranslationPreviewDataCostUs
	a.TranslationGroupUs += polish.TranslationGroupCostUs
	a.ActionNormalizeUs += polish.ActionNormalizeCostUs
	a.ActionDefaultUs += polish.ActionDefaultCostUs
	a.ActionSortUs += polish.ActionSortCostUs
	a.HotkeyNormalizeUs += polish.HotkeyNormalizeCostUs
	a.PreviewRemoteUs += polish.PreviewRemoteCostUs
	a.PreviewGlobalUs += polish.PreviewGlobalCostUs
	a.PreviewWrapUs += polish.PreviewWrapCostUs
	a.ScoreUs += polish.ScoreCostUs
	a.ScoreFeatureUs += polish.ScoreFeatureCostUs
	a.AutoScoreUs += polish.AutoScoreCostUs
	a.FavoriteUs += polish.FavoriteCostUs
	a.DevScoreTailUs += polish.DevScoreTailCostUs
	a.CacheUs += polish.CacheCostUs
	a.PreviewDataLen += polish.PreviewDataLen
	if polish.IconConversionSet {
		a.IconConvertCount++
		a.IconConvertUs += polish.IconConversion.TotalCostUs
		a.IconFileIconUs += polish.IconConversion.FileIconCostUs
		a.IconRelativeUs += polish.IconConversion.RelativeCostUs
		a.IconSvgCheckUs += polish.IconConversion.SvgCheckCostUs
		a.IconCacheUs += polish.IconConversion.CacheCostUs
		a.IconLazyCheckUs += polish.IconConversion.LazyCheckCostUs
		a.IconCropUs += polish.IconConversion.CropCostUs
		a.IconResizeUs += polish.IconConversion.ResizeCostUs
		if polish.IconConversion.CacheHit {
			a.IconCacheHits++
		}
		if polish.IconConversion.CacheSource == "already_resized" {
			a.IconAlreadyResizedHits++
		}
		if polish.IconConversion.Lazy {
			a.IconLazyCount++
		}
		a.MaxIconConvert.Record(polish.IconConversion.TotalCostUs, result)
	}
	a.MaxTotal.Record(resultTotalUs, result)
	a.MaxDefaultActions.Record(defaultActionsUs, result)
	a.MaxPolish.Record(polish.TotalCostUs, result)
	a.MaxResultIcon.Record(polish.ResultIconCostUs, result)
	a.MaxScore.Record(polish.ScoreCostUs, result)
	a.MaxCache.Record(polish.CacheCostUs, result)
	a.MaxTranslationPreviewMeta.Record(polish.TranslationPreviewMetaCostUs, result)
}

// Log writes the aggregate polish diagnostics for one plugin response.
func (a ResultsPolishAggregate) Log(ctx context.Context, queryId string, pluginLabel string) {
	if a.Count == 0 {
		return
	}

	tracker := New("polish_results_aggregate")
	if !tracker.Enabled() {
		return
	}

	tracker.SetRawString("queryId", queryId)
	tracker.SetRawString("plugin", pluginLabel)
	tracker.SetInt("resultCount", a.Count)
	tracker.SetInt64("resultTotalUs", a.ResultTotalUs)
	tracker.SetInt64("defaultActionsUs", a.DefaultActionsUs)
	tracker.SetInt64("recordPluginElapsedUs", a.RecordPluginElapsedUs)
	tracker.SetInt64("polishUs", a.PolishUs)
	tracker.SetInt64("actionSetupUs", a.ActionSetupUs)
	tracker.SetInt64("actionDefaultsUs", a.ActionDefaultsUs)
	tracker.SetInt64("actionIconUs", a.ActionIconUs)
	tracker.SetInt64("actionCallbackUs", a.ActionCallbackUs)
	tracker.SetInt("actionIconCount", a.ActionIconCount)
	tracker.SetInt64("visualUs", a.VisualUs)
	tracker.SetInt64("resultIconUs", a.ResultIconUs)
	tracker.SetInt64("dragUs", a.DragUs)
	tracker.SetInt64("tailIconUs", a.TailIconUs)
	tracker.SetInt("tailImageCount", a.TailImageCount)
	tracker.SetInt64("previewUs", a.PreviewUs)
	tracker.SetInt64("previewDefaultUs", a.PreviewDefaultUs)
	tracker.SetInt64("previewNormalizeUs", a.PreviewNormalizeUs)
	tracker.SetInt64("translationUs", a.TranslationUs)
	tracker.SetInt64("translationTextUs", a.TranslationTextUs)
	tracker.SetInt64("translationTailUs", a.TranslationTailUs)
	tracker.SetInt64("translationPreviewMetaUs", a.TranslationPreviewMetaUs)
	tracker.SetInt64("translationActionUs", a.TranslationActionUs)
	tracker.SetInt64("translationPreviewDataUs", a.TranslationPreviewDataUs)
	tracker.SetInt64("translationGroupUs", a.TranslationGroupUs)
	tracker.SetInt64("actionNormalizeUs", a.ActionNormalizeUs)
	tracker.SetInt64("actionDefaultUs", a.ActionDefaultUs)
	tracker.SetInt64("actionSortUs", a.ActionSortUs)
	tracker.SetInt64("hotkeyNormalizeUs", a.HotkeyNormalizeUs)
	tracker.SetInt64("previewRemoteUs", a.PreviewRemoteUs)
	tracker.SetInt64("previewGlobalUs", a.PreviewGlobalUs)
	tracker.SetInt64("previewWrapUs", a.PreviewWrapUs)
	tracker.SetInt64("scoreUs", a.ScoreUs)
	tracker.SetInt64("scoreFeatureUs", a.ScoreFeatureUs)
	tracker.SetInt64("autoScoreUs", a.AutoScoreUs)
	tracker.SetInt64("favoriteUs", a.FavoriteUs)
	tracker.SetInt64("devScoreTailUs", a.DevScoreTailUs)
	tracker.SetInt64("cacheUs", a.CacheUs)
	tracker.SetInt("previewDataLen", a.PreviewDataLen)
	tracker.SetInt("iconConvertCount", a.IconConvertCount)
	tracker.SetInt64("iconConvertUs", a.IconConvertUs)
	tracker.SetInt64("iconFileIconUs", a.IconFileIconUs)
	tracker.SetInt64("iconRelativeUs", a.IconRelativeUs)
	tracker.SetInt64("iconSvgCheckUs", a.IconSvgCheckUs)
	tracker.SetInt64("iconCacheUs", a.IconCacheUs)
	tracker.SetInt64("iconLazyCheckUs", a.IconLazyCheckUs)
	tracker.SetInt64("iconCropUs", a.IconCropUs)
	tracker.SetInt64("iconResizeUs", a.IconResizeUs)
	tracker.SetInt("iconCacheHits", a.IconCacheHits)
	tracker.SetInt("iconAlreadyResizedHits", a.IconAlreadyResizedHits)
	tracker.SetInt("iconLazyCount", a.IconLazyCount)
	addMaxFields(tracker, "maxTotal", a.MaxTotal, true)
	addMaxFields(tracker, "maxDefaultActions", a.MaxDefaultActions, false)
	addMaxFields(tracker, "maxPolish", a.MaxPolish, false)
	addMaxFields(tracker, "maxResultIcon", a.MaxResultIcon, false)
	addMaxFields(tracker, "maxIconConvert", a.MaxIconConvert, false)
	addMaxFields(tracker, "maxScore", a.MaxScore, false)
	addMaxFields(tracker, "maxCache", a.MaxCache, false)
	addMaxFields(tracker, "maxTranslationPreviewMeta", a.MaxTranslationPreviewMeta, false)
	tracker.Log(ctx)
}

func addMaxFields(tracker *TimeTracker, prefix string, max MaxTiming, includeId bool) {
	tracker.SetInt64(prefix+"Us", max.CostUs)
	tracker.SetInt(prefix+"Index", max.Result.Index)
	if includeId {
		tracker.SetRawString(prefix+"Id", max.Result.Id)
	}
	tracker.SetString(prefix+"Title", max.Result.Title)
}
