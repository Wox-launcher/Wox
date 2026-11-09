package launcher

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const (
	staleQueryResultsDuration = 200 * time.Millisecond
	queryLoadingDelay         = 500 * time.Millisecond
)

func (a *App) refinementViewProps(snapshot viewSnapshot, width, height, imageScale float32) launcherview.RefinementsProps {
	fallback := a.language.translate("i18n:ui_query_refinement_filters")
	if strings.HasPrefix(fallback, "ui query refinement") || fallback == "" {
		fallback = "Filters"
	}
	groups := make([]launcherview.RefinementGroup, 0, len(snapshot.refinements))
	for _, refinement := range snapshot.refinements {
		selected := splitRefinementValues(snapshot.refinementValues[refinement.ID])
		group := launcherview.RefinementGroup{Title: a.language.translate(refinement.Title), Hotkey: strings.Join(formatHotkeyLabels(refinement.Hotkey), "+")}
		if refinementUsesDropdown(refinement) {
			group.ID = refinement.ID
			group.Current = a.refinementSelectionLabel(refinement, selected)
			group.Expanded = refinement.ID == a.resultList.refinementPicker.id
			group.OnTap = func() { a.toggleRefinementPicker(refinement.ID) }
			group.OnKey = func(event woxui.KeyEvent) bool { return a.onRefinementGroupKey(refinement.ID, event) }
			groups = append(groups, group)
			continue
		}
		options := refinement.Options
		if len(options) == 0 {
			value := "true"
			if len(refinement.DefaultValue) > 0 && refinement.DefaultValue[0] != "" {
				value = refinement.DefaultValue[0]
			}
			options = []queryRefinementOption{{Value: value, Title: refinement.Title}}
		}
		converted := make([]launcherview.RefinementOption, 0, len(options))
		for _, option := range options {
			refinementID := refinement.ID
			converted = append(converted, launcherview.RefinementOption{
				Value: option.Value, Label: a.language.translate(option.Title), Count: option.Count, Icon: a.imageForSize(option.Icon, physicalImageSize(int(snapshot.densityMetrics.scaled(16)), imageScale)),
				Selected: slices.Contains(selected, option.Value),
				OnTap:    func() { a.selectRefinementOption(refinementID, option.Value) },
			})
		}
		hotkey := strings.Join(formatHotkeyLabels(refinement.Hotkey), "+")
		if !strings.Contains(hotkey, "+") {
			hotkey = ""
		}
		group.Hotkey, group.Options = hotkey, converted
		groups = append(groups, group)
	}
	props := launcherview.RefinementsProps{
		Revision: snapshot.refinementsRevision,
		Width:    width, Height: height, Theme: snapshot.palette.componentTheme(), Window: a.window, DensityScale: snapshot.densityMetrics.scale,
		SummaryLabels: a.refinementSummaryLabels(snapshot), DefaultLabel: fallback, Open: snapshot.refinementOpen,
		Tooltip: a.refinementToggleTooltip(), Groups: groups, OnToggle: func() { a.toggleRefinementBar() },
		OnTooltip: a.setRefinementTooltip,
		PickerID:  a.resultList.refinementPicker.id,
	}
	if len(props.SummaryLabels) > 0 {
		props.ClearLabel = a.language.translate("i18n:ui_query_refinement_clear")
		props.OnClear = a.clearRefinementSelections
	}
	if width > 0 && a.resultList.refinementDefault.scope.PluginID != "" {
		saved := a.resultList.refinementDefault.matches(snapshot.refinementValues)
		label := "i18n:ui_query_refinement_default_save"
		if saved {
			label = "i18n:ui_query_refinement_default_remove"
		}
		props.DefaultButton = &launcherview.RefinementDefaultButtonProps{
			Label: a.language.translate(label), Hotkey: strings.Join(formatHotkeyLabels(refinementDefaultHotkey()), "+"),
			Saved: saved, Busy: a.resultList.refinementDefaultBusy || !a.canToggleRefinementDefault(),
			OnToggle: a.toggleRefinementDefault, OnTooltip: a.setRefinementDefaultTooltip,
		}
	}
	return props
}

