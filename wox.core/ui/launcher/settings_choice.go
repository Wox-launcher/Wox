package launcher

import (
	"context"
	"log"
	"sort"
	"strings"
	"time"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type settingChoicePickerState struct {
	item   settingItem
	anchor woxui.Rect
}

type settingChoicePickerSnapshot struct {
	item   settingItem
	anchor woxui.Rect
}

// buildSettingChoicePickerOverlay adapts controller state to the package-independent choice picker view.
func (a *App) buildSettingChoicePickerOverlay(snapshot *settingChoicePickerSnapshot, palette uiPalette, width, height float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.item.choices))
	for index, choice := range snapshot.item.choices {
		choices[index] = launcherview.SettingsChoice{Value: choice.value, Label: choice.label, Trailing: snapshot.item.trailers[choice.value], Tooltip: a.localizedSettingChoiceTooltip(snapshot.item.key, choice)}
	}
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		ID: "setting-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Filterable: snapshot.item.filterable, Theme: palette.componentTheme(), Window: a.settingsNativeWindow(), Title: snapshot.item.title,
		CurrentValue: snapshot.item.value, Choices: choices, OnChoose: a.chooseSettingChoice, OnCancel: a.closeSettingChoicePicker, OnTooltip: a.setSettingChoiceTooltip,
	})
}

func snapshotSettingChoicePickerLocked(state *settingChoicePickerState) *settingChoicePickerSnapshot {
	if state == nil {
		return nil
	}
	item := state.item
	item.choices = append([]settingChoice(nil), state.item.choices...)
	return &settingChoicePickerSnapshot{item: item, anchor: state.anchor}
}

func (a *App) openOrActivateSetting() {
	snapshot := a.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.saving || snapshot.row < 0 || snapshot.row >= len(items) {
		return
	}
	item := a.localizedSettingItem(items[snapshot.row])
	if item.disabled {
		return
	}
	if item.text || isBooleanSettingItem(item) {
		a.activateSetting(1)
		return
	}
	a.openSettingChoicePicker(item)
}

func isBooleanSettingItem(item settingItem) bool {
	return len(item.choices) == 2 && item.choices[0].value == "false" && item.choices[1].value == "true"
}

func (a *App) openSettingChoicePicker(item settingItem) {
	a.mu.RLock()
	host := a.settingsHost
	a.mu.RUnlock()
	anchor := woxui.Rect{}
	if host != nil {
		anchor, _ = host.BoundsForKey(launcherview.SettingChoiceAnchorKey(item.key))
	}
	a.openSettingChoicePickerAt(item, anchor)
}

// openSettingChoicePickerAt anchors pointer-opened menus to the bounds from the exact hit-tested frame.
func (a *App) openSettingChoicePickerAt(item settingItem, anchor woxui.Rect) {
	a.mu.Lock()
	if a.settingSaving || item.disabled || len(item.choices) == 0 {
		a.mu.Unlock()
		return
	}
	a.settingChoicePicker = &settingChoicePickerState{item: item, anchor: anchor}
	a.settingEditKey = ""
	a.settingEditor = nil
	a.settingNote = ""
	if item.filterable {
		a.settingNote = "Filter and select " + item.title
	}
	a.mu.Unlock()
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) closeSettingChoicePicker() {
	closed := false
	a.mu.Lock()
	if a.settingChoicePicker != nil {
		a.settingChoicePicker = nil
		a.settingNote = ""
		closed = true
	}
	a.mu.Unlock()
	if closed {
		a.setSettingChoiceTooltip(false, "", woxui.Rect{})
	}
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
}

func (a *App) chooseSettingChoice(index int) {
	a.mu.Lock()
	state := a.settingChoicePicker
	if state == nil || a.settingSaving {
		a.mu.Unlock()
		return
	}
	if index < 0 || index >= len(state.item.choices) {
		a.mu.Unlock()
		return
	}
	item := state.item
	choice := state.item.choices[index]
	a.settingChoicePicker = nil
	a.settingSaving = true
	a.settingNote = "Saving " + item.title + "…"
	a.mu.Unlock()
	a.setSettingChoiceTooltip(false, "", woxui.Rect{})
	a.updateSettingsTextInput(false)
	a.invalidateSettingsWindow()
	go a.saveSetting(item, choice)
}

