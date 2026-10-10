package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestDataLogLevelUsesSharedAnchoredDropdown(t *testing.T) {
	var openedAt woxui.Rect
	field := dataLogLevelField(DataSettingsProps{
		LogLevel: "DEBUG",
		Theme:    woxcomponent.ControlTheme{},
		OnOpenLogLevel: func(anchor woxui.Rect) {
			openedAt = anchor
		},
	}, 800)

	container := field.(woxwidget.Container)
	row := container.Child.(woxwidget.Flex)
	keyed := row.Children[1].(woxwidget.Keyed)
	if keyed.Key != SettingChoiceAnchorKey("LogLevel") {
		t.Fatalf("dropdown key = %q, want LogLevel choice anchor", keyed.Key)
	}
	semantics := keyed.Child.(woxwidget.Semantics)
	if semantics.AutomationID != "data-log-level" || semantics.Role != woxui.AccessibilityRoleButton {
		t.Fatalf("dropdown semantics = %#v, want standard button", semantics)
	}
	trigger := focusedControlGesture(semantics)
	if trigger.OnTap != nil || trigger.OnTapBounds == nil {
		t.Fatal("log level should open an anchored dropdown instead of changing directly")
	}
	anchor := woxui.Rect{X: 10, Y: 20, Width: 280, Height: 34}
	trigger.OnTapBounds(anchor)
	if openedAt != anchor {
		t.Fatalf("opened anchor = %#v, want %#v", openedAt, anchor)
	}
}

func TestDataRestoreErrorStaysWithBackupList(t *testing.T) {
	message := "Could not restore backup: rename C:\\Users\\qianl\\.wox\\wox-user: Access is denied."
	props := DataSettingsProps{
		Width: 960, Height: 720,
		Labels: DataSettingsLabels{
			StorageSection: "STORAGE", BackupSection: "BACKUP", LogsSection: "LOGS",
			BackupListTitle: "Backup List",
		},
		Backups:      []DataBackup{{ID: "backup-1", Timestamp: 1, Type: "auto"}},
		Error:        message,
		ErrorSection: dataErrorSectionBackup,
	}
	page := dataSettingsScrollProps(t, DataSettingsView(props))
	if page.KeepVisibleKey != dataSettingsErrorKey(props) {
		t.Fatalf("keep visible = %q, want the restore error", page.KeepVisibleKey)
	}
	children := page.Content.(woxwidget.Container).Child.(woxwidget.Flex).Children
	errorIndex := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
		keyed, ok := widget.(woxwidget.Keyed)
		return ok && keyed.Key == dataSettingsErrorKey(props)
	})
	backupIndex := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
		return dataWidgetHasKey(widget, "data-backups-grid")
	})
	logsIndex := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
		return dataWidgetHasText(widget, "LOGS")
	})
	if errorIndex != backupIndex+1 || logsIndex <= errorIndex {
		t.Fatalf("error index = %d, backup = %d, logs = %d, want the error directly under the backup list", errorIndex, backupIndex, logsIndex)
	}
	keyed := children[errorIndex].(woxwidget.Keyed)
	semantics := keyed.Child.(woxwidget.Semantics)
	if semantics.AutomationID != "data-settings-error" || semantics.LiveRegion != woxui.AccessibilityLiveRegionPolite || semantics.Value != message {
		t.Fatalf("error semantics = %#v", semantics)
	}
	text := semantics.Child.(woxwidget.Container).Child.(woxwidget.TextBlock)
	if text.MaxLines < 2 || text.Color != props.Theme.Error {
		t.Fatalf("error text = max lines %d color %#v, want a wrapping error", text.MaxLines, text.Color)
	}
}

func TestDataBackupRestoreConfirmsOnTheButton(t *testing.T) {
	state := &dataBackupRestoreState{}
	if state.advance() {
		t.Fatal("first restore activation must only enter confirmation")
	}
	if !state.confirm {
		t.Fatal("first restore activation did not retain confirmation")
	}
	state.confirm = false
	if state.confirm {
		t.Fatal("leaving the button must clear confirmation")
	}
	state.confirm = true
	if !state.advance() || state.confirm {
		t.Fatal("second activation must restore and clear confirmation")
	}
}