// refinementToggleTooltip advertises the configured shortcut for the filter bar.
func (a *App) refinementToggleTooltip() string {
	hotkey := strings.Join(formatHotkeyLabels(a.refinementHotkey()), "+")
	text := a.language.translate("i18n:ui_query_refinement_filters_tooltip")
	if strings.HasPrefix(text, "ui query refinement") || text == "" {
		text = "Filter search results ({hotkey})"
	}
	return strings.ReplaceAll(text, "{hotkey}", hotkey)
}

// setRefinementTooltip anchors filter-button help to the launcher query accessory.
func (a *App) setRefinementTooltip(inside bool, text string, anchor woxui.Rect) {
	a.tooltips.setNativeHoverTooltip(&a.resultList.refinementTooltipRevision, "go-ui-refinement", "update refinement tooltip", inside, text, anchor, "top", func() *woxui.Window { return a.window })
}

func (a *App) buildRefinementToggle(snapshot viewSnapshot, imageScale float32) woxwidget.Widget {
	return launcherview.RefinementToggleBoundary(a.refinementViewProps(snapshot, 0, 0, imageScale))
}

func (a *App) refinementToggleWidth(snapshot viewSnapshot, imageScale float32) float32 {
	return launcherview.RefinementToggleWidth(a.refinementViewProps(snapshot, 0, 0, imageScale))
}

func (a *App) buildRefinementBar(snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	return launcherview.RefinementsBoundary(a.refinementViewProps(snapshot, width, height, imageScale))
}

// refinementSummaryLabels gives each active filter group one slot in the header.
func (a *App) refinementSummaryLabels(snapshot viewSnapshot) []string {
	labels := make([]string, 0, len(snapshot.refinements))
	for _, refinement := range snapshot.refinements {
		selected := splitRefinementValues(snapshot.refinementValues[refinement.ID])
		if len(selected) == 0 || sameStringSet(selected, refinement.DefaultValue) {
			continue
		}
		label := a.refinementSelectionLabel(refinement, selected)
		if refinement.Type == plugin.QueryRefinementTypeToggle && len(refinement.Options) == 0 {
			label = a.language.translate(refinement.Title)
		}
		if label != "" {
			labels = append(labels, label)
		}
	}
	return labels
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range left {
		if !slices.Contains(right, value) {
			return false
		}
	}
	return true
}

// applyRefinementsLocked replaces query-scoped controls and materializes their normalized defaults.
func (a *App) applyRefinementsLocked(refinements []queryRefinement) {
	valid := make([]queryRefinement, 0, len(refinements))
	values := make(map[string]string, len(refinements))
	for _, refinement := range refinements {
		if refinement.ID == "" || refinement.Type == "" {
			continue
		}
		value, hasSelection := a.querySession.query.QueryRefinements[refinement.ID]
		selected := splitRefinementValues(value)
		refinement.DefaultValue = normalizeRefinementValues(refinement, nil)
		// An explicitly empty multi-select must not pick its defaults again after a query refresh.
		if !hasSelection || len(selected) > 0 || refinement.Type != plugin.QueryRefinementTypeMultiSelect {
			selected = normalizeRefinementValues(refinement, selected)
		}
		if len(selected) > 0 || (hasSelection && refinement.Type == plugin.QueryRefinementTypeMultiSelect) {
			values[refinement.ID] = strings.Join(selected, ",")
		}
		valid = append(valid, refinement)
	}
	a.resultList.refinements = valid
	a.resultList.refinementsSectionRevision++
	a.querySession.query.QueryRefinements = values
	a.resultList.refinementScope = a.querySession.queryScopeIdentityLocked(a.querySession.query.QueryText)
	if len(valid) == 0 {
		a.resultList.refinementOpen = false
	}
	a.reconcileRefinementPicker()
}

