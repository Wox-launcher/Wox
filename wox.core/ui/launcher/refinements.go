package launcher

import (
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const staleQueryResultsDuration = 150 * time.Millisecond

func (a *App) refinementViewProps(snapshot viewSnapshot, width, height, imageScale float32) launcherview.RefinementsProps {
	fallback := a.translate("i18n:ui_query_refinement_filters")
	if strings.HasPrefix(fallback, "ui query refinement") || fallback == "" {
		fallback = "Filters"
	}
	groups := make([]launcherview.RefinementGroup, 0, len(snapshot.refinements))
	for _, refinement := range snapshot.refinements {
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
				Value: option.Value, Label: a.translate(option.Title), Count: option.Count, Icon: a.imageForSize(option.Icon, physicalImageSize(int(snapshot.densityMetrics.scaled(16)), imageScale)),
				Selected: slices.Contains(splitRefinementValues(snapshot.refinementValues[refinement.ID]), option.Value),
				OnTap:    func() { a.selectRefinementOption(refinementID, option.Value) },
			})
		}
		hotkey := strings.Join(formatHotkeyLabels(refinement.Hotkey), "+")
		if !strings.Contains(hotkey, "+") {
			hotkey = ""
		}
		groups = append(groups, launcherview.RefinementGroup{Title: a.translate(refinement.Title), Hotkey: hotkey, Options: converted})
	}
	return launcherview.RefinementsProps{
		Width: width, Height: height, Theme: snapshot.palette.componentTheme(), Window: a.window, DensityScale: snapshot.densityMetrics.scale,
		Summary: a.refinementSummary(snapshot, fallback), DefaultLabel: fallback, Open: snapshot.refinementOpen,
		Groups: groups, OnToggle: func() { a.toggleRefinementBar() },
	}
}

func (a *App) buildRefinementToggle(snapshot viewSnapshot, imageScale float32) woxwidget.Widget {
	return launcherview.RefinementToggle(a.refinementViewProps(snapshot, 0, 0, imageScale))
}

func (a *App) refinementToggleWidth(snapshot viewSnapshot, imageScale float32) float32 {
	return launcherview.RefinementToggleWidth(a.refinementViewProps(snapshot, 0, 0, imageScale))
}

func (a *App) buildRefinementBar(snapshot viewSnapshot, width, height, imageScale float32) woxwidget.Widget {
	return launcherview.RefinementsView(a.refinementViewProps(snapshot, width, height, imageScale))
}

func (a *App) refinementSummary(snapshot viewSnapshot, fallback string) string {
	labels := make([]string, 0, 2)
	activeControls := 0
	for _, refinement := range snapshot.refinements {
		selected := normalizeRefinementValues(refinement, splitRefinementValues(snapshot.refinementValues[refinement.ID]))
		defaults := normalizeRefinementValues(refinement, nil)
		if sameStringSet(selected, defaults) {
			continue
		}
		activeControls++
		for _, value := range selected {
			for _, option := range refinement.Options {
				if option.Value == value {
					labels = append(labels, a.translate(option.Title))
					break
				}
			}
			if len(labels) == 2 {
				break
			}
		}
	}
	if len(labels) == 0 {
		return fallback
	}
	label := strings.Join(labels, ", ")
	if activeControls > len(labels) {
		label += fmt.Sprintf(" +%d", activeControls-len(labels))
	}
	return label
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
		selected := splitRefinementValues(a.query.QueryRefinements[refinement.ID])
		selected = normalizeRefinementValues(refinement, selected)
		if len(selected) > 0 {
			values[refinement.ID] = strings.Join(selected, ",")
		}
		valid = append(valid, refinement)
	}
	a.refinements = valid
	a.query.QueryRefinements = values
	a.refinementScope = refinementQueryScope(a.query.QueryText)
	if len(valid) == 0 {
		a.refinementOpen = false
	}
}