func (a *App) setSettingChoiceTooltip(inside bool, text string, anchor woxui.Rect) {
	a.mu.Lock()
	a.choiceTooltipRevision++
	revision := a.choiceTooltipRevision
	a.mu.Unlock()

	go func() {
		a.tooltipMu.Lock()
		defer a.tooltipMu.Unlock()
		a.mu.RLock()
		current := revision == a.choiceTooltipRevision
		a.mu.RUnlock()
		if !current {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if !inside {
			if err := a.client.Post(ctx, "/tooltip/hide", map[string]string{"name": "go-ui-setting-choice"}, nil); err != nil {
				log.Printf("hide settings choice tooltip: %v", err)
			}
			return
		}
		window := a.settingsNativeWindow()
		if window == nil {
			return
		}
		windowBounds, err := window.Bounds()
		if err != nil {
			log.Printf("read settings bounds for choice tooltip: %v", err)
			return
		}
		err = a.client.Post(ctx, "/tooltip/show", map[string]any{
			"name": "go-ui-setting-choice", "text": text, "side": "left",
			"anchorX": windowBounds.X + anchor.X, "anchorY": windowBounds.Y + anchor.Y,
			"anchorWidth": anchor.Width, "anchorHeight": anchor.Height,
		}, nil)
		if err != nil {
			log.Printf("show settings choice tooltip: %v", err)
		}
	}()
}

// loadSystemFontFamilies keeps enumeration in core while the framework only consumes portable family names.
func (a *App) loadSystemFontFamilies() {
	a.mu.Lock()
	if a.systemFontsLoaded || a.systemFontsLoading {
		a.mu.Unlock()
		return
	}
	a.systemFontsLoading = true
	a.systemFontsError = ""
	a.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var families []string
	err := a.client.Post(ctx, "/setting/ui/fonts", map[string]any{}, &families)
	cancel()
	if err == nil {
		seen := make(map[string]bool, len(families))
		filtered := make([]string, 0, len(families))
		for _, family := range families {
			family = strings.TrimSpace(family)
			key := strings.ToLower(family)
			if family == "" || seen[key] {
				continue
			}
			seen[key] = true
			filtered = append(filtered, family)
		}
		sort.SliceStable(filtered, func(i, j int) bool { return strings.ToLower(filtered[i]) < strings.ToLower(filtered[j]) })
		families = filtered
	}
	a.mu.Lock()
	a.systemFontsLoading = false
	if err != nil {
		a.systemFontsError = err.Error()
	} else {
		a.systemFontFamilies = families
		a.systemFontsLoaded = true
		a.systemFontsError = ""
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}

func systemFontSettingItem(snapshot settingsSnapshot) settingItem {
	choices := make([]settingChoice, 0, len(snapshot.systemFontFamilies)+2)
	choices = append(choices, settingChoice{value: "", label: "System default"})
	found := snapshot.data.AppFontFamily == ""
	for _, family := range snapshot.systemFontFamilies {
		choices = append(choices, settingChoice{value: family, label: family})
		if family == snapshot.data.AppFontFamily {
			found = true
		}
	}
	if !found {
		choices = append([]settingChoice{{value: snapshot.data.AppFontFamily, label: snapshot.data.AppFontFamily}}, choices...)
	}
	description := "Font family used by Query and Settings windows"
	if snapshot.systemFontsLoading {
		description = "Loading installed font families…"
	} else if snapshot.systemFontsError != "" {
		description = "Could not load installed fonts: " + snapshot.systemFontsError
	}
	return settingItem{key: "AppFontFamily", title: "Application font", description: description, value: snapshot.data.AppFontFamily, choices: choices, filterable: true}
}
