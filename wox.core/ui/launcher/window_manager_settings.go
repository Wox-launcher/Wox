package launcher

import (
	"encoding/json"
	"strconv"
	"strings"
	"wox/common"
	woxcomponent "wox/ui/launcher/component"

	"github.com/google/uuid"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	utilwindow "wox/util/window"
)

const (
	windowManagerPluginID          = common.WindowManagerPluginID
	windowManagerGroupsSettingKey  = "windowGroups"
	windowManagerExtensionStoreURL = "https://chromewebstore.google.com/detail/wox/bjbkdpjdnagiongdfemjhepkkglnailh"
)

type windowManagerGroupUI struct {
	Id      string
	Name    string
	Screens []windowManagerGroupScreenUI
}

type windowManagerGroupScreenUI struct {
	DisplayId    string
	DisplayIndex int
	Layout       string
	Assignments  []windowManagerGroupAssignmentUI
}

type windowManagerGroupAssignmentUI struct {
	Slot string
	App  ignoredHotkeyApp
	Urls []string
}

type windowGroupEditorState struct {
	group              windowManagerGroupUI
	displays           []utilwindow.DisplayInfo
	selectedDisplay    int
	nameError          string
	loadingDisplays    bool
	displaysError      string
	appPickerSlot      string
	urlEditorSlot      string
	extensionChecking  bool
	extensionConnected bool
	editing            bool
}

type windowGroupEditorSnapshot struct {
	group              windowManagerGroupUI
	displays           []utilwindow.DisplayInfo
	selectedDisplay    int
	nameError          string
	loadingDisplays    bool
	displaysError      string
	appPickerSlot      string
	urlEditorSlot      string
	extensionChecking  bool
	extensionConnected bool
	editing            bool
}

type windowGroupUILayoutSlot struct {
	Id       string
	TitleKey string
	Cols     int
	Rows     int
	Col      int
	Row      int
	ColSpan  int
	RowSpan  int
}

type windowGroupUILayout struct {
	Id        string
	TitleKey  string
	SlotCount int
	Slots     []windowGroupUILayoutSlot
}

