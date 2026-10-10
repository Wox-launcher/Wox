package launcher

import (
	"encoding/json"
	"fmt"
	"strings"

	"wox/setting"
	woxcomponent "wox/ui/launcher/component"
	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
	"wox/util"
	"wox/util/screen"
	utilwindow "wox/util/window"
)

type showDisplayPickerState struct {
	loading  bool
	error    string
	displays []screen.Display
}

type showDisplayPickerSnapshot struct {
	Loading  bool
	Error    string
	Displays []screen.Display
}

// ensureShowDisplays loads the current layout once for the specific-screen button.
func (a *App) ensureShowDisplays() {
	if a.generalSettings.ShowDisplaysLoaded() || a.generalSettings.ShowDisplaysLoading() || a.generalSettings.showDisplaysError != "" || a.lifecycleCtx == nil {
		return
	}
	a.refreshShowDisplays()
}

// refreshShowDisplays re-reads monitors and updates the button and an open picker.
func (a *App) refreshShowDisplays() {
	if a.lifecycleCtx == nil || a.generalSettings.ShowDisplaysLoading() {
		return
	}
	a.generalSettings.SetShowDisplaysLoading()
	util.Go(a.lifecycleCtx, "load show displays", func() {
		displays, err := screen.ListDisplays()
		if err == nil {
			screen.SortDisplays(displays)
		}
		_ = a.runOnUI("apply show displays", func() {
			a.generalSettings.ApplyShowDisplays(displays, err)
			a.invalidateSettingsWindow()
		})
	})
}

// openShowDisplayPicker shows the screen map for the specific-screen setting.
func (a *App) openShowDisplayPicker() {
	if a.generalSettings.ShowDisplayPicker() != nil {
		return
	}
	a.generalSettings.SetChoicePicker(nil)
	displays, loaded := a.generalSettings.ShowDisplays()
	a.generalSettings.SetShowDisplayPicker(&showDisplayPickerState{loading: !loaded, displays: displays})
	a.invalidateSettingsWindow()
	a.refreshShowDisplays()
}

// closeShowDisplayPicker dismisses the screen map without changing the saved monitor.
func (a *App) closeShowDisplayPicker() {
	if a.generalSettings.ShowDisplayPicker() == nil {
		return
	}
	a.generalSettings.SetShowDisplayPicker(nil)
	a.invalidateSettingsWindow()
}

// chooseShowDisplay stores the clicked monitor and closes the map.
func (a *App) chooseShowDisplay(index int) {
	picker := a.generalSettings.ShowDisplayPicker()
	if picker == nil || a.settingSaving || index < 0 || index >= len(picker.displays) {
		return
	}
	display := picker.displays[index]
	area := display.WorkArea
	if area.IsEmpty() {
		area = display.Bounds
	}
	target := setting.ShowDisplayTarget{
		ID: display.ID, WorkX: area.X, WorkY: area.Y, WorkWidth: area.Width, WorkHeight: area.Height, Primary: display.Primary,
	}
	encoded, err := json.Marshal(target)
	a.generalSettings.SetShowDisplayPicker(nil)
	a.invalidateSettingsWindow()
	if err != nil {
		return
	}
	a.startGeneralSettingSave("save show display", settingItem{key: "ShowDisplay"}, settingChoice{value: string(encoded)})
}

// retryShowDisplays reloads the map after an enumeration failure.
func (a *App) retryShowDisplays() {
	picker := a.generalSettings.ShowDisplayPicker()
	if picker == nil {
		return
	}
	picker.loading = true
	picker.error = ""
	a.generalSettings.ClearShowDisplaysLoading()
	a.invalidateSettingsWindow()
	a.refreshShowDisplays()
}

