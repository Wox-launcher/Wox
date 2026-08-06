package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestCloudAccountPlanTooltipForwardsHover(t *testing.T) {
	var gotInside bool
	var gotBounds woxui.Rect
	icon := &woxui.Image{}
	card := cloudAccountCard(CloudAccountProps{
		LoggedIn:   true,
		LabelWidth: 520,
		PlanLabel:  "Plan",
		InfoIcon:   icon,
		OnPlanTooltip: func(inside bool, bounds woxui.Rect) {
			gotInside = inside
			gotBounds = bounds
		},
	}, 830, 162, woxcomponent.Theme{}).(woxwidget.Container)

	column := card.Child.(woxwidget.Flex)
	planRow := column.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	planLabel := planRow.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
	heading := planLabel.Children[0].(woxwidget.Flex)
	tooltip := heading.Children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	bounds := woxui.Rect{X: 12, Y: 18, Width: 14, Height: 14}
	tooltip.OnHoverAt(true, bounds)

	if !gotInside || gotBounds != bounds {
		t.Fatalf("tooltip hover = (%v, %+v), want (true, %+v)", gotInside, gotBounds, bounds)
	}
}

func TestCloudWideFormActionsEndAtContentEdge(t *testing.T) {
	const width = float32(830)
	const labelWidth = float32(520)
	const gap = float32(32)

	accountCard := cloudAccountCard(CloudAccountProps{LoggedIn: true, LabelWidth: labelWidth, SupportLabel: "Support"}, width, 162, woxcomponent.Theme{}).(woxwidget.Container)
	accountColumn := accountCard.Child.(woxwidget.Flex)
	billingRow := accountColumn.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	supportValue := billingRow.Children[1].(woxwidget.Container)
	supportRow := supportValue.Child.(woxwidget.Flex)
	supportSpacer := supportRow.Children[0].(woxwidget.Painter)
	if got := labelWidth + gap + supportValue.Width; got != width {
		t.Fatalf("billing row width = %v, want %v", got, width)
	}
	if got := supportSpacer.Width; got+112 != supportValue.Width {
		t.Fatalf("support button right edge = %v, want %v", got+112, supportValue.Width)
	}

	buttonTheme := woxcomponent.Theme{ActionSelected: woxui.Color{R: 1, A: 255}, ResultSubtitle: woxui.Color{R: 2, A: 255}}
	syncCard := cloudSyncCard(CloudSyncProps{LabelWidth: labelWidth, ButtonLabel: "Sync"}, width, buttonTheme).(woxwidget.Container)
	syncRow := syncCard.Child.(woxwidget.Flex)
	syncValue := syncRow.Children[1].(woxwidget.Container)
	if got := labelWidth + gap + syncValue.Width; got != width {
		t.Fatalf("sync row width = %v, want %v", got, width)
	}
	if got := syncValue.Padding.Left; got+64 != syncValue.Width {
		t.Fatalf("sync button right edge = %v, want %v", got+64, syncValue.Width)
	}

	deviceHeader := cloudDeviceHeader(CloudDevicesProps{LabelWidth: labelWidth, RefreshLabel: "Refresh"}, width, buttonTheme).(woxwidget.Container)
	deviceRow := deviceHeader.Child.(woxwidget.Flex)
	refreshValue := deviceRow.Children[1].(woxwidget.Container)
	if got := labelWidth + gap + refreshValue.Width; got != width {
		t.Fatalf("device row width = %v, want %v", got, width)
	}
	if got := refreshValue.Padding.Left; got+88 != refreshValue.Width {
		t.Fatalf("refresh button right edge = %v, want %v", got+88, refreshValue.Width)
	}
	syncButton := syncValue.Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	refreshButton := refreshValue.Child.(woxwidget.Semantics).Child.(woxwidget.Focusable).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	if syncButton.Color != refreshButton.Color || syncButton.BorderColor != refreshButton.BorderColor || syncButton.BorderWidth != refreshButton.BorderWidth {
		t.Fatalf("sync button surface = %+v, want refresh surface %+v", syncButton, refreshButton)
	}
}

func TestCloudAccountActionsUseCenteredSharedDropdownIndicator(t *testing.T) {
	theme := woxcomponent.Theme{ResultSubtitle: woxui.Color{A: 255}}
	action := cloudValueAction("cloud-account-action", "account@example.com", 260, func() {}, theme).(woxwidget.Align)
	if action.Horizontal != 1 || action.Vertical != 0.5 {
		t.Fatalf("account action alignment = (%v, %v), want (1, 0.5)", action.Horizontal, action.Vertical)
	}
	content := action.Child.(woxwidget.Flex)
	if content.Gap != 6 || content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("account action content = gap %v alignment %v, want gap 6 and centered", content.Gap, content.CrossAxisAlignment)
	}
	button := content.Children[1].(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
	if button.Width != 28 || button.Height != 28 || button.HoverBackground.A == 0 || button.OnHoverAt != nil {
		t.Fatalf("account dropdown button = %+v, want hoverable 28x28 icon button without tooltip", button)
	}
	indicator := button.Icon.(woxwidget.Painter)
	if indicator.Width != 28 || indicator.Height != 28 {
		t.Fatalf("account dropdown indicator = %vx%v, want 28x28", indicator.Width, indicator.Height)
	}
}

