package launcher

import (
	"context"
	"strings"
	woxcomponent "wox/ui/launcher/component"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

type settingChoicePickerState struct {
	multi    bool
	item     settingItem
	anchor   woxui.Rect
	onChoose func(settingChoice)
}

type settingChoicePickerSnapshot struct {
	multi  bool
	item   settingItem
	anchor woxui.Rect
}

// buildSettingChoicePickerOverlay adapts controller state to the package-independent choice picker view.
func (c *generalSettingsController) buildSettingChoicePickerOverlay(snapshot *settingChoicePickerSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	choices := make([]launcherview.SettingsChoice, len(snapshot.item.choices))
	for index, choice := range snapshot.item.choices {
		trailing := snapshot.item.trailers[choice.value]
		if description := snapshot.item.choiceDescriptions[choice.value]; description != "" {
			trailing = description
		}
		var leading *woxui.Image
		if source := snapshot.item.icons[choice.value]; source.ImageData != "" {
			if snapshot.item.preserveIconColor {
				leading = c.imageForSurface(source, physicalImageSize(18, imageScale), settingsPalette().Background)
			} else {
				leading = c.imageForTint(source, &palette.Text, physicalImageSize(18, imageScale))
			}
		}
		choices[index] = launcherview.SettingsChoice{
			Value: choice.value, Label: choice.label, Leading: leading, Trailing: trailing,
			Tooltip: c.localizedSettingChoiceTooltip(snapshot.item.key, choice),
		}
	}
	searchIcon := c.imageForTint(settingControlIconSource("search"), &palette.TextSecondary, physicalImageSize(16, imageScale))
	var selectedValues []string
	if snapshot.multi {
		// A non-nil slice selects multi-select mode even when nothing is chosen.
		selectedValues = make([]string, 0)
		for _, part := range strings.Split(snapshot.item.value, ",") {
			if part = strings.TrimSpace(part); part != "" {
				selectedValues = append(selectedValues, part)
			}
		}
	}
	return launcherview.SettingsChoiceView(launcherview.SettingsChoiceProps{
		SelectedValues: selectedValues,
		ID:             "setting-choice-picker", Width: width, Height: height, Anchor: snapshot.anchor, Filterable: snapshot.item.filterable, Theme: palette, Window: c.settings.settingsNativeWindow(), Title: snapshot.item.title,
		FilterHint: c.language.translate("i18n:ui_filter_placeholder"), SearchIcon: searchIcon, CurrentValue: snapshot.item.value, Choices: choices, OnChoose: c.chooseSettingChoice, OnCancel: c.closeSettingChoicePicker, OnTooltip: c.settings.setSettingChoiceTooltip,
	})
}

func snapshotSettingChoicePickerLocked(state *settingChoicePickerState) *settingChoicePickerSnapshot {
	if state == nil {
		return nil
	}
	item := state.item
	item.choices = append([]settingChoice(nil), state.item.choices...)
	return &settingChoicePickerSnapshot{item: item, anchor: state.anchor, multi: state.multi}
}

func (c *generalSettingsController) openOrActivateSetting() {
	snapshot := c.settingsSnapshot()
	items := settingItemsForSnapshot(snapshot)
	if snapshot.saving || snapshot.row < 0 || snapshot.row >= len(items) {
		return
	}
	item := c.localizedSettingItem(items[snapshot.row])
	if item.disabled {
		return
	}
	if item.text || isBooleanSettingItem(item) {
		c.activateSetting(1)
		return
	}
	c.openSettingChoicePicker(item)
}

func isBooleanSettingItem(item settingItem) bool {
	return len(item.choices) == 2 && item.choices[0].value == "false" && item.choices[1].value == "true"
}

func (c *generalSettingsController) openSettingChoicePicker(item settingItem) {
	host := c.settings.host
	anchor := woxui.Rect{}
	if host != nil {
		anchor, _ = host.BoundsForKey(launcherview.SettingChoiceAnchorKey(item.key))
	}
	c.openSettingChoicePickerAt(item, anchor)
}

// openSettingChoicePickerAt anchors pointer-opened menus to the bounds from the exact hit-tested frame.
func (c *generalSettingsController) openSettingChoicePickerAt(item settingItem, anchor woxui.Rect) {
	if c.settings.saving || item.disabled || len(item.choices) == 0 {
		return
	}
	c.EndEdit()
	c.SetShowDisplayPicker(nil)
	c.SetChoicePicker(&settingChoicePickerState{item: item, anchor: anchor})
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *generalSettingsController) closeSettingChoicePicker() {
	closed := false
	if c.ChoicePicker() != nil {
		c.SetChoicePicker(nil)
		closed = true
	}
	if closed {
		c.settings.setSettingChoiceTooltip(false, "", woxui.Rect{})
	}
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
}

func (c *generalSettingsController) chooseSettingChoice(index int) {
	state := c.ChoicePicker()
	if state == nil || c.settings.saving {
		return
	}
	if index < 0 || index >= len(state.item.choices) {
		return
	}
	item := state.item
	choice := state.item.choices[index]
	if !state.multi {
		c.SetChoicePicker(nil)
	}
	if state.onChoose != nil {
		c.settings.setSettingChoiceTooltip(false, "", woxui.Rect{})
		c.updateSettingsText(false)
		state.onChoose(choice)
		c.settings.invalidateSettingsWindow()
		return
	}
	c.settings.setSettingChoiceTooltip(false, "", woxui.Rect{})
	c.updateSettingsText(false)
	c.settings.invalidateSettingsWindow()
	c.startGeneralSettingSave("save setting choice", item, choice)
}

// loadSystemFontFamilies keeps enumeration in core while the framework only consumes portable family names.
func (a *App) loadSystemFontFamilies() {
	a.appearanceSettings.ReloadFonts(context.Background(), a.services, a.sessionID)
}

func systemFontSettingItem(snapshot settingsSnapshot) settingItem {
	appearance := snapshot.appearance
	choices := make([]settingChoice, 0, len(appearance.FontFamilies)+2)
	choices = append(choices, settingChoice{value: "", label: "System default"})
	found := snapshot.general.Data.AppFontFamily == ""
	for _, family := range appearance.FontFamilies {
		choices = append(choices, settingChoice{value: family, label: family})
		if family == snapshot.general.Data.AppFontFamily {
			found = true
		}
	}
	if !found {
		choices = append([]settingChoice{{value: snapshot.general.Data.AppFontFamily, label: snapshot.general.Data.AppFontFamily}}, choices...)
	}
	description := "Font family used by Query and Settings windows"
	if appearance.FontsError != "" {
		description = "Could not load installed fonts: " + appearance.FontsError
	}
	return settingItem{key: "AppFontFamily", title: "Application font", description: description, value: snapshot.general.Data.AppFontFamily, choices: choices, filterable: true}
}