func (a *App) buildShowDisplayRow(snapshot settingsSnapshot, width float32) woxwidget.Widget {
	label := showDisplayButtonLabel(snapshot.general.Data.ShowDisplay, snapshot.general.ShowDisplays, snapshot.general.ShowDisplaysLoaded, !util.IsLinux(), a.translate)
	row := launcherview.SettingRow(launcherview.SettingRowProps{
		ID: "ShowDisplay", Title: a.translate("i18n:ui_show_position_screen"), Description: a.translate("i18n:ui_show_position_screen_tips"),
		Value: label, Width: width, Kind: "button", Theme: snapshot.palette, OnTap: a.openShowDisplayPicker,
	})
	return woxwidget.Keyed{Key: "setting-row-ShowDisplay", Child: woxcomponent.WoxSettingTarget(woxcomponent.SettingTargetProps{
		Width: width, Child: row, Theme: snapshot.palette,
	})}
}

func (a *App) buildShowDisplayPickerOverlay(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	picker := snapshot.general.ShowDisplayPicker
	if picker == nil {
		return nil
	}
	saved := snapshot.general.Data.ShowDisplay
	trustID := !util.IsLinux()
	selected := -1
	if saved.Chosen() && len(picker.Displays) > 0 {
		display, match := screen.ResolveShowDisplay(picker.Displays, savedShowDisplay(saved), trustID)
		if match == screen.ShowDisplayMatchID || match == screen.ShowDisplayMatchGeometry {
			selected = showDisplayOffset(picker.Displays, display)
		}
	}
	tiles := make([]launcherview.DisplayArrangementTile, 0, len(picker.Displays))
	physical := !util.IsLinux()
	for index, display := range picker.Displays {
		// Linux GDK geometry uses one logical desktop; scaling each origin separately
		// makes mixed-DPI tiles overlap. Other platforms use physical desktop bounds.
		rect := windowRectFromScreen(showDisplayMapRect(display, physical))
		tiles = append(tiles, launcherview.DisplayArrangementTile{
			Index: index, Selected: index == selected, IsPrimary: display.Primary,
			Bounds: rect, WorkArea: rect,
		})
	}
	return launcherview.ShowDisplayPicker(launcherview.ShowDisplayPickerProps{
		Width: width, Height: height,
		Title:       a.translate("i18n:ui_show_position_screen_picker_title"),
		Description: a.translate("i18n:ui_show_position_screen_tips"),
		CancelLabel: a.translate("i18n:ui_cancel"),
		Theme:       snapshot.palette, OnCancel: a.closeShowDisplayPicker,
		Arrangement: launcherview.DisplayArrangementProps{
			Loading: picker.Loading, Error: picker.Error, Tiles: tiles, Theme: snapshot.palette,
			TileIDPrefix: "show-display", PrimaryLabel: a.translate("i18n:plugin_window_manager_group_display_primary"),
			EmptyLabel: a.translate("i18n:plugin_window_manager_group_no_displays"),
			RetryLabel: a.translate("i18n:plugin_window_manager_group_retry"),
			OnSelect:   a.chooseShowDisplay, OnRetry: a.retryShowDisplays,
		},
	})
}

// showDisplayButtonLabel names the saved monitor, or asks the user to choose one.
func showDisplayButtonLabel(saved setting.ShowDisplayTarget, displays []screen.Display, loaded bool, trustID bool, translate func(string) string) string {
	if !saved.Chosen() {
		return translate("i18n:ui_show_position_choose_screen")
	}
	if !loaded {
		if saved.WorkWidth > 0 && saved.WorkHeight > 0 {
			if saved.Primary {
				return formatShowDisplayLabel(translate, true, 0, saved.WorkWidth, saved.WorkHeight)
			}
			return translate("i18n:ui_show_position_screen") + " · " + fmt.Sprintf("%d×%d", saved.WorkWidth, saved.WorkHeight)
		}
		return translate("i18n:ui_show_position_screen")
	}
	display, match := screen.ResolveShowDisplay(displays, savedShowDisplay(saved), trustID)
	if match != screen.ShowDisplayMatchID && match != screen.ShowDisplayMatchGeometry {
		return translate("i18n:ui_show_position_screen_disconnected")
	}
	bounds := showDisplayMapRect(display, true)
	return formatShowDisplayLabel(translate, display.Primary, showDisplayOffset(displays, display)+1, bounds.Width, bounds.Height)
}

