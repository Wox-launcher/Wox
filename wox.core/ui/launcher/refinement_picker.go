package launcher

import (
	"cmp"
	"fmt"
	"slices"
	"strings"

	"wox/plugin"
	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

type refinementPicker struct {
	id, search, value string
	options           []int
	cursor            int
}

func refinementUsesDropdown(refinement queryRefinement) bool {
	return refinement.Type == plugin.QueryRefinementTypeSingleSelect || refinement.Type == plugin.QueryRefinementTypeMultiSelect
}

func (a *App) refinementByID(id string) *queryRefinement {
	for index := range a.resultList.refinements {
		if a.resultList.refinements[index].ID == id {
			return &a.resultList.refinements[index]
		}
	}
	return nil
}

// refinementSelectionLabel describes the current value in the collapsed dropdown.
func (a *App) refinementSelectionLabel(refinement queryRefinement, selected []string) string {
	switch len(selected) {
	case 0:
		return a.language.translate("i18n:ui_glance_none")
	case 1:
		for _, option := range refinement.Options {
			if option.Value == selected[0] {
				return firstNonEmpty(a.language.translate(option.Title), option.Value)
			}
		}
		return selected[0]
	default:
		return fmt.Sprintf(a.language.translate("i18n:ui_query_refinement_selected_count"), len(selected))
	}
}

// toggleRefinementPicker opens or switches one picker while leaving the main query intact.
func (a *App) toggleRefinementPicker(id string) {
	if a.resultList.refinementPicker.id == id {
		a.dismissRefinementPicker()
		return
	}
	refinement := a.refinementByID(id)
	if refinement == nil || !refinementUsesDropdown(*refinement) || !a.resultList.refinementOpen || a.show.HideQueryBox {
		return
	}
	a.actions.hideActionPanel()
	a.resultList.refinementFocusPending = ""
	a.resultList.refinementPicker = refinementPicker{id: id}
	a.filterRefinementPicker(refinement, "")
	a.refreshRefinementPickerWindow()
}

func (a *App) dismissRefinementPicker() {
	if a.resultList.refinementPicker.id == "" {
		return
	}
	a.resultList.refinementPicker = refinementPicker{}
	a.resultList.refinementFocusPending = launcherview.LauncherQueryInputKey
	a.refreshRefinementPickerWindow()
}

func (a *App) refreshRefinementPickerWindow() {
	if a.window == nil {
		return
	}
	_ = a.window.Invalidate()
	if err := a.applyWindowBounds(); err != nil {
		util.GetLogger().Warn(a.lifecycleCtx, fmt.Sprintf("resize launcher for refinement picker: %v", err))
	}
}

// reconcileRefinementPicker refreshes a live list against the latest plugin options, preserving its cursor by value.
func (a *App) reconcileRefinementPicker() {
	picker := &a.resultList.refinementPicker
	if picker.id == "" {
		return
	}
	refinement := a.refinementByID(picker.id)
	if !a.resultList.refinementOpen || refinement == nil || !refinementUsesDropdown(*refinement) {
		a.resultList.refinementPicker = refinementPicker{}
		a.resultList.refinementFocusPending = launcherview.LauncherQueryInputKey
		return
	}
	a.filterRefinementPicker(refinement, picker.value)
}

// filterRefinementPicker ranks translated labels and plugin-supplied aliases locally.
func (a *App) filterRefinementPicker(refinement *queryRefinement, retain string) {
	picker := &a.resultList.refinementPicker
	term := strings.TrimSpace(picker.search)
	type match struct {
		index int
		score int64
	}
	matches := make([]match, 0, len(refinement.Options))
	translations, usePinYin := a.language.translationSnapshot(), a.usePinYin()
	for index, option := range refinement.Options {
		if option.Value == "" {
			continue
		}
		if term == "" {
			matches = append(matches, match{index: index})
			continue
		}
		aliases := make([]string, 0, 1+len(option.Keywords))
		aliases = append(aliases, option.Value)
		aliases = append(aliases, option.Keywords...)
		score, found := actionFilterMatchScore(actionPanelEntry{Name: option.Title, SearchAliases: aliases}, term, translations, usePinYin)
		if found {
			matches = append(matches, match{index: index, score: score})
		}
	}
	if term != "" {
		slices.SortStableFunc(matches, func(left, right match) int {
			return cmp.Compare(right.score, left.score)
		})
	}
	picker.options = make([]int, len(matches))
	picker.cursor = -1
	picker.value = ""
	for index, item := range matches {
		picker.options[index] = item.index
		if index == 0 || refinement.Options[item.index].Value == retain {
			picker.cursor = index
			picker.value = refinement.Options[item.index].Value
		}
	}
}

// activateRefinementPickerOption commits one selection and leaves multi-select menus open.
func (a *App) activateRefinementPickerOption(id, value string) {
	refinement := a.refinementByID(id)
	if a.resultList.refinementPicker.id != id || refinement == nil ||
		!slices.ContainsFunc(refinement.Options, func(option queryRefinementOption) bool { return option.Value == value }) {
		return
	}
	if refinement.Type == plugin.QueryRefinementTypeSingleSelect {
		a.dismissRefinementPicker()
		if a.querySession.query.QueryRefinements[id] == value {
			return
		}
	}
	a.selectRefinementOption(id, value)
}

// onRefinementPickerKey gives navigation to the menu and leaves editing to its text field.
func (a *App) onRefinementPickerKey(event woxui.KeyEvent) bool {
	picker := &a.resultList.refinementPicker
	if !event.Down || event.Composing || picker.id == "" {
		return false
	}
	if hotkeyMatches(a.refinementHotkey(), event) {
		if !event.Repeat {
			a.toggleRefinementBar()
		}
		return true
	}
	if event.Modifiers != 0 {
		return a.onRefinementDefaultKey(event) || a.onRefinementHotkey(event)
	}
	switch event.Key {
	case woxui.KeyEscape:
		a.dismissRefinementPicker()
	case woxui.KeyArrowUp, woxui.KeyArrowDown:
		if len(picker.options) > 0 {
			delta := 1
			if event.Key == woxui.KeyArrowUp {
				delta = -1
			}
			picker.cursor = min(max(picker.cursor+delta, 0), len(picker.options)-1)
			if refinement := a.refinementByID(picker.id); refinement != nil {
				picker.value = refinement.Options[picker.options[picker.cursor]].Value
			}
			if a.window != nil {
				_ = a.window.Invalidate()
			}
		}
	case woxui.KeyEnter:
		if !event.Repeat && picker.value != "" {
			a.activateRefinementPickerOption(picker.id, picker.value)
		}
	default:
		return false
	}
	return true
}

// onRefinementGroupKey makes groups reachable even when a plugin does not assign shortcuts.
func (a *App) onRefinementGroupKey(id string, event woxui.KeyEvent) bool {
	if !event.Down || event.Composing || event.Modifiers != 0 || a.host == nil {
		return false
	}
	if event.Key == woxui.KeyEscape {
		a.host.RequestFocus(launcherview.LauncherQueryInputKey)
		return true
	}
	delta := 1
	switch event.Key {
	case woxui.KeyArrowLeft:
		delta = -1
	case woxui.KeyArrowRight:
	default:
		return false
	}
	refinements := a.resultList.refinements
	start := slices.IndexFunc(refinements, func(refinement queryRefinement) bool { return refinement.ID == id })
	for index := start + delta; index >= 0 && index < len(refinements); index += delta {
		if refinementUsesDropdown(refinements[index]) {
			a.host.RequestFocus(launcherview.RefinementGroupKey(refinements[index].ID))
			break
		}
	}
	return true
}

// buildRefinementPicker resolves the laid-out trigger, including horizontal scroll and bottom query placement.
func (a *App) buildRefinementPicker(snapshot viewSnapshot, contentBounds woxui.Rect, imageScale float32) (overlay woxwidget.Widget, anchor, popupBounds woxui.Rect) {
	picker := a.resultList.refinementPicker
	refinement := a.refinementByID(picker.id)
	if a.host == nil || picker.id == "" || refinement == nil {
		return nil, woxui.Rect{}, woxui.Rect{}
	}
	anchor, found := a.host.BoundsForKey(launcherview.RefinementGroupKey(picker.id))
	if !found {
		return nil, woxui.Rect{}, woxui.Rect{}
	}
	anchor.X -= contentBounds.X
	anchor.Y -= contentBounds.Y
	selected := splitRefinementValues(snapshot.refinementValues[refinement.ID])
	options := refinement.Options
	overlay, popupBounds = launcherview.RefinementPickerView(launcherview.RefinementPickerProps{
		ID: picker.id, Anchor: anchor, Theme: snapshot.palette.componentTheme(), Window: a.window, DensityScale: snapshot.densityMetrics.scale,
		Search: picker.search, SearchLabel: a.language.translate("i18n:ui_table_search"), EmptyLabel: a.language.translate("i18n:ui_no_matches"),
		Count: len(picker.options), Cursor: picker.cursor,
		OptionAt: func(index int) launcherview.RefinementOption {
			option := options[picker.options[index]]
			return launcherview.RefinementOption{
				Value: option.Value, Label: firstNonEmpty(a.language.translate(option.Title), option.Value), Count: option.Count,
				Icon:     a.imageForSize(option.Icon, physicalImageSize(int(snapshot.densityMetrics.scaled(14)), imageScale)),
				Selected: slices.Contains(selected, option.Value), OnTap: func() { a.activateRefinementPickerOption(picker.id, option.Value) },
			}
		},
		OnChanged: func(text string) {
			live := &a.resultList.refinementPicker
			if live.id != picker.id || live.search == text {
				return
			}
			if refinement := a.refinementByID(live.id); refinement != nil {
				live.search = text
				a.filterRefinementPicker(refinement, "")
				_ = a.window.Invalidate()
			}
		},
		OnKey: a.onRefinementPickerKey, OnDismiss: a.dismissRefinementPicker,
	}, woxui.Size{Width: contentBounds.Width, Height: contentBounds.Height})
	anchor.X += contentBounds.X
	anchor.Y += contentBounds.Y
	return overlay, anchor, popupBounds
}
