package launcher

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/google/uuid"

	launcherview "wox/ui/launcher/view"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
	utilwindow "wox/util/window"
)

const (
	windowManagerPluginID          = "5b7d9f22-4d87-4c0f-a2c1-8e2b50c8bca0"
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

func isWindowManagerGroupsEditor(a *App, state *formTableEditorState) bool {
	if state == nil || !isWindowManagerGroupsTable(state.definition) {
		return false
	}
	pluginForm := a.pluginSettings.Form()
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

func (a *App) beginWindowManagerGroupEdit(rowIndex int, rowEditorOnly bool) bool {
	state := a.activeFormTableEditor()
	if !isWindowManagerGroupsEditor(a, state) {
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
	a.loadWindowManagerDisplays(state)
	a.invalidateFormTableWindow()
	return true
}

func (a *App) loadWindowManagerDisplays(state *formTableEditorState) {
	util.Go(a.lifecycleCtx, "load window manager displays", func() {
		displays, err := utilwindow.ListDisplays()
		_ = a.runOnUI("apply window manager displays", func() {
			current := a.activeFormTableEditor()
			if current != state || current.windowGroupEditor == nil {
				return
			}
			editor := current.windowGroupEditor
			editor.loadingDisplays = false
			if err != nil {
				editor.displaysError = err.Error()
				a.invalidateFormTableWindow()
				return
			}
			utilwindow.SortDisplays(displays)
			editor.displays = displays
			editor.group.reconcileScreens(displays)
			if editor.selectedDisplay >= len(displays) {
				editor.selectedDisplay = 0
			}
			a.invalidateFormTableWindow()
		})
	})
}

func (a *App) cancelWindowManagerGroupEdit() {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	closeEditor := state.rowEditorOnly
	state.windowGroupEditor = nil
	state.rowIndex = -1
	state.rowEditorOnly = false
	if closeEditor {
		a.closeFormTableEditor()
		return
	}
	a.invalidateFormTableWindow()
}

func (a *App) saveWindowManagerGroupEdit() {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || state.invalid || state.saving {
		return
	}
	editor := state.windowGroupEditor
	name := strings.TrimSpace(editor.group.Name)
	if name == "" {
		editor.nameError = a.translate("i18n:plugin_window_manager_group_name_required")
		a.invalidateFormTableWindow()
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
	if err := a.commitFormTableRowsLocked(state); err != nil {
		state.status = err.Error()
		a.invalidateFormTableWindow()
		return
	}
	closeEditor := state.rowEditorOnly
	state.windowGroupEditor = nil
	state.rowIndex = -1
	state.rowEditorOnly = false
	state.status = ""
	if closeEditor {
		a.closeFormTableEditor()
	} else {
		a.invalidateFormTableWindow()
	}
	a.submitPluginSettings()
}

func (a *App) setWindowManagerGroupName(value string) {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	hadError := state.windowGroupEditor.nameError != ""
	state.windowGroupEditor.group.Name = value
	if strings.TrimSpace(value) != "" {
		state.windowGroupEditor.nameError = ""
	}
	if hadError && state.windowGroupEditor.nameError == "" {
		a.invalidateFormTableWindow()
	}
}

func (a *App) selectWindowManagerGroupDisplay(index int) {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil || index < 0 || index >= len(state.windowGroupEditor.displays) {
		return
	}
	state.windowGroupEditor.selectedDisplay = index
	a.invalidateFormTableWindow()
}

func (a *App) setWindowManagerGroupLayout(layoutID string) {
	state := a.activeFormTableEditor()
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
	a.invalidateFormTableWindow()
}

func (a *App) openWindowManagerGroupAppPicker(slotID string) {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.appPickerSlot = slotID
	if !a.hotkeySettings.AppsLoaded() && !a.hotkeySettings.AppsLoading() {
		util.Go(a.lifecycleCtx, "load hotkey app candidates", a.loadHotkeyAppCandidates)
	}
	a.invalidateFormTableWindow()
}

func (a *App) chooseWindowManagerGroupApp(candidate ignoredHotkeyApp) {
	state := a.activeFormTableEditor()
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
	a.invalidateFormTableWindow()
}

func (a *App) closeWindowManagerGroupAppPicker() {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.appPickerSlot = ""
	a.invalidateFormTableWindow()
}

func (a *App) openWindowManagerGroupUrlEditor(slotID string) {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.urlEditorSlot = slotID
	state.windowGroupEditor.extensionChecking = true
	state.windowGroupEditor.extensionConnected = false
	a.loadWindowManagerExtensionStatus(state)
	a.invalidateFormTableWindow()
}

// loadWindowManagerExtensionStatus resolves browser integration outside the retained build pass.
func (a *App) loadWindowManagerExtensionStatus(state *formTableEditorState) {
	util.Go(a.lifecycleCtx, "load browser extension status", func() {
		connected, _ := a.services.BrowserExtensionConnected(a.lifecycleCtx, a.sessionID)
		_ = a.runOnUI("apply browser extension status", func() {
			current := a.activeFormTableEditor()
			if current != state || current.windowGroupEditor == nil || current.windowGroupEditor.urlEditorSlot == "" {
				return
			}
			current.windowGroupEditor.extensionChecking = false
			current.windowGroupEditor.extensionConnected = connected
			a.invalidateFormTableWindow()
		})
	})
}

func (a *App) saveWindowManagerGroupUrls(urls []string) {
	state := a.activeFormTableEditor()
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
		a.invalidateFormTableWindow()
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
	a.invalidateFormTableWindow()
}

func (a *App) openWindowManagerExtensionStore() {
	if window := a.formTableNativeWindow(); window != nil {
		_ = window.OpenExternalURL(windowManagerExtensionStoreURL)
	}
}

func (a *App) cancelWindowManagerGroupUrlEditor() {
	state := a.activeFormTableEditor()
	if state == nil || state.windowGroupEditor == nil {
		return
	}
	state.windowGroupEditor.urlEditorSlot = ""
	a.invalidateFormTableWindow()
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

func (a *App) buildWindowManagerGroupEditor(snapshot *windowGroupEditorSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	if snapshot == nil {
		return woxwidget.Container{}
	}
	editor := snapshot
	iconTint := palette.resultTitle
	addIconTint := palette.resultSubtitle
	linkTint := woxui.Color{R: 110, G: 231, B: 183, A: 255}
	props := launcherview.WindowGroupEditorProps{
		Width: width, Height: height, GroupName: editor.group.Name, NameError: editor.nameError,
		LoadingDisplays: editor.loadingDisplays, DisplaysError: editor.displaysError, Editing: editor.editing,
		SelectedDisplay: editor.selectedDisplay, Theme: palette.componentTheme(), Window: a.formTableNativeWindow(),
		CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: a.translate("i18n:ui_save"),
		SelectDisplayLabel:   a.translate("i18n:plugin_window_manager_group_select_display"),
		NoDisplaysLabel:      a.translate("i18n:plugin_window_manager_group_no_displays"),
		PrimaryDisplayLabel:  a.translate("i18n:plugin_window_manager_group_display_primary"),
		LayoutsLabel:         a.translate("i18n:plugin_window_manager_group_layouts"),
		LayoutsDescription:   a.translate("i18n:plugin_window_manager_group_layouts_description"),
		ChooseAppLabel:       a.translate("i18n:plugin_window_manager_group_slot_choose_app"),
		ChangeAppLabel:       a.translate("i18n:plugin_window_manager_group_slot_change_app"),
		BrowserUrlsLabel:     a.translate("i18n:plugin_window_manager_group_browser_urls"),
		BrowserUrlEmptyLabel: "URL",
		RetryLabel:           a.translate("i18n:plugin_window_manager_group_retry"),
		AddSlotIcon:          a.imageForTint(settingControlIconSource("add-circle"), &addIconTint, physicalImageSize(15, imageScale)),
		AppsIcon:             a.imageForTint(usageIconSource("apps"), &iconTint, physicalImageSize(18, imageScale)),
		LinkIcon:             a.imageForTint(settingControlIconSource("link"), &linkTint, physicalImageSize(13, imageScale)),
		OnCancel:             a.cancelWindowManagerGroupEdit, OnSave: a.saveWindowManagerGroupEdit, OnNameChanged: a.setWindowManagerGroupName,
		OnSelectDisplay: a.selectWindowManagerGroupDisplay, OnSelectLayout: a.setWindowManagerGroupLayout,
		OnOpenAppPicker: a.openWindowManagerGroupAppPicker, OnOpenUrlEditor: a.openWindowManagerGroupUrlEditor,
	}
	if editor.editing {
		props.Title = a.translate("i18n:plugin_window_manager_group_edit_dialog_title")
	} else {
		props.Title = a.translate("i18n:plugin_window_manager_group_create_dialog_title")
	}
	props.NamePlaceholder = a.translate("i18n:plugin_window_manager_group_name")
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
		slotLabel := strings.ReplaceAll(a.translate("i18n:plugin_window_manager_group_slot_count"), "{count}", strconv.Itoa(slotCount))
		group := launcherview.WindowGroupLayoutGroupProps{SlotCountLabel: slotLabel}
		for _, layout := range layouts {
			option := launcherview.WindowGroupLayoutOptionProps{
				ID: layout.Id, Label: a.translate("i18n:" + layout.TitleKey), Selected: layout.Id == selectedLayout.Id,
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
					slotTile.AppIcon = a.imageFor(assignment.App.Icon)
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
			if current := a.activeFormTableEditor(); current != nil && current.windowGroupEditor != nil {
				current.windowGroupEditor.loadingDisplays = true
				current.windowGroupEditor.displaysError = ""
				a.loadWindowManagerDisplays(current)
				a.invalidateFormTableWindow()
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
		layers = append(layers, woxwidget.StackChild{Child: a.buildWindowManagerGroupAppPicker(current, palette, width, height, imageScale)})
	}
	if editor.urlEditorSlot != "" && selectedScreen != nil {
		if assignment := selectedScreen.assignmentFor(editor.urlEditorSlot); assignment != nil {
			layers = append(layers, woxwidget.StackChild{Child: a.buildWindowManagerGroupUrlEditor(assignment.Urls, editor, palette, width, height, imageScale)})
		}
	}
	if len(layers) == 1 {
		return layers[0].Child
	}
	return woxwidget.Stack{Width: width, Height: height, Children: layers}
}

func (a *App) buildWindowManagerGroupAppPicker(current ignoredHotkeyApp, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	apps := a.hotkeySettings.AppCandidates()
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
			Name: candidate.Name, Identity: candidate.Identity, Detail: detail, Icon: a.imageForSize(candidate.Icon, physicalImageSize(28, imageScale)),
		}
	}
	theme := palette.componentTheme()
	cancelLabel := a.translate("i18n:ui_cancel")
	confirmLabel := a.translate("i18n:ui_ok")
	return launcherview.FormAppPickerView(launcherview.FormAppPickerProps{
		OverlayWidth: width, OverlayHeight: height, Window: a.formTableNativeWindow(), Theme: theme,
		Title:             a.translate("i18n:plugin_window_manager_group_app_selector_title"),
		SearchPlaceholder: a.translate("i18n:ui_hotkey_ignore_apps_search"),
		LoadingLabel:      a.translate("i18n:ui_hotkey_ignore_apps_loading"), EmptyLabel: a.translate("i18n:ui_hotkey_ignore_apps_empty"),
		CancelLabel: cancelLabel, ConfirmLabel: confirmLabel, CancelWidth: a.formTableButtonWidth(cancelLabel, 70), ConfirmWidth: a.formTableButtonWidth(confirmLabel, 70),
		Candidates: candidates, SelectedIdentity: current.Identity,
		Loading: a.hotkeySettings.AppsLoading() || (!a.hotkeySettings.AppsLoaded() && a.hotkeySettings.AppsError() == ""),
		Error:   a.hotkeySettings.AppsError(),
		OnConfirm: func(index int) {
			if index < 0 || index >= len(apps) {
				a.closeWindowManagerGroupAppPicker()
				return
			}
			a.chooseWindowManagerGroupApp(apps[index])
		},
		OnCancel: a.closeWindowManagerGroupAppPicker,
	})
}

func (a *App) buildWindowManagerGroupUrlEditor(urls []string, editor *windowGroupEditorSnapshot, palette uiPalette, width, height, imageScale float32) woxwidget.Widget {
	theme := palette.componentTheme()
	foreground := theme.ResultSubtitle
	connectedAccent := woxui.Color{R: 74, G: 222, B: 128, A: 255}
	disconnectedAccent := woxui.Color{R: 253, G: 186, B: 116, A: 255}
	if !themeColorIsDark(palette.background) {
		connectedAccent = woxui.Color{R: 21, G: 128, B: 61, A: 255}
		disconnectedAccent = woxui.Color{R: 194, G: 65, B: 12, A: 255}
	}
	return launcherview.WindowGroupUrlEditor(launcherview.WindowGroupUrlEditorProps{
		Width: width, Height: height, URLs: urls, Window: a.formTableNativeWindow(), Theme: theme,
		Title: a.translate("i18n:plugin_window_manager_group_browser_urls"), Description: a.translate("i18n:plugin_window_manager_group_browser_urls_description"),
		CancelLabel: a.translate("i18n:ui_cancel"), SaveLabel: a.translate("i18n:ui_save"), AddLabel: a.translate("i18n:ui_add"), EditLabel: a.translate("i18n:ui_setting_theme_edit"),
		DeleteLabel: a.translate("i18n:ui_delete"), OperationLabel: a.translate("i18n:ui_operation"), EmptyLabel: a.translate("i18n:ui_no_data"), DeleteConfirmation: a.translate("i18n:ui_delete_row_confirm"), RequiredLabel: a.translate("i18n:ui_validator_value_can_not_be_empty"),
		ExtensionChecking: editor.extensionChecking, ExtensionConnected: editor.extensionConnected,
		ExtensionConnectedLabel: a.translate("i18n:plugin_window_manager_group_browser_extension_connected"), ExtensionDisconnectedLabel: a.translate("i18n:plugin_window_manager_group_browser_extension_not_connected"),
		ExtensionInstallLabel: a.translate("i18n:plugin_window_manager_group_browser_extension_install"),
		AddIcon:               a.imageForTint(settingControlIconSource("add"), &foreground, physicalImageSize(15, imageScale)), EditIcon: a.imageForTint(settingControlIconSource("edit"), &foreground, physicalImageSize(16, imageScale)),
		DeleteIcon: a.imageForTint(settingControlIconSource("delete"), &foreground, physicalImageSize(16, imageScale)), EmptyIcon: a.imageForTint(settingControlIconSource("inbox"), &foreground, physicalImageSize(24, imageScale)),
		ExtensionLoadingIcon: a.imageForTint(settingControlIconSource("loading"), &foreground, physicalImageSize(14, imageScale)), ExtensionConnectedIcon: a.imageForTint(settingControlIconSource("check-circle"), &connectedAccent, physicalImageSize(14, imageScale)),
		ExtensionDisconnectedIcon: a.imageForTint(settingControlIconSource("warning"), &disconnectedAccent, physicalImageSize(14, imageScale)), ExtensionExternalIcon: a.imageForTint(settingControlIconSource("external"), &disconnectedAccent, physicalImageSize(11, imageScale)),
		ExtensionConnectedAccent: connectedAccent, ExtensionDisconnectedAccent: disconnectedAccent,
		OnCancel: a.cancelWindowManagerGroupUrlEditor,
		OnSave:   a.saveWindowManagerGroupUrls, OnOpenExtensionStore: a.openWindowManagerExtensionStore,
	})
}