// applyQueryTextChangeLocked starts a new query while retaining controls only inside their plugin scope.
func (a *App) applyQueryTextChangeLocked(text string) {
	text = a.querySession.updateQueryHintText(text)
	a.querySession.canRecallHistory = false
	a.querySession.reuseCompletionHintLocked(text)
	nextScope := a.querySession.queryScopeIdentityLocked(text)
	preservePluginLayout := a.shouldPreservePluginLayoutLocked(text)
	if a.resultList.refinementScope != "" && nextScope != a.resultList.refinementScope {
		a.resultList.clearRefinements()
		a.querySession.query.QueryRefinements = map[string]string{}
	}
	if a.querySession.query.QueryText == "" {
		a.attention.attentionQueryWasGlobal = true
	}
	a.querySession.query.QueryText = text
	a.beginQueryGenerationLocked()
	a.resultList.resultScrollDetached = false
	a.querySession.beginQueryTransitionLocked(preservePluginLayout)
	// Preserve the visible global accessory until the backend classifies the new query.
	a.glance.stopGlanceLocked(false)
	a.actions.clearActionPanelStateLocked()
	a.chat.chatFullscreen = false
	a.webView.clearWebViewPreviewModeLocked()
}

// shouldPreservePluginLayoutLocked keeps confirmed plugin chrome stable while the same scope remains active.
func (a *App) shouldPreservePluginLayoutLocked(nextText string) bool {
	if a.querySession.query.QueryType != "input" || !a.querySession.queryContextKnown || a.querySession.queryContext.IsGlobalQuery {
		return false
	}
	if len(a.querySession.query.QueryScope.Plugins) > 0 {
		return a.querySession.queryScopeIdentityLocked(a.querySession.query.QueryText) == a.querySession.queryScopeIdentityLocked(nextText)
	}
	return a.querySession.queryContext.PluginID != "" &&
		refinementQueryScope(a.querySession.query.QueryText) == refinementQueryScope(nextText)
}

// queryScopeIdentityLocked returns the stable scope key for refinements and layout retention.
func (a *queryController) queryScopeIdentityLocked(text string) string {
	if len(a.query.QueryScope.Plugins) > 0 {
		return toCoreQueryScope(a.query.QueryScope).Identity()
	}
	return refinementQueryScope(text)
}

// beginQueryTransitionLocked gives fast query responses time to replace the visible snapshot without an empty frame.
func (a *queryController) beginQueryTransitionLocked(preservePluginLayout bool) {
	a.releaseRefreshResultHoldLocked()
	a.resetQueryTransitionLocked()
	a.resetQueryLoadingLocked()
	if a.visible() && len(a.resultList.results) > 0 && (a.query.QueryText != "" || len(a.query.QueryScope.Plugins) > 0 || a.startPage() == "mru") {
		queryID := a.query.QueryID
		a.queryTransitionTimer = time.AfterFunc(staleQueryResultsDuration, func() {
			if err := a.runOnUI("show pending query results", func() {
				a.showPendingQueryResults(queryID, preservePluginLayout)
			}); err != nil {
				log.Printf("dispatch pending query results: %v", err)
			}
		})
	} else {
		a.resultList.results = nil
		a.resultList.resultsSectionRevision++
		a.resultList.resultsQueryID = ""
		a.resultList.selected = -1
		if !preservePluginLayout {
			a.resultList.layout = queryLayout{}
		}
	}
}

// startQueryLoadingLocked starts Flutter's delay for every shared query entry point.
func (a *queryController) startQueryLoadingLocked() {
	a.resetQueryLoadingLocked()
	if a.query.QueryText == "" && len(a.query.QueryScope.Plugins) == 0 {
		return
	}
	queryID := a.query.QueryID
	a.queryLoadingTimer = time.AfterFunc(queryLoadingDelay, func() {
		if err := a.runOnUI("show query loading", func() { a.showQueryLoading(queryID) }); err != nil {
			log.Printf("dispatch query loading: %v", err)
		}
	})
}

// resetQueryLoadingLocked cancels both the delayed trigger and any visible loading state.
func (a *queryController) resetQueryLoadingLocked() {
	if a.queryLoadingTimer != nil {
		a.queryLoadingTimer.Stop()
		a.queryLoadingTimer = nil
	}
	wasLoading := a.queryLoading
	a.queryLoading = false
	if wasLoading && a.host() != nil {
		a.host().InvalidateBoundary(launcherview.LauncherQueryLoadingBoundaryKey)
	}
}