func formatShowDisplayLabel(translate func(string) string, primary bool, index int, width, height int) string {
	key := "ui_show_position_screen_numbered"
	if primary {
		key = "ui_show_position_screen_primary"
	}
	label := translate("i18n:" + key)
	label = strings.ReplaceAll(label, "{index}", fmt.Sprintf("%d", index))
	label = strings.ReplaceAll(label, "{width}", fmt.Sprintf("%d", width))
	label = strings.ReplaceAll(label, "{height}", fmt.Sprintf("%d", height))
	return label
}

func showDisplayOffset(displays []screen.Display, display screen.Display) int {
	for index, candidate := range displays {
		if candidate.ID == display.ID && candidate.Bounds == display.Bounds && candidate.WorkArea == display.WorkArea {
			return index
		}
	}
	for index, candidate := range displays {
		if candidate.ID == display.ID {
			return index
		}
	}
	return 0
}

// showDisplayMapRect is the monitor rectangle drawn in the picker.
// Linux uses GDK's shared logical desktop; other platforms use physical bounds.
func showDisplayMapRect(display screen.Display, physical bool) screen.Rect {
	if physical && !display.PixelBounds.IsEmpty() {
		return display.PixelBounds
	}
	if !display.Bounds.IsEmpty() {
		return display.Bounds
	}
	return display.WorkArea
}

func windowRectFromScreen(rect screen.Rect) utilwindow.WindowRect {
	return utilwindow.WindowRect{X: rect.X, Y: rect.Y, Width: rect.Width, Height: rect.Height}
}

func (c *generalSettingsController) ShowDisplaysLoaded() bool {
	return c.showDisplaysLoaded
}

func (c *generalSettingsController) ShowDisplaysLoading() bool {
	return c.showDisplaysLoading
}

func (c *generalSettingsController) SetShowDisplaysLoading() {
	c.showDisplaysLoading = true
}

func (c *generalSettingsController) ClearShowDisplaysLoading() {
	c.showDisplaysLoading = false
}

// ShowDisplays returns the cached layout. The bool is false until the first successful read.
func (c *generalSettingsController) ShowDisplays() ([]screen.Display, bool) {
	return append([]screen.Display(nil), c.showDisplays...), c.showDisplaysLoaded
}

// ApplyShowDisplays stores a layout read and mirrors it into an open picker.
func (c *generalSettingsController) ApplyShowDisplays(displays []screen.Display, err error) {
	c.showDisplaysLoading = false
	if err != nil {
		if !c.showDisplaysLoaded {
			c.showDisplaysError = err.Error()
		}
	} else {
		c.showDisplaysLoaded = true
		c.showDisplaysError = ""
		c.showDisplays = displays
	}
	picker := c.showDisplayPicker
	if picker == nil {
		return
	}
	picker.loading = false
	if err != nil && len(picker.displays) == 0 {
		picker.error = err.Error()
		return
	}
	if err == nil {
		picker.error = ""
		picker.displays = displays
	}
}

func (c *generalSettingsController) ShowDisplayPicker() *showDisplayPickerState {
	return c.showDisplayPicker
}

func (c *generalSettingsController) SetShowDisplayPicker(state *showDisplayPickerState) {
	c.showDisplayPicker = state
}

// ClearShowDisplays drops the cached layout so the next settings open reads it again.
func (c *generalSettingsController) ClearShowDisplays() {
	c.showDisplays = nil
	c.showDisplaysLoaded = false
	c.showDisplaysLoading = false
	c.showDisplaysError = ""
}

func savedShowDisplay(target setting.ShowDisplayTarget) screen.SavedDisplay {
	return screen.SavedDisplay{
		ID: target.ID, WorkX: target.WorkX, WorkY: target.WorkY,
		WorkWidth: target.WorkWidth, WorkHeight: target.WorkHeight, Primary: target.Primary,
	}
}
