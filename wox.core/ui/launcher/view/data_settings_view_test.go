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
		Theme:    woxcomponent.Theme{},
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

func TestDataBackupTableKeepsOperationColumnInsideNarrowViewport(t *testing.T) {
	table := dataBackupTable(DataSettingsProps{Labels: DataSettingsLabels{BackupListTitle: "Backups"}}, 880)
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