func TestCloudPlanTooltipOverlayOccupiesOnlyItsVisiblePanel(t *testing.T) {
	overlay, left, top := CloudPlanTooltipOverlay(
		CloudIntroProps{FreeLabel: "Free", ProLabel: "Pro"},
		woxui.Rect{X: 308, Y: 192, Width: 14, Height: 14},
		1152,
		768,
		woxcomponent.Theme{},
	)
	tooltip := overlay.(woxwidget.Semantics)
	panel := tooltip.Child.(woxwidget.Container)
	if panel.Width != 580 || panel.Height != 260 {
		t.Fatalf("tooltip panel = %vx%v, want 580x260", panel.Width, panel.Height)
	}

	window := SettingsWindow(SettingsWindowProps{
		Width: 1152, Height: 768, Platform: "darwin", RailWidth: 240,
		Page: woxwidget.Painter{}, Rail: woxwidget.Painter{}, TitleBar: woxwidget.Painter{},
		Overlay: overlay, OverlayLeft: left, OverlayTop: top,
	}).(woxwidget.Semantics)
	windowContainer := window.Child.(woxwidget.Container)
	stack := windowContainer.Child.(woxwidget.Stack)
	overlayChild := stack.Children[1]
	if overlayChild.Left != left || overlayChild.Top != top {
		t.Fatalf("tooltip placement = (%v, %v), want (%v, %v)", overlayChild.Left, overlayChild.Top, left, top)
	}
	if _, fullWindowOverlay := overlayChild.Child.(woxwidget.Stack); fullWindowOverlay {
		t.Fatal("tooltip must not add a full-window hit-test layer above its hover anchor")
	}
}

func TestCloudPluginExclusionDialogUsesFlutterRowEditorChrome(t *testing.T) {
	selectedIcon := &woxui.Image{}
	dialog := CloudPluginExclusionDialog(CloudPluginExclusionDialogProps{
		Width: 1200, Height: 800, PanelWidth: 648, PanelHeight: 170, FieldLabel: "Plugin", Selected: "plugin-a", SelectedName: "Plugin A",
		SelectedIcon: selectedIcon, CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.Theme{}, OnCancel: func() {}, OnSave: func() {},
	}).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	if props.Width != 648 || props.Height != 170 || props.Radius != 20 {
		t.Fatalf("dialog geometry = %vx%v radius %v, want 648x170 radius 20", props.Width, props.Height, props.Radius)
	}
	if props.Padding != (woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}) {
		t.Fatalf("dialog padding = %+v, want 24px all around", props.Padding)
	}
	child := props.Child.(woxwidget.Flex)
	if len(child.Children) != 2 || child.Gap != 12 {
		t.Fatalf("dialog content = %d children gap %v, want field/actions with 12px gap", len(child.Children), child.Gap)
	}
	field := child.Children[0].(woxwidget.Container)
	fieldLayout := field.Child.(woxwidget.Flex)
	selectSemantics := fieldLayout.Children[1].(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	selectFocus := selectSemantics.Child.(woxwidget.Focusable)
	selectTrigger := selectFocus.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	selectContent := selectTrigger.Child.(woxwidget.Flex)
	if selectContent.Children[0].(woxwidget.Align).Child.(woxwidget.Image).Source != selectedIcon {
		t.Fatal("selected plugin icon is not forwarded to the closed dropdown")
	}

	rowIcon := &woxui.Image{}
	card := cloudPluginExclusionsCard(CloudPluginExclusionsProps{
		SectionLabel: "Exclusions", ColumnLabel: "Plugin", Tips: "Tips", Items: []CloudPluginExclusionProps{{Name: "Plugin A", Icon: rowIcon}},
	}, 700, 140, woxcomponent.Theme{}).(woxwidget.Container)
	cardFlex := card.Child.(woxwidget.Flex)
	grid := cardFlex.Children[1].(woxwidget.Stateful).Widget.(formTableGridProps)
	if grid.field.Rows[0].Cells[0].Icon != rowIcon || grid.field.Rows[0].Cells[0].IconSize != 18 {
		t.Fatal("plugin table row does not preserve the 18px plugin icon")
	}

	choiceDialog := CloudPluginExclusionDialog(CloudPluginExclusionDialogProps{
		Width: 1200, Height: 800, PanelWidth: 648, PanelHeight: 170, FieldLabel: "Plugin", Selected: "plugin-a", SelectedName: "Plugin A", ChoiceOpen: true,
		Choices: []SettingsChoice{{Value: "plugin-a", Label: "Plugin A"}}, Theme: woxcomponent.Theme{}, OnCancel: func() {}, OnSave: func() {},
	}).(woxwidget.Stack)
	if len(choiceDialog.Children) != 2 {
		t.Fatalf("choice dialog layers = %d, want dialog and anchored choice menu", len(choiceDialog.Children))
	}
	choice := choiceDialog.Children[1].Child.(woxwidget.Stateful).Widget.(SettingsChoiceProps)
	if choice.ID != "cloud-plugin-exclusion-choice" || choice.CurrentValue != "plugin-a" {
		t.Fatalf("choice props = %+v, want cloud plugin selector with current value", choice)
	}
}