func TestDataBackupRestoreLeavesConfirmationOnPointerExit(t *testing.T) {
	restored := ""
	button := dataBackupRestoreButton(DataSettingsProps{
		Labels:          DataSettingsLabels{BackupRestore: "Restore", BackupRestoreConfirm: "Confirm"},
		Theme:           woxcomponent.ControlTheme{Warning: woxui.Color{R: 253, G: 186, B: 116, A: 255}, Text: woxui.Color{A: 255}},
		OnRestoreBackup: func(id string) { restored = id },
	}, DataBackup{ID: "backup-1"}, 0)
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return button })
	host.AttachServices(settingsWindowHostServices{})
	defer host.Dispose()
	frame := woxui.FrameInfo{Size: woxui.Size{Width: 240, Height: 40}, PixelSize: woxui.PixelSize{Width: 240, Height: 40}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)

	bounds, ok := host.BoundsForKey(woxwidget.Key("data-backup-restore-0"))
	if !ok {
		t.Fatal("restore button has no bounds")
	}
	point := woxui.Point{X: bounds.X + bounds.Width/2, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerMove, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	host.Frame(&woxui.DisplayList{}, frame)
	if label := dataAutomationLabel(host, "data-backup-restore-0"); label != "Confirm" {
		t.Fatalf("label after first click = %q, want Confirm", label)
	}
	if restored != "" {
		t.Fatalf("restore started after the first click: %q", restored)
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerLeave, Position: point})
	host.Frame(&woxui.DisplayList{}, frame)
	if label := dataAutomationLabel(host, "data-backup-restore-0"); label != "Restore" {
		t.Fatalf("label after pointer leave = %q, want Restore", label)
	}
	if restored != "" {
		t.Fatalf("pointer leave started restore: %q", restored)
	}

	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	host.Frame(&woxui.DisplayList{}, frame)
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	if restored != "backup-1" {
		t.Fatalf("second click restored %q, want backup-1", restored)
	}
}

func dataAutomationLabel(host *woxwidget.Host, id string) string {
	for _, node := range host.Snapshot().Tree.Nodes {
		if node.AutomationID == id {
			return node.Label
		}
	}
	return ""
}

func TestDataErrorsStayWithTheirSection(t *testing.T) {
	base := DataSettingsProps{
		Width: 960, Height: 720,
		Labels: DataSettingsLabels{StorageSection: "STORAGE", BackupSection: "BACKUP", LogsSection: "LOGS"},
	}
	cases := []struct {
		section string
		before  string
		after   string
	}{
		{section: dataErrorSectionStorage, before: "STORAGE", after: "BACKUP"},
		{section: dataErrorSectionLogs, before: "LOGS", after: ""},
		{section: "", before: "", after: "STORAGE"},
	}
	for _, tc := range cases {
		props := base
		props.Error = "failed"
		props.ErrorSection = tc.section
		children := dataSettingsContent(props, 800)
		errorIndex := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
			keyed, ok := widget.(woxwidget.Keyed)
			return ok && keyed.Key == dataSettingsErrorKey(props)
		})
		if tc.before != "" {
			before := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
				return dataWidgetHasText(widget, tc.before)
			})
			if errorIndex <= before {
				t.Fatalf("section %q error index = %d, %s = %d", tc.section, errorIndex, tc.before, before)
			}
		}
		if tc.after != "" {
			after := dataChildIndex(t, children, func(widget woxwidget.Widget) bool {
				return dataWidgetHasText(widget, tc.after)
			})
			if errorIndex >= after {
				t.Fatalf("section %q error index = %d, %s = %d", tc.section, errorIndex, tc.after, after)
			}
		} else if errorIndex != len(children)-1 {
			t.Fatalf("logs error index = %d, want last child %d", errorIndex, len(children)-1)
		}
	}
}

func dataSettingsScrollProps(t *testing.T, page woxwidget.Widget) woxcomponent.ScrollViewProps {
	t.Helper()
	container, ok := page.(woxwidget.Container)
	if !ok {
		t.Fatalf("page = %T", page)
	}
	scroll, ok := container.Child.(woxwidget.Stateful)
	if !ok {
		t.Fatalf("scroll = %T", container.Child)
	}
	props, ok := scroll.Widget.(woxcomponent.ScrollViewProps)
	if !ok {
		t.Fatalf("scroll widget = %T", scroll.Widget)
	}
	return props
}

func dataChildIndex(t *testing.T, children []woxwidget.Widget, match func(woxwidget.Widget) bool) int {
	t.Helper()
	for index, child := range children {
		if match(child) {
			return index
		}
	}
	t.Fatal("matching data page child not found")
	return -1
}

