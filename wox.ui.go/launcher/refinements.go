package launcher

import (
	"log"
	"slices"
	"strings"
	"time"

	"github.com/Wox-launcher/wox.ui.go/coreclient"
)

const refinementBarHeight = 44

const staleQueryResultsDuration = 80 * time.Millisecond

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
	a.reuseCompletionHintLocked(text)
	nextScope := refinementQueryScope(text)
	if a.refinementScope != "" && nextScope != a.refinementScope {
		a.refinements = nil
		a.refinementOpen = false
		a.refinementScope = ""
		a.query.QueryRefinements = map[string]string{}
	}
	a.query.QueryText = text
	a.query.QueryID = coreclient.NewID()
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.resetQueryTransitionLocked()
	if text != "" && a.visible && len(a.results) > 0 {
		queryID := a.query.QueryID
		a.queryTransitionTimer = time.AfterFunc(staleQueryResultsDuration, func() {
			a.showPendingQueryResults(queryID)
		})
	} else {
		a.results = nil
		a.resultsQueryID = ""
		a.selected = -1
		a.layout = queryLayout{}
	}
	// Preserve the visible global accessory until the backend classifies the new query.
	a.stopGlanceLocked(false)
	a.actionPanel = false
	a.actionSelected = 0
	a.actionFilter = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.chatPreview = nil
	a.chatFullscreen = false
}

func (a *App) resetQueryTransitionLocked() {
	if a.queryTransitionTimer != nil {
		a.queryTransitionTimer.Stop()
		a.queryTransitionTimer = nil
	}
	a.pendingResults = false
}

// showPendingQueryResults clears stale content without shrinking the window while the current query is still waiting.
func (a *App) showPendingQueryResults(queryID string) {
	a.mu.Lock()
	if a.query.QueryID != queryID || a.resultsQueryID == queryID {
		a.mu.Unlock()
		return
	}
	a.queryTransitionTimer = nil
	a.pendingResults = true
	a.results = nil
	a.resultsQueryID = ""
	a.selected = -1
	a.layout = queryLayout{}
	a.mu.Unlock()
	a.deactivateTerminalPreview()
	a.deactivateWebViewPreview()
	_ = a.window.Invalidate()
}

func (a *App) toggleRefinementBar() bool {
	a.mu.Lock()
	if len(a.refinements) == 0 || a.show.HideQueryBox {
		a.mu.Unlock()
		return false
	}
	a.refinementOpen = !a.refinementOpen
	a.mu.Unlock()
	if err := a.applyWindowBounds(); err != nil {
		log.Printf("resize launcher for query refinements: %v", err)
	}
	_ = a.window.Invalidate()
	return true
}

func (a *App) selectRefinementOption(refinementID, value string) {
	a.mu.Lock()
	var refinement *queryRefinement
	for index := range a.refinements {
		if a.refinements[index].ID == refinementID {
			refinement = &a.refinements[index]
			break
		}
	}
	if refinement == nil || value == "" {
		a.mu.Unlock()
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
	selected = normalizeRefinementValues(*refinement, selected)
	if len(selected) == 0 {
		delete(a.query.QueryRefinements, refinementID)
	} else {
		a.query.QueryRefinements[refinementID] = strings.Join(selected, ",")
	}
	a.query.QueryID = coreclient.NewID()
	a.queryContext = queryContext{}
	a.queryContextKnown = false
	a.completionHint = nil
	a.resetQueryTransitionLocked()
	a.results = nil
	a.resultsQueryID = ""
	a.selected = -1
	a.layout = queryLayout{}
	a.stopGlanceLocked(true)
	a.actionPanel = false
	a.actionSelected = 0
	a.actionFilter = nil
	a.requirementForm = nil
	a.triggerConflict = nil
	a.themeEditor = nil
	a.chatPreview = nil
	a.chatFullscreen = false
	a.mu.Unlock()
	a.deactivateTerminalPreview()
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