// showQueryLoading exposes the spinner only while the same query still has no result snapshot.
func (a *queryController) showQueryLoading(queryID string) {
	queryReturned := a.resultList.resultsQueryID == queryID && (len(a.resultList.results) > 0 || a.queryComplete)
	if a.isDestroyed() || a.query.QueryID != queryID || queryReturned {
		return
	}
	a.queryLoadingTimer = nil
	a.queryLoading = true
	if a.window() != nil {
		_ = a.window().Invalidate()
	}
}

func (a *queryController) resetQueryTransitionLocked() {
	if a.queryTransitionTimer != nil {
		a.queryTransitionTimer.Stop()
		a.queryTransitionTimer = nil
	}
}

// showPendingQueryResults clears stale content without shrinking the window while the current query is still waiting.
func (a *queryController) showPendingQueryResults(queryID string, preservePluginLayout bool) {
	if a.isDestroyed() || a.query.QueryID != queryID || a.resultList.resultsQueryID == queryID {
		return
	}
	a.queryTransitionTimer = nil
	a.resultList.results = nil
	a.resultList.resultsSectionRevision++
	a.resultList.resultsQueryID = ""
	a.resultList.selected = -1
	a.resultList.resultScrollDetached = false
	if !preservePluginLayout {
		a.resultList.layout = queryLayout{}
	}
	a.reconcileSelectedPreview()
	_ = a.window().Invalidate()
}

func (a *App) toggleRefinementBar() bool {
	if len(a.resultList.refinements) == 0 || a.show.HideQueryBox {
		return false
	}
	a.resultList.refinementOpen = !a.resultList.refinementOpen
	if !a.resultList.refinementOpen {
		a.resultList.refinementPicker = refinementPicker{}
		a.resultList.refinementFocusPending = launcherview.LauncherQueryInputKey
	} else if index := slices.IndexFunc(a.resultList.refinements, refinementUsesDropdown); index >= 0 {
		a.resultList.refinementFocusPending = launcherview.RefinementGroupKey(a.resultList.refinements[index].ID)
	}
	if a.window == nil {
		return true
	}
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query refinements: %v", err)
	}
	_ = a.window.Invalidate()
	return true
}

// onRefinementHotkey retains blind cycling until the user shows the filter bar.
func (a *App) onRefinementHotkey(event woxui.KeyEvent) bool {
	if a.show.HideQueryBox {
		return false
	}
	for index := range a.resultList.refinements {
		refinement := &a.resultList.refinements[index]
		if !hotkeyMatches(refinement.Hotkey, event) {
			continue
		}
		if a.resultList.refinementOpen && refinementUsesDropdown(*refinement) {
			if !event.Repeat {
				a.toggleRefinementPicker(refinement.ID)
			}
			return true
		}
		a.updateRefinementSelection(refinement, nextRefinementHotkeyValues(*refinement, splitRefinementValues(a.querySession.query.QueryRefinements[refinement.ID])))
		return true
	}
	return false
}

// nextRefinementHotkeyValues mirrors the cycling and bulk-toggle behavior of the visible controls.
func nextRefinementHotkeyValues(refinement queryRefinement, selected []string) []string {
	optionValues := make([]string, 0, len(refinement.Options))
	for _, option := range refinement.Options {
		if option.Value != "" {
			optionValues = append(optionValues, option.Value)
		}
	}
	if refinement.Type != plugin.QueryRefinementTypeMultiSelect {
		selected = normalizeRefinementValues(refinement, selected)
	}

	switch refinement.Type {
	case "toggle":
		toggleValue := "true"
		hasDefault := false
		for _, value := range refinement.DefaultValue {
			if value != "" {
				toggleValue = value
				hasDefault = true
				break
			}
		}
		if !hasDefault && len(optionValues) > 0 {
			toggleValue = optionValues[0]
		}
		if slices.Contains(selected, toggleValue) {
			return nil
		}
		return []string{toggleValue}
	case "multiSelect":
		if len(optionValues) > 0 {
			allSelected := true
			for _, value := range optionValues {
				if !slices.Contains(selected, value) {
					allSelected = false
					break
				}
			}
			if allSelected {
				return nil
			}
		}
		return optionValues
	}

	if len(optionValues) == 0 {
		return nil
	}
	currentIndex := 0
	if len(selected) > 0 {
		if selectedIndex := slices.Index(optionValues, selected[0]); selectedIndex >= 0 {
			currentIndex = selectedIndex
		}
	}
	return []string{optionValues[(currentIndex+1)%len(optionValues)]}
}

