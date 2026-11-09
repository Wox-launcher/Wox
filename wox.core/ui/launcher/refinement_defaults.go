package launcher

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/notifier"
)

type refinementDefaultState struct {
	scope       contract.RefinementScope
	queryID     string
	savedValues map[string]string
}

// matches compares selected values, allowing different multi-select order and omitted empty groups.
func (s refinementDefaultState) matches(values map[string]string) bool {
	if s.savedValues == nil {
		return false
	}
	for id, saved := range s.savedValues {
		current := values[id]
		if saved != current && !sameStringSet(splitRefinementValues(saved), splitRefinementValues(current)) {
			return false
		}
	}
	for id, current := range values {
		if _, saved := s.savedValues[id]; !saved && current != "" {
			return false
		}
	}
	return true
}

func refinementDefaultHotkey() string { return primaryHotkey("shift+d") }

// applyRefinementDefaults keeps the UI aligned with the values core used for this query.
// A stale saved option triggers one corrected query before publishing those results.
func (a *App) applyRefinementDefaults(queryID string, context *queryContext, refinements []queryRefinement, complete bool) bool {
	state := &a.resultList.refinementDefault
	if context == nil || state.queryID == queryID || (len(refinements) == 0 && !complete) {
		return false
	}
	incoming := context.refinementDefaults
	if len(refinements) == 0 {
		*state = refinementDefaultState{}
		return false
	}
	if state.scope != incoming.Scope {
		*state = refinementDefaultState{scope: incoming.Scope, savedValues: incoming.SavedValues}
	}
	state.queryID = queryID
	if incoming.AppliedValues == nil {
		return false
	}
	a.querySession.query.QueryRefinements = maps.Clone(incoming.AppliedValues)
	if !incoming.Restored {
		return false
	}
	validated := validateRestoredRefinements(refinements, incoming.AppliedValues)
	if maps.Equal(validated, incoming.AppliedValues) {
		return false
	}
	a.querySession.query.QueryRefinements = validated
	a.resultList.refinementScope = a.querySession.queryScopeIdentityLocked(a.querySession.query.QueryText)
	a.beginQueryGenerationLocked()
	if err := a.sendCurrentQuery(); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("query with validated refinement defaults: %v", err))
	}
	return true
}

// validateRestoredRefinements drops missing groups and falls back to the advertised defaults when options have changed.
func validateRestoredRefinements(refinements []queryRefinement, saved map[string]string) map[string]string {
	valid := make(map[string]string, len(saved))
	for _, refinement := range refinements {
		raw, exists := saved[refinement.ID]
		if !exists {
			continue
		}
		if raw == "" {
			valid[refinement.ID] = raw
			continue
		}
		values := splitRefinementValues(raw)
		if len(refinement.Options) > 0 {
			values = slices.DeleteFunc(values, func(value string) bool {
				return !slices.ContainsFunc(refinement.Options, func(option queryRefinementOption) bool { return option.Value == value })
			})
		}
		values = normalizeRefinementValues(refinement, values)
		if len(values) > 0 {
			valid[refinement.ID] = strings.Join(values, ",")
		}
	}
	return valid
}

func (a *App) canToggleRefinementDefault() bool {
	return a.resultList.refinementDefault.scope.PluginID != "" && len(a.resultList.refinements) > 0 &&
		a.resultList.refinementDefault.queryID == a.querySession.query.QueryID && a.resultList.refinementOpen && !a.show.HideQueryBox
}

func (a *App) onRefinementDefaultKey(event woxui.KeyEvent) bool {
	if !a.canToggleRefinementDefault() || !hotkeyMatches(refinementDefaultHotkey(), event) {
		return false
	}
	if !event.Repeat {
		a.toggleRefinementDefault()
	}
	return true
}

// toggleRefinementDefault removes a matching default or replaces it with the current selections.
func (a *App) toggleRefinementDefault() {
	service, ok := a.services.(contract.RefinementDefaultServices)
	if !ok || !a.canToggleRefinementDefault() || a.resultList.refinementDefaultBusy {
		return
	}
	state := a.resultList.refinementDefault
	var values map[string]string
	if !state.matches(a.querySession.query.QueryRefinements) {
		values = make(map[string]string, len(a.querySession.query.QueryRefinements))
		maps.Copy(values, a.querySession.query.QueryRefinements)
	}
	a.resultList.refinementDefaultBusy = true
	a.setRefinementDefaultTooltip(false, "", woxui.Rect{})
	if a.window != nil {
		_ = a.window.Invalidate()
	}
	util.Go(a.lifecycleCtx, "update refinement default", func() {
		err := service.SetRefinementDefault(a.lifecycleCtx, state.scope, values)
		if dispatchErr := a.runOnUI("finish updating refinement default", func() {
			a.resultList.refinementDefaultBusy = false
			if err == nil && a.resultList.refinementDefault.scope == state.scope {
				a.resultList.refinementDefault.savedValues = values
			}
			if err != nil {
				util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("update refinement default: %v", err))
				notifier.ShowToast(notifier.Toast{Message: a.language.translate("i18n:ui_query_refinement_default_failed")})
			}
			if !a.isDestroyed() && a.window != nil {
				_ = a.window.Invalidate()
			}
		}); dispatchErr != nil {
			util.GetLogger().Warn(a.lifecycleCtx, dispatchErr.Error())
		}
	})
}

// setRefinementDefaultTooltip shows the current bookmark action with its shortcut.
func (a *App) setRefinementDefaultTooltip(inside bool, text string, anchor woxui.Rect) {
	a.tooltips.setNativeHoverTooltipWithHotkeys(&a.resultList.refinementDefaultTooltip, "go-ui-refinement-default",
		"update refinement default tooltip", inside, text, formatHotkeyLabels(refinementDefaultHotkey()), anchor, "top", func() *woxui.Window { return a.window })
}