// applyQueryTextChangeLocked starts a new query while retaining controls only inside their plugin scope.
func (a *App) applyQueryTextChangeLocked(text string) {
	a.canRecallHistory = false
	a.reuseCompletionHintLocked(text)
	nextScope := refinementQueryScope(text)
	if a.refinementScope != "" && nextScope != a.refinementScope {
		a.refinements = nil
		a.refinementOpen = false
		a.refinementScope = ""
		a.query.QueryRefinements = map[string]string{}
	}
	a.query.QueryText = text
	a.query.QueryID = newID()
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.resultScrollDetached = false
	a.beginQueryTransitionLocked()
	// Preserve the visible global accessory until the backend classifies the new query.
	a.stopGlanceLocked(false)
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.chatFullscreen = false
}

// beginQueryTransitionLocked gives fast query responses time to replace the visible snapshot without an empty frame.
func (a *App) beginQueryTransitionLocked() {
	a.resetQueryTransitionLocked()
	if a.query.QueryText != "" && a.visible && len(a.results) > 0 {
		queryID := a.query.QueryID
		a.queryTransitionTimer = time.AfterFunc(staleQueryResultsDuration, func() {
			if err := a.runOnUI("show pending query results", func() {
				a.showPendingQueryResults(queryID)
			}); err != nil {
				log.Printf("dispatch pending query results: %v", err)
			}
		})
	} else {
		a.results = nil
		a.resultsQueryID = ""
		a.selected = -1
		a.layout = queryLayout{}
	}
}

func (a *App) resetQueryTransitionLocked() {
	if a.queryTransitionTimer != nil {
		a.queryTransitionTimer.Stop()
		a.queryTransitionTimer = nil
	}
	if a.queryResizeTimer != nil {
		a.queryResizeTimer.Stop()
		a.queryResizeTimer = nil
	}
	a.queryResizeRevision++
}

// showPendingQueryResults clears stale content without shrinking the window while the current query is still waiting.
func (a *App) showPendingQueryResults(queryID string) {
	if a.query.QueryID != queryID || a.resultsQueryID == queryID {
		return
	}
	a.queryTransitionTimer = nil
	a.results = nil
	a.resultsQueryID = ""
	a.selected = -1
	a.resultScrollDetached = false
	a.layout = queryLayout{}
	a.reconcileSelectedPreview()
	_ = a.window.Invalidate()
}

func (a *App) toggleRefinementBar() bool {
	if len(a.refinements) == 0 || a.show.HideQueryBox {
		return false
	}
	a.refinementOpen = !a.refinementOpen
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query refinements: %v", err)
	}
	_ = a.window.Invalidate()
	return true
}

// onRefinementHotkey applies the first matching query-scoped refinement without moving query focus.
func (a *App) onRefinementHotkey(event woxui.KeyEvent) bool {
	if a.show.HideQueryBox {
		return false
	}
	for index := range a.refinements {
		refinement := &a.refinements[index]
		if !hotkeyMatches(refinement.Hotkey, event) {
			continue
		}
		a.updateRefinementSelection(refinement, nextRefinementHotkeyValues(*refinement, splitRefinementValues(a.query.QueryRefinements[refinement.ID])))
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
	selected = normalizeRefinementValues(refinement, selected)

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
	var refinement *queryRefinement
	for index := range a.refinements {
		if a.refinements[index].ID == refinementID {
			refinement = &a.refinements[index]
			break
		}
	}
	if refinement == nil || value == "" {
		return
	}
	selected := splitRefinementValues(a.query.QueryRefinements[refinementID])
	switch refinement.Type {
	case "multiSelect", "toggle":
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
	selected = normalizeRefinementValues(*refinement, selected)
	if len(selected) == 0 {
		delete(a.query.QueryRefinements, refinement.ID)
	} else {
		a.query.QueryRefinements[refinement.ID] = strings.Join(selected, ",")
	}
	a.query.QueryID = newID()
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.completionHint = nil
	a.resultScrollDetached = false
	a.beginQueryTransitionLocked()
	a.stopGlanceLocked(true)
	a.actionPanel = false
	a.actionSelected = 0
	a.actionSelectionKey = ""
	a.actionFilter = nil
	a.chatFullscreen = false
	a.reconcileSelectedPreview()
	if err := a.sendCurrentQuery(); err != nil {
		log.Printf("send query after refinement change: %v", err)
	}
	_ = a.window.Invalidate()
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