func (a *App) selectRefinementOption(refinementID, value string) {
	refinement := a.refinementByID(refinementID)
	if refinement == nil || value == "" {
		return
	}
	selected := splitRefinementValues(a.querySession.query.QueryRefinements[refinementID])
	switch refinement.Type {
	case plugin.QueryRefinementTypeMultiSelect, plugin.QueryRefinementTypeToggle:
		if slices.Contains(selected, value) {
			selected = slices.DeleteFunc(selected, func(candidate string) bool { return candidate == value })
		} else {
			selected = append(selected, value)
		}
	default:
		selected = []string{value}
	}
	a.updateRefinementSelection(refinement, selected)
}

// updateRefinementSelection routes keyboard and pointer changes through the same query refresh path.
func (a *App) updateRefinementSelection(refinement *queryRefinement, selected []string) {
	multiSelect := refinement.Type == plugin.QueryRefinementTypeMultiSelect
	if !multiSelect || len(selected) > 0 {
		selected = normalizeRefinementValues(*refinement, selected)
	}
	if len(selected) == 0 && !multiSelect {
		delete(a.querySession.query.QueryRefinements, refinement.ID)
	} else {
		if a.querySession.query.QueryRefinements == nil {
			a.querySession.query.QueryRefinements = make(map[string]string)
		}
		a.querySession.query.QueryRefinements[refinement.ID] = strings.Join(selected, ",")
	}
	a.refreshRefinementResults()
}

// clearRefinementSelections restores the same plugin defaults the summary treats as inactive.
func (a *App) clearRefinementSelections() {
	values := make(map[string]string, len(a.resultList.refinements))
	for _, refinement := range a.resultList.refinements {
		if len(refinement.DefaultValue) > 0 {
			values[refinement.ID] = strings.Join(refinement.DefaultValue, ",")
		}
	}
	a.querySession.query.QueryRefinements = values
	a.dismissRefinementPicker()
	a.resultList.refinementFocusPending = launcherview.LauncherQueryInputKey
	a.refreshRefinementResults()
}

// refreshRefinementResults starts one query after the entire selection change is applied.
func (a *App) refreshRefinementResults() {
	preservePluginLayout := a.shouldPreservePluginLayoutLocked(a.querySession.query.QueryText)
	a.resultList.refinementsSectionRevision++
	a.beginQueryGenerationLocked()
	a.querySession.completionHint = nil
	a.resultList.resultScrollDetached = false
	a.querySession.beginQueryTransitionLocked(preservePluginLayout)
	a.glance.stopGlanceLocked(true)
	a.actions.clearActionPanelStateLocked()
	a.chat.chatFullscreen = false
	a.webView.clearWebViewPreviewModeLocked()
	a.previews.reconcileSelectedPreview()
	if err := a.sendCurrentQuery(); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("send query after refinement change: %v", err))
	}
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

func normalizeRefinementValues(refinement queryRefinement, values []string) []string {
	allowed := make(map[string]bool, len(refinement.Options))
	for _, option := range refinement.Options {
		if option.Value != "" {
			allowed[option.Value] = true
		}
	}
	filter := func(candidates []string) []string {
		result := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			if candidate == "" || (len(allowed) > 0 && !allowed[candidate]) || slices.Contains(result, candidate) {
				continue
			}
			result = append(result, candidate)
		}
		return result
	}
	normalized := filter(values)
	if len(normalized) == 0 && len(values) == 0 {
		normalized = filter(refinement.DefaultValue)
	}
	if (refinement.Type == "singleSelect" || refinement.Type == "sort") && len(normalized) == 0 && len(refinement.Options) > 0 {
		normalized = []string{refinement.Options[0].Value}
	}
	if (refinement.Type == "singleSelect" || refinement.Type == "sort") && len(normalized) > 1 {
		normalized = normalized[:1]
	}
	return normalized
}

func splitRefinementValues(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	result := parts[:0]
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result
}

func refinementQueryScope(text string) string {
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}