var windowGroupUILayoutCatalog = []windowGroupUILayout{
	{Id: "full", TitleKey: "plugin_window_manager_group_layout_full", SlotCount: 1, Slots: []windowGroupUILayoutSlot{
		{Id: "full", TitleKey: "plugin_window_manager_group_slot_full", Cols: 1, Rows: 1, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
	}},
	{Id: "halves-horizontal", TitleKey: "plugin_window_manager_group_layout_halves_horizontal", SlotCount: 2, Slots: []windowGroupUILayoutSlot{
		{Id: "left", TitleKey: "plugin_window_manager_group_slot_left", Cols: 2, Rows: 1, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "right", TitleKey: "plugin_window_manager_group_slot_right", Cols: 2, Rows: 1, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
	}},
	{Id: "halves-vertical", TitleKey: "plugin_window_manager_group_layout_halves_vertical", SlotCount: 2, Slots: []windowGroupUILayoutSlot{
		{Id: "top", TitleKey: "plugin_window_manager_group_slot_top", Cols: 1, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "bottom", TitleKey: "plugin_window_manager_group_slot_bottom", Cols: 1, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
	}},
	{Id: "three-left-main", TitleKey: "plugin_window_manager_group_layout_three_left_main", SlotCount: 3, Slots: []windowGroupUILayoutSlot{
		{Id: "left", TitleKey: "plugin_window_manager_group_slot_left", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 2},
		{Id: "rightTop", TitleKey: "plugin_window_manager_group_slot_right_top", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "rightBottom", TitleKey: "plugin_window_manager_group_slot_right_bottom", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
	}},
	{Id: "three-right-main", TitleKey: "plugin_window_manager_group_layout_three_right_main", SlotCount: 3, Slots: []windowGroupUILayoutSlot{
		{Id: "leftTop", TitleKey: "plugin_window_manager_group_slot_left_top", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "leftBottom", TitleKey: "plugin_window_manager_group_slot_left_bottom", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
		{Id: "right", TitleKey: "plugin_window_manager_group_slot_right", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 2},
	}},
	{Id: "three-top-main", TitleKey: "plugin_window_manager_group_layout_three_top_main", SlotCount: 3, Slots: []windowGroupUILayoutSlot{
		{Id: "top", TitleKey: "plugin_window_manager_group_slot_top", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 2, RowSpan: 1},
		{Id: "bottomLeft", TitleKey: "plugin_window_manager_group_slot_bottom_left", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
		{Id: "bottomRight", TitleKey: "plugin_window_manager_group_slot_right_bottom", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
	}},
	{Id: "three-bottom-main", TitleKey: "plugin_window_manager_group_layout_three_bottom_main", SlotCount: 3, Slots: []windowGroupUILayoutSlot{
		{Id: "topLeft", TitleKey: "plugin_window_manager_group_slot_top_left", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "topRight", TitleKey: "plugin_window_manager_group_slot_top_right", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "bottom", TitleKey: "plugin_window_manager_group_slot_bottom", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 2, RowSpan: 1},
	}},
	{Id: "quarters", TitleKey: "plugin_window_manager_group_layout_quarters", SlotCount: 4, Slots: []windowGroupUILayoutSlot{
		{Id: "topLeft", TitleKey: "plugin_window_manager_group_slot_top_left", Cols: 2, Rows: 2, Col: 0, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "topRight", TitleKey: "plugin_window_manager_group_slot_top_right", Cols: 2, Rows: 2, Col: 1, Row: 0, ColSpan: 1, RowSpan: 1},
		{Id: "bottomLeft", TitleKey: "plugin_window_manager_group_slot_bottom_left", Cols: 2, Rows: 2, Col: 0, Row: 1, ColSpan: 1, RowSpan: 1},
		{Id: "bottomRight", TitleKey: "plugin_window_manager_group_slot_right_bottom", Cols: 2, Rows: 2, Col: 1, Row: 1, ColSpan: 1, RowSpan: 1},
	}},
}

func windowGroupLayoutByID(id string) windowGroupUILayout {
	for _, layout := range windowGroupUILayoutCatalog {
		if layout.Id == id {
			return layout
		}
	}
	return windowGroupUILayoutCatalog[0]
}

func windowGroupLayoutsBySlotCount(slotCount int) []windowGroupUILayout {
	layouts := make([]windowGroupUILayout, 0, len(windowGroupUILayoutCatalog))
	for _, layout := range windowGroupUILayoutCatalog {
		if layout.SlotCount == slotCount {
			layouts = append(layouts, layout)
		}
	}
	return layouts
}

func isWindowManagerGroupsTable(definition formDefinition) bool {
	return definition.Type == "table" && definition.Value.Key == windowManagerGroupsSettingKey
}

func isWindowManagerGroupsEditor(pluginForm *pluginSettingsFormState, state *formTableEditorState) bool {
	if state == nil || !isWindowManagerGroupsTable(state.definition) {
		return false
	}
	return pluginForm != nil && state.target == &pluginForm.formFieldsState && pluginForm.pluginID == windowManagerPluginID
}

func snapshotWindowGroupEditorLocked(editor *windowGroupEditorState) *windowGroupEditorSnapshot {
	if editor == nil {
		return nil
	}
	return &windowGroupEditorSnapshot{
		group: editor.group.copyGroup(), displays: append([]utilwindow.DisplayInfo(nil), editor.displays...),
		selectedDisplay: editor.selectedDisplay, nameError: editor.nameError, loadingDisplays: editor.loadingDisplays,
		displaysError: editor.displaysError, appPickerSlot: editor.appPickerSlot, urlEditorSlot: editor.urlEditorSlot,
		extensionChecking: editor.extensionChecking, extensionConnected: editor.extensionConnected, editing: editor.editing,
	}
}

func windowManagerGroupFromRow(row map[string]any) windowManagerGroupUI {
	encoded, err := json.Marshal(row)
	if err != nil {
		return windowManagerGroupUI{}
	}
	var group windowManagerGroupUI
	if json.Unmarshal(encoded, &group) != nil {
		return windowManagerGroupUI{}
	}
	return group
}

func windowManagerGroupToRow(group windowManagerGroupUI) map[string]any {
	encoded, err := json.Marshal(group)
	if err != nil {
		return map[string]any{}
	}
	var row map[string]any
	if json.Unmarshal(encoded, &row) != nil {
		return map[string]any{}
	}
	return row
}

func (group *windowManagerGroupUI) appCount() int {
	count := 0
	for _, screen := range group.Screens {
		for _, assignment := range screen.Assignments {
			if strings.TrimSpace(assignment.App.Identity) != "" {
				count++
			}
		}
	}
	return count
}

func (group windowManagerGroupUI) copyGroup() windowManagerGroupUI {
	encoded, _ := json.Marshal(group)
	var copied windowManagerGroupUI
	_ = json.Unmarshal(encoded, &copied)
	return copied
}

func (group *windowManagerGroupUI) reconcileScreens(displays []utilwindow.DisplayInfo) {
	if len(displays) == 0 {
		return
	}
	resolved := make([]*windowManagerGroupScreenUI, len(displays))
	used := map[int]bool{}
	for index, display := range displays {
		displayID := strings.TrimSpace(display.Id)
		if displayID == "" {
			continue
		}
		for screenIndex, screen := range group.Screens {
			if used[screenIndex] {
				continue
			}
			if strings.TrimSpace(screen.DisplayId) == displayID {
				resolved[index] = &group.Screens[screenIndex]
				used[screenIndex] = true
				break
			}
		}
	}
	for index := range displays {
		if resolved[index] != nil {
			continue
		}
		for screenIndex, screen := range group.Screens {
			if used[screenIndex] {
				continue
			}
			if screen.DisplayIndex == index {
				resolved[index] = &group.Screens[screenIndex]
				used[screenIndex] = true
				break
			}
		}
	}
	screens := make([]windowManagerGroupScreenUI, len(displays))
	for index, display := range displays {
		if resolved[index] != nil {
			screen := resolved[index].copyScreen()
			screen.DisplayId = display.Id
			screen.DisplayIndex = index
			screens[index] = screen
			continue
		}
		screens[index] = windowManagerGroupScreenUI{
			DisplayId: display.Id, DisplayIndex: index, Layout: windowGroupUILayoutCatalog[0].Id,
			Assignments: []windowManagerGroupAssignmentUI{},
		}
	}
	group.Screens = screens
}

func (screen *windowManagerGroupScreenUI) copyScreen() windowManagerGroupScreenUI {
	encoded, _ := json.Marshal(screen)
	var copied windowManagerGroupScreenUI
	_ = json.Unmarshal(encoded, &copied)
	return copied
}

func (screen *windowManagerGroupScreenUI) assignmentFor(slotID string) *windowManagerGroupAssignmentUI {
	for index := range screen.Assignments {
		if screen.Assignments[index].Slot == slotID {
			return &screen.Assignments[index]
		}
	}
	return nil
}

func (group *windowManagerGroupUI) screenFor(displayIndex int) *windowManagerGroupScreenUI {
	if displayIndex < 0 || displayIndex >= len(group.Screens) {
		return nil
	}
	return &group.Screens[displayIndex]
}

func windowManagerGroupDisplayValue(column formTableColumn, row map[string]any) string {
	group := windowManagerGroupFromRow(row)
	switch column.Key {
	case "Name":
		name := strings.TrimSpace(group.Name)
		if name == "" {
			name = strings.TrimSpace(group.Id)
		}
		return name
	case "AppCount":
		return strconv.Itoa(group.appCount())
	case "DisplayCount":
		return strconv.Itoa(len(group.Screens))
	default:
		return formTableColumnValue(column, row)
	}
}

func (c *windowGroupEditorController) beginWindowManagerGroupEdit(rowIndex int, rowEditorOnly bool) bool {
	state := c.formTables.activeFormTableEditor()
	if !isWindowManagerGroupsEditor(c.pluginSettings().Form(), state) {
		return false
	}
	group := windowManagerGroupUI{Id: uuid.NewString(), Name: "", Screens: []windowManagerGroupScreenUI{}}
	editing := false
	if rowIndex >= 0 && rowIndex < len(state.rows) {
		group = windowManagerGroupFromRow(state.rows[rowIndex]).copyGroup()
		if strings.TrimSpace(group.Id) == "" {
			group.Id = uuid.NewString()
		}
		editing = true
	}
	state.rowIndex = rowIndex
	state.rowEditorOnly = rowEditorOnly
	state.rowForm = nil
	state.windowGroupEditor = &windowGroupEditorState{group: group, editing: editing, loadingDisplays: true}
	c.loadWindowManagerDisplays(state)
	c.formTables.invalidateFormTableWindow()
	return true
}

func (c *windowGroupEditorController) loadWindowManagerDisplays(state *formTableEditorState) {
	util.Go(c.lifecycleCtx, "load window manager displays", func() {
		displays, err := utilwindow.ListDisplays()
		_ = c.runOnUI("apply window manager displays", func() {
			current := c.formTables.activeFormTableEditor()
			if current != state || current.windowGroupEditor == nil {
				return
			}
			editor := current.windowGroupEditor
			editor.loadingDisplays = false
			if err != nil {
				editor.displaysError = err.Error()
				c.formTables.invalidateFormTableWindow()
				return
			}
			utilwindow.SortDisplays(displays)
			editor.displays = displays
			editor.group.reconcileScreens(displays)
			if editor.selectedDisplay >= len(displays) {
				editor.selectedDisplay = 0
			}
			c.formTables.invalidateFormTableWindow()
		})
	})
}

func (c *windowGroupEditorController) cancelWindowManagerGroupEdit() {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	closeEditor := state.rowEditorOnly
	state.windowGroupEditor = nil
	state.rowIndex = -1
	state.rowEditorOnly = false
	if closeEditor {
		c.formTables.closeFormTableEditor()
		return
	}
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) saveWindowManagerGroupEdit() {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || state.invalid || state.saving {
		return
	}
	editor := state.windowGroupEditor
	name := strings.TrimSpace(editor.group.Name)
	if name == "" {
		editor.nameError = c.language.translate("i18n:plugin_window_manager_group_name_required")
		c.formTables.invalidateFormTableWindow()
		return
	}
	editor.group.Name = name
	row := windowManagerGroupToRow(editor.group)
	if state.rowIndex >= 0 && state.rowIndex < len(state.rows) {
		state.rows[state.rowIndex] = row
		state.selected = state.rowIndex
	} else {
		state.rows = append(state.rows, row)
		state.selected = len(state.rows) - 1
	}
	if err := c.formTables.commitFormTableRowsLocked(state); err != nil {
		state.status = err.Error()
		c.formTables.invalidateFormTableWindow()
		return
	}
	closeEditor := state.rowEditorOnly
	state.windowGroupEditor = nil
	state.rowIndex = -1
	state.rowEditorOnly = false
	state.status = ""
	if closeEditor {
		c.formTables.closeFormTableEditor()
	} else {
		c.formTables.invalidateFormTableWindow()
	}
	c.pluginSettings().submitPluginSettings()
}

func (c *windowGroupEditorController) setWindowManagerGroupName(value string) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	hadError := state.windowGroupEditor.nameError != ""
	state.windowGroupEditor.group.Name = value
	if strings.TrimSpace(value) != "" {
		state.windowGroupEditor.nameError = ""
	}
	if hadError && state.windowGroupEditor.nameError == "" {
		c.formTables.invalidateFormTableWindow()
	}
}

func (c *windowGroupEditorController) selectWindowManagerGroupDisplay(index int) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || index < 0 || index >= len(state.windowGroupEditor.displays) {
		return
	}
	state.windowGroupEditor.selectedDisplay = index
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) setWindowManagerGroupLayout(layoutID string) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	screen := state.windowGroupEditor.group.screenFor(state.windowGroupEditor.selectedDisplay)
	if screen == nil {
		return
	}
	layout := windowGroupLayoutByID(layoutID)
	screen.Layout = layout.Id
	validSlots := map[string]bool{}
	for _, slot := range layout.Slots {
		validSlots[slot.Id] = true
	}
	filtered := screen.Assignments[:0]
	for _, assignment := range screen.Assignments {
		if validSlots[assignment.Slot] {
			filtered = append(filtered, assignment)
		}
	}
	screen.Assignments = filtered
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) openWindowManagerGroupAppPicker(slotID string) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.appPickerSlot = slotID
	if !c.hotkeySettings().AppsLoaded() && !c.hotkeySettings().AppsLoading() {
		util.Go(c.lifecycleCtx, "load hotkey app candidates", c.hotkeySettings().loadHotkeyAppCandidates)
	}
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) chooseWindowManagerGroupApp(candidate ignoredHotkeyApp) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || state.windowGroupEditor.appPickerSlot == "" {
		return
	}
	slotID := state.windowGroupEditor.appPickerSlot
	screen := state.windowGroupEditor.group.screenFor(state.windowGroupEditor.selectedDisplay)
	if screen == nil {
		return
	}
	preserved := []string(nil)
	if existing := screen.assignmentFor(slotID); existing != nil {
		preserved = append([]string(nil), existing.Urls...)
	}
	identity := strings.ToLower(strings.TrimSpace(candidate.Identity))
	if identity != "" {
		for displayIndex := range state.windowGroupEditor.group.Screens {
			filtered := state.windowGroupEditor.group.Screens[displayIndex].Assignments[:0]
			for _, assignment := range state.windowGroupEditor.group.Screens[displayIndex].Assignments {
				if strings.ToLower(strings.TrimSpace(assignment.App.Identity)) != identity {
					filtered = append(filtered, assignment)
				}
			}
			state.windowGroupEditor.group.Screens[displayIndex].Assignments = filtered
		}
	}
	filtered := screen.Assignments[:0]
	for _, assignment := range screen.Assignments {
		if assignment.Slot != slotID {
			filtered = append(filtered, assignment)
		}
	}
	screen.Assignments = filtered
	if identity != "" {
		screen.Assignments = append(screen.Assignments, windowManagerGroupAssignmentUI{Slot: slotID, App: candidate, Urls: preserved})
	}
	state.windowGroupEditor.appPickerSlot = ""
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) closeWindowManagerGroupAppPicker() {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.appPickerSlot = ""
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) openWindowManagerGroupUrlEditor(slotID string) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.urlEditorSlot = slotID
	state.windowGroupEditor.extensionChecking = true
	state.windowGroupEditor.extensionConnected = false
	c.loadWindowManagerExtensionStatus(state)
	c.formTables.invalidateFormTableWindow()
}

// loadWindowManagerExtensionStatus resolves browser integration outside the retained build pass.
func (c *windowGroupEditorController) loadWindowManagerExtensionStatus(state *formTableEditorState) {
	util.Go(c.lifecycleCtx, "load browser extension status", func() {
		connected, _ := c.services.BrowserExtensionConnected(c.lifecycleCtx, c.sessionID)
		_ = c.runOnUI("apply browser extension status", func() {
			current := c.formTables.activeFormTableEditor()
			if current != state || current.windowGroupEditor == nil || current.windowGroupEditor.urlEditorSlot == "" {
				return
			}
			current.windowGroupEditor.extensionChecking = false
			current.windowGroupEditor.extensionConnected = connected
			c.formTables.invalidateFormTableWindow()
		})
	})
}

func (c *windowGroupEditorController) saveWindowManagerGroupUrls(urls []string) {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || state.windowGroupEditor.urlEditorSlot == "" {
		return
	}
	slotID := state.windowGroupEditor.urlEditorSlot
	screen := state.windowGroupEditor.group.screenFor(state.windowGroupEditor.selectedDisplay)
	if screen == nil {
		return
	}
	assignment := screen.assignmentFor(slotID)
	if assignment == nil {
		state.windowGroupEditor.urlEditorSlot = ""
		c.formTables.invalidateFormTableWindow()
		return
	}
	filtered := make([]string, 0, len(urls))
	for _, url := range urls {
		if trimmed := strings.TrimSpace(url); trimmed != "" {
			filtered = append(filtered, trimmed)
		}
	}
	replaced := windowManagerGroupAssignmentUI{Slot: assignment.Slot, App: assignment.App, Urls: filtered}
	filteredAssignments := screen.Assignments[:0]
	for _, current := range screen.Assignments {
		if current.Slot == slotID {
			filteredAssignments = append(filteredAssignments, replaced)
			continue
		}
		filteredAssignments = append(filteredAssignments, current)
	}
	screen.Assignments = filteredAssignments
	state.windowGroupEditor.urlEditorSlot = ""
	c.formTables.invalidateFormTableWindow()
}

func (c *windowGroupEditorController) openWindowManagerExtensionStore() {
	if window := c.formTables.formTableNativeWindow(); window != nil {
		_ = window.OpenExternalURL(windowManagerExtensionStoreURL)
	}
}

func (c *windowGroupEditorController) cancelWindowManagerGroupUrlEditor() {
	state := c.formTables.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.urlEditorSlot = ""
	c.formTables.invalidateFormTableWindow()
}

func isBrowserApp(app ignoredHotkeyApp) bool {
	id := strings.ToLower(strings.TrimSpace(app.Identity))
	if id == "" {
		return false
	}
	win := map[string]bool{"chrome.exe": true, "msedge.exe": true, "firefox.exe": true, "brave.exe": true, "opera.exe": true, "launcher.exe": true}
	if win[id] {
		return true
	}
	macPrefixes := []string{"com.google.chrome", "com.microsoft.edgemac", "org.mozilla.firefox", "com.brave.browser", "com.operasoftware.opera", "org.chromium.chromium", "com.apple.safari"}
	for _, prefix := range macPrefixes {
		if strings.HasPrefix(id, prefix) {
			return true
		}
	}
	linux := map[string]bool{"google-chrome": true, "google-chrome-stable": true, "chromium": true, "chromium-browser": true, "microsoft-edge": true, "microsoft-edge-stable": true, "firefox": true, "brave-browser": true, "opera": true}
	return linux[id]
}

func (c *windowGroupEditorController) buildWindowManagerGroupEditor(snapshot *windowGroupEditorSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	if snapshot == nil {
		return woxwidget.Container{}
	}
	editor := snapshot
	iconTint := palette.Text
	addIconTint := palette.TextSecondary
	linkTint := woxui.Color{R: 110, G: 231, B: 183, A: 255}
	props := launcherview.WindowGroupEditorProps{
		Width: width, Height: height, GroupName: editor.group.Name, NameError: editor.nameError,
		LoadingDisplays: editor.loadingDisplays, DisplaysError: editor.displaysError, Editing: editor.editing,
		SelectedDisplay: editor.selectedDisplay, Theme: palette, Window: c.formTables.formTableNativeWindow(),
		CancelLabel: c.language.translate("i18n:ui_cancel"), SaveLabel: c.language.translate("i18n:ui_save"),
		SelectDisplayLabel:   c.language.translate("i18n:plugin_window_manager_group_select_display"),
		NoDisplaysLabel:      c.language.translate("i18n:plugin_window_manager_group_no_displays"),
		PrimaryDisplayLabel:  c.language.translate("i18n:plugin_window_manager_group_display_primary"),
		LayoutsLabel:         c.language.translate("i18n:plugin_window_manager_group_layouts"),
		LayoutsDescription:   c.language.translate("i18n:plugin_window_manager_group_layouts_description"),
		ChooseAppLabel:       c.language.translate("i18n:plugin_window_manager_group_slot_choose_app"),
		ChangeAppLabel:       c.language.translate("i18n:plugin_window_manager_group_slot_change_app"),
		BrowserUrlsLabel:     c.language.translate("i18n:plugin_window_manager_group_browser_urls"),
		BrowserUrlEmptyLabel: "URL",
		RetryLabel:           c.language.translate("i18n:plugin_window_manager_group_retry"),
		AddSlotIcon:          c.imageForTint(settingControlIconSource("add-circle"), &addIconTint, physicalImageSize(15, imageScale)),
		AppsIcon:             c.imageForTint(usageIconSource("apps"), &iconTint, physicalImageSize(18, imageScale)),
		LinkIcon:             c.imageForTint(settingControlIconSource("link"), &linkTint, physicalImageSize(13, imageScale)),
		OnCancel:             c.cancelWindowManagerGroupEdit, OnSave: c.saveWindowManagerGroupEdit, OnNameChanged: c.setWindowManagerGroupName,
		OnSelectDisplay: c.selectWindowManagerGroupDisplay, OnSelectLayout: c.setWindowManagerGroupLayout,
		OnOpenAppPicker: c.openWindowManagerGroupAppPicker, OnOpenUrlEditor: c.openWindowManagerGroupUrlEditor,
	}
	if editor.editing {
		props.Title = c.language.translate("i18n:plugin_window_manager_group_edit_dialog_title")
	} else {
		props.Title = c.language.translate("i18n:plugin_window_manager_group_create_dialog_title")
	}
	props.NamePlaceholder = c.language.translate("i18n:plugin_window_manager_group_name")
	for _, display := range editor.displays {
		props.Displays = append(props.Displays, launcherview.WindowGroupDisplayProps{
			Id: display.Id, Bounds: display.Bounds, WorkArea: display.WorkArea, IsPrimary: display.IsPrimary,
		})
	}
	selectedScreen := editor.group.screenFor(editor.selectedDisplay)
	selectedLayout := windowGroupLayoutByID(windowGroupUILayoutCatalog[0].Id)
	if selectedScreen != nil {
		selectedLayout = windowGroupLayoutByID(selectedScreen.Layout)
	}
	props.SelectedLayoutID = selectedLayout.Id
	for slotCount := 1; slotCount <= 4; slotCount++ {
		layouts := windowGroupLayoutsBySlotCount(slotCount)
		if len(layouts) == 0 {
			continue
		}
		slotLabel := strings.ReplaceAll(c.language.translate("i18n:plugin_window_manager_group_slot_count"), "{count}", strconv.Itoa(slotCount))
		group := launcherview.WindowGroupLayoutGroupProps{SlotCountLabel: slotLabel}
		for _, layout := range layouts {
			option := launcherview.WindowGroupLayoutOptionProps{
				ID: layout.Id, Label: c.language.translate("i18n:" + layout.TitleKey), Selected: layout.Id == selectedLayout.Id,
			}
			for _, slot := range layout.Slots {
				option.Slots = append(option.Slots, launcherview.WindowGroupSlotProps{
					ID: slot.Id, Cols: slot.Cols, Rows: slot.Rows, Col: slot.Col, Row: slot.Row, ColSpan: slot.ColSpan, RowSpan: slot.RowSpan,
				})
			}
			group.Layouts = append(group.Layouts, option)
		}
		props.LayoutGroups = append(props.LayoutGroups, group)
	}
	for displayIndex, display := range editor.displays {
		screen := editor.group.screenFor(displayIndex)
		layoutID := selectedLayout.Id
		if displayIndex != editor.selectedDisplay && screen != nil {
			layoutID = screen.Layout
		} else if displayIndex != editor.selectedDisplay {
			layoutID = windowGroupUILayoutCatalog[0].Id
		}
		tile := launcherview.WindowGroupDisplayTileProps{
			Index: displayIndex, Selected: displayIndex == editor.selectedDisplay, IsPrimary: display.IsPrimary,
			Bounds: display.Bounds, WorkArea: display.WorkArea, LayoutID: layoutID,
		}
		layout := windowGroupLayoutByID(layoutID)
		for _, slot := range layout.Slots {
			slotTile := launcherview.WindowGroupSlotProps{ID: slot.Id, Cols: slot.Cols, Rows: slot.Rows, Col: slot.Col, Row: slot.Row, ColSpan: slot.ColSpan, RowSpan: slot.RowSpan}
			if screen != nil {
				if assignment := screen.assignmentFor(slot.Id); assignment != nil && strings.TrimSpace(assignment.App.Identity) != "" {
					slotTile.AppName = assignment.App.Name
					if slotTile.AppName == "" {
						slotTile.AppName = assignment.App.Identity
					}
					slotTile.AppIcon = c.imageForSurface(assignment.App.Icon, 256, settingsPalette().Background)
					slotTile.IsBrowser = isBrowserApp(assignment.App)
					for _, url := range assignment.Urls {
						if strings.TrimSpace(url) != "" {
							slotTile.UrlCount++
						}
					}
				}
			}
			tile.Slots = append(tile.Slots, slotTile)
		}
		props.DisplayTiles = append(props.DisplayTiles, tile)
	}
	if editor.displaysError != "" {
		props.OnRetryDisplays = func() {
			if current := c.formTables.activeFormTableEditor(); current != nil && current.windowGroupEditor != nil {
				current.windowGroupEditor.loadingDisplays = true
				current.windowGroupEditor.displaysError = ""
				c.loadWindowManagerDisplays(current)
				c.formTables.invalidateFormTableWindow()
			}
		}
	}
	layers := []woxwidget.StackChild{{Child: launcherview.WindowGroupEditor(props)}}
	if editor.appPickerSlot != "" {
		var current ignoredHotkeyApp
		if selectedScreen != nil {
			if assignment := selectedScreen.assignmentFor(editor.appPickerSlot); assignment != nil {
				current = assignment.App
			}
		}
		layers = append(layers, woxwidget.StackChild{Child: c.buildWindowManagerGroupAppPicker(current, palette, width, height, imageScale)})
	}
	if editor.urlEditorSlot != "" && selectedScreen != nil {
		if assignment := selectedScreen.assignmentFor(editor.urlEditorSlot); assignment != nil {
			layers = append(layers, woxwidget.StackChild{Child: c.buildWindowManagerGroupUrlEditor(assignment.Urls, editor, palette, width, height, imageScale)})
		}
	}
	if len(layers) == 1 {
		return layers[0].Child
	}
	return woxwidget.Stack{Width: width, Height: height, Children: layers}
}

func (c *windowGroupEditorController) buildWindowManagerGroupAppPicker(current ignoredHotkeyApp, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	apps := c.hotkeySettings().AppCandidates()
	if identity := strings.TrimSpace(current.Identity); identity != "" {
		found := false
		for _, candidate := range apps {
			if strings.EqualFold(strings.TrimSpace(candidate.Identity), identity) {
				found = true
				break
			}
		}
		if !found {
			apps = append([]ignoredHotkeyApp{current}, apps...)
		}
	}
	candidates := make([]launcherview.FormAppCandidate, len(apps))
	for index, candidate := range apps {
		detail := strings.TrimSpace(candidate.Path)
		if detail == "" {
			detail = candidate.Identity
		}
		candidates[index] = launcherview.FormAppCandidate{
			Name: candidate.Name, Identity: candidate.Identity, Detail: detail, Icon: c.imageForSurface(candidate.Icon, physicalImageSize(28, imageScale), settingsPalette().Background),
		}
	}
	theme := palette
	cancelLabel := c.language.translate("i18n:ui_cancel")
	confirmLabel := c.language.translate("i18n:ui_ok")
	return launcherview.FormAppPickerView(launcherview.FormAppPickerProps{
		OverlayWidth: width, OverlayHeight: height, Window: c.formTables.formTableNativeWindow(), Theme: theme,
		Title:             c.language.translate("i18n:plugin_window_manager_group_app_selector_title"),
		SearchPlaceholder: c.language.translate("i18n:ui_hotkey_ignore_apps_search"),
		LoadingLabel:      c.language.translate("i18n:ui_hotkey_ignore_apps_loading"), EmptyLabel: c.language.translate("i18n:ui_hotkey_ignore_apps_empty"),
		CancelLabel: cancelLabel, ConfirmLabel: confirmLabel, CancelWidth: c.formTables.formTableButtonWidth(cancelLabel, 70), ConfirmWidth: c.formTables.formTableButtonWidth(confirmLabel, 70),
		Candidates: candidates, SelectedIdentity: current.Identity,
		Loading: c.hotkeySettings().AppsLoading() || (!c.hotkeySettings().AppsLoaded() && c.hotkeySettings().AppsError() == ""),
		Error:   c.hotkeySettings().AppsError(),
		OnConfirm: func(index int) {
			if index < 0 || index >= len(apps) {
				c.closeWindowManagerGroupAppPicker()
				return
			}
			c.chooseWindowManagerGroupApp(apps[index])
		},
		OnCancel: c.closeWindowManagerGroupAppPicker,
	})
}

func (c *windowGroupEditorController) buildWindowManagerGroupUrlEditor(urls []string, editor *windowGroupEditorSnapshot, palette woxcomponent.ControlTheme, width, height, imageScale float32) woxwidget.Widget {
	theme := palette
	foreground := theme.TextSecondary

	return launcherview.WindowGroupUrlEditor(launcherview.WindowGroupUrlEditorProps{
		Width: width, Height: height, URLs: urls, Window: c.formTables.formTableNativeWindow(), Theme: theme,
		Title: c.language.translate("i18n:plugin_window_manager_group_browser_urls"), Description: c.language.translate("i18n:plugin_window_manager_group_browser_urls_description"),
		CancelLabel: c.language.translate("i18n:ui_cancel"), SaveLabel: c.language.translate("i18n:ui_save"), AddLabel: c.language.translate("i18n:ui_add"), EditLabel: c.language.translate("i18n:ui_setting_theme_edit"),
		DeleteLabel: c.language.translate("i18n:ui_delete"), OperationLabel: c.language.translate("i18n:ui_operation"), DeleteConfirmation: c.language.translate("i18n:ui_delete_row_confirm"), RequiredLabel: c.language.translate("i18n:ui_validator_value_can_not_be_empty"),
		ExtensionChecking: editor.extensionChecking, ExtensionConnected: editor.extensionConnected,
		ExtensionConnectedLabel: c.language.translate("i18n:plugin_window_manager_group_browser_extension_connected"), ExtensionDisconnectedLabel: c.language.translate("i18n:plugin_window_manager_group_browser_extension_not_connected"),
		ExtensionInstallLabel: c.language.translate("i18n:plugin_window_manager_group_browser_extension_install"),
		AddIcon:               c.imageForTint(settingControlIconSource("add"), &foreground, physicalImageSize(15, imageScale)), EditIcon: c.imageForTint(settingControlIconSource("edit"), &foreground, physicalImageSize(16, imageScale)),
		DeleteIcon: c.imageForTint(settingControlIconSource("delete"), &foreground, physicalImageSize(16, imageScale)), EmptyIcon: c.imageForTint(settingControlIconSource("inbox"), &foreground, physicalImageSize(24, imageScale)),
		ExtensionLoadingIcon: c.imageForTint(settingControlIconSource("loading"), &foreground, physicalImageSize(14, imageScale)), ExtensionConnectedIcon: c.imageForTint(settingControlIconSource("check-circle"), &palette.Success, physicalImageSize(14, imageScale)),
		ExtensionDisconnectedIcon: c.imageForTint(settingControlIconSource("warning"), &palette.Warning, physicalImageSize(14, imageScale)), ExtensionExternalIcon: c.imageForTint(settingControlIconSource("external"), &palette.Warning, physicalImageSize(11, imageScale)),
		ExtensionConnectedAccent: palette.Success, ExtensionDisconnectedAccent: palette.Warning,
		OnCancel: c.cancelWindowManagerGroupUrlEditor,
		OnSave:   c.saveWindowManagerGroupUrls, OnOpenExtensionStore: c.openWindowManagerExtensionStore,
	})
}