func dataWidgetHasText(widget woxwidget.Widget, value string) bool {
	found := false
	dataWalkWidget(widget, func(node woxwidget.Widget) bool {
		switch typed := node.(type) {
		case woxwidget.Text:
			if typed.Value == value {
				found = true
				return false
			}
		case woxwidget.TextBlock:
			if typed.Value == value {
				found = true
				return false
			}
		case woxwidget.Semantics:
			if typed.Label == value || typed.Value == value {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func dataWidgetHasKey(widget woxwidget.Widget, key woxwidget.Key) bool {
	found := false
	dataWalkWidget(widget, func(node woxwidget.Widget) bool {
		switch typed := node.(type) {
		case woxwidget.Keyed:
			if typed.Key == key {
				found = true
				return false
			}
		case woxwidget.Stateful:
			if typed.Key == key {
				found = true
				return false
			}
		}
		return true
	})
	return found
}

func dataWalkWidget(widget woxwidget.Widget, visit func(woxwidget.Widget) bool) {
	if widget == nil || !visit(widget) {
		return
	}
	switch node := widget.(type) {
	case woxwidget.Container:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Flex:
		for _, child := range node.Children {
			dataWalkWidget(child, visit)
		}
	case woxwidget.Expanded:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Align:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Keyed:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Semantics:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Focusable:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Gesture:
		dataWalkWidget(node.Child, visit)
	case woxwidget.Stack:
		for _, child := range node.Children {
			dataWalkWidget(child.Child, visit)
		}
	case woxwidget.Stateful:
		if child, ok := node.Widget.(woxwidget.Widget); ok {
			dataWalkWidget(child, visit)
		}
	}
}

func TestDataBackupTableKeepsOperationColumnInsideNarrowViewport(t *testing.T) {
	table := dataBackupTable(DataSettingsProps{Labels: DataSettingsLabels{BackupListTitle: "Backups"}, Backups: []DataBackup{{ID: "backup-fixture"}}}, 880)
	content := table.(woxwidget.Container).Child.(woxwidget.Flex)
	grid := content.Children[1].(woxwidget.Stateful).Widget.(formTableGridProps)
	widths := formTableColumnWidthsWithOperation(grid.field.Columns, grid.width, false)
	totalWidth := float32(0)
	for _, width := range widths {
		totalWidth += width
	}
	if totalWidth > grid.width {
		t.Fatalf("backup table declared width = %.0f, want no wider than viewport %.0f", totalWidth, grid.width)
	}
	if got := grid.field.Columns[2].Width; got != dataBackupOperationColumnWidth {
		t.Fatalf("backup operation column width = %.0f, want %.0f", got, dataBackupOperationColumnWidth)
	}
}

func TestDataStorageFieldUsesIntrinsicButtonWidths(t *testing.T) {
	field := dataStorageField(DataSettingsProps{
		Labels: DataSettingsLabels{
			Open:           "Open",
			LocationChange: "Change Location Path",
			LocationTitle:  "Location",
		},
	}, 820).(woxwidget.Container)

	row := field.Child.(woxwidget.Flex)
	label := row.Children[0].(woxwidget.Expanded)
	actionsContainer := row.Children[1].(woxwidget.Container)
	actions := actionsContainer.Child.(woxwidget.Flex)
	changeButton := focusedControlGesture(actions.Children[1]).Child.(woxwidget.Container)

	if actions.MainAxisAlignment != woxwidget.MainAxisStart {
		t.Fatalf("storage actions alignment = %v, want intrinsic start", actions.MainAxisAlignment)
	}
	if changeButton.Width != 0 || actionsContainer.Width != 0 {
		t.Fatalf("storage widths = button %.0f/container %.0f, want intrinsic sizing", changeButton.Width, actionsContainer.Width)
	}
	if label.Child.(woxwidget.Container).Width != 0 {
		t.Fatal("storage label should use the remaining field width")
	}
}

func TestDataLogActionsAreRightAligned(t *testing.T) {
	field := dataLogActionsField(DataSettingsProps{
		Labels: DataSettingsLabels{
			LogClearTitle:  "Clear logs",
			LogClearButton: "Clear",
			LogOpenButton:  "Open log file",
		},
	}, 820).(woxwidget.Container)

	row := field.Child.(woxwidget.Flex)
	actionsContainer := row.Children[1].(woxwidget.Container)
	actions := actionsContainer.Child.(woxwidget.Flex)
	if actions.MainAxisAlignment != woxwidget.MainAxisStart {
		t.Fatalf("log actions alignment = %v, want intrinsic start", actions.MainAxisAlignment)
	}
	clearButton := focusedControlGesture(actions.Children[0]).Child.(woxwidget.Container)
	openButton := focusedControlGesture(actions.Children[1]).Child.(woxwidget.Container)
	if clearButton.Width != 0 || openButton.Width != 0 || actionsContainer.Width != 0 {
		t.Fatalf("log widths = %.0f/%.0f/container %.0f, want intrinsic sizing", clearButton.Width, openButton.Width, actionsContainer.Width)
	}
}

func TestDataLogOpenButtonAcceptsClicksAcrossIntrinsicBounds(t *testing.T) {
	openTaps := 0
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return dataLogActionsField(DataSettingsProps{
			Labels:    DataSettingsLabels{LogClearTitle: "Clear logs", LogClearButton: "Clear", LogOpenButton: "Open log file"},
			OnOpenLog: func() { openTaps++ },
		}, 820)
	})
	host.AttachServices(settingsWindowHostServices{})
	defer host.Dispose()
	host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: 820, Height: 66}, PixelSize: woxui.PixelSize{Width: 820, Height: 66}, Scale: 1})

	bounds, ok := host.BoundsForKey(woxwidget.Key("data-log-open"))
	if !ok || bounds.Width <= 2 {
		t.Fatalf("open button bounds = %+v, want an intrinsic clickable area", bounds)
	}
	point := woxui.Point{X: bounds.X + 1, Y: bounds.Y + bounds.Height/2}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: point})
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: point})
	if openTaps != 1 {
		t.Fatalf("open button taps = %d at %+v, want 1 from its padding area", openTaps, bounds)
	}
}
