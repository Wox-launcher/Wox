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
		PlanTips:   "Free supports up to 2 devices and manual sync only. Pro allows unlimited devices and automatic sync.",
		InfoIcon:   icon,
		OnPlanTooltip: func(inside bool, bounds woxui.Rect) {
			gotInside = inside
			gotBounds = bounds
		},
	}, 830, 0, woxcomponent.ControlTheme{}).(woxwidget.Container)

	column := card.Child.(woxwidget.Flex)
	planRow := column.Children[1].(woxwidget.Container).Child.(woxwidget.Flex)
	planLabel := planRow.Children[0].(woxwidget.Container).Child.(woxwidget.Constrained).Child.(woxwidget.Flex)
	heading := planLabel.Children[0].(woxwidget.Flex)
	tooltip := heading.Children[1].(woxwidget.Semantics).Child.(woxwidget.Gesture)
	bounds := woxui.Rect{X: 12, Y: 18, Width: 14, Height: 14}
	tooltip.OnHoverAt(true, bounds)

	if !gotInside || gotBounds != bounds {
		t.Fatalf("tooltip hover = (%v, %+v), want (true, %+v)", gotInside, gotBounds, bounds)
	}
}

func TestCloudAccountHelpTextWrapsInsteadOfClipping(t *testing.T) {
	const planTips = "Free supports up to 2 devices and manual sync only. Pro allows unlimited devices and automatic sync."
	const billingTips = "For any billing issue, email billing@woxlauncher.com and we will help resolve it."
	card := cloudAccountCard(CloudAccountProps{
		LoggedIn:     true,
		LabelWidth:   520,
		PlanLabel:    "Plan",
		PlanTips:     planTips,
		BillingLabel: "Billing Help",
		BillingTips:  billingTips,
		SupportLabel: "Contact Support",
	}, 830, 0, woxcomponent.ControlTheme{}).(woxwidget.Container)
	if card.Height != 0 {
		t.Fatalf("account card height = %v, want intrinsic height so wrapped tips are not clipped", card.Height)
	}

	column := card.Child.(woxwidget.Flex)
	assertWrappingHelp := func(name string, field woxwidget.Widget, want string, labelWidth float32) {
		t.Helper()
		row := field.(woxwidget.Container)
		if row.Height != 0 {
			t.Fatalf("%s row height = %v, want intrinsic wrap", name, row.Height)
		}
		if row.Padding.Bottom != cloudHelpRowBottom {
			t.Fatalf("%s bottom gap = %v, want %v so wrapped copy does not sit on the next title", name, row.Padding.Bottom, cloudHelpRowBottom)
		}
		label := row.Child.(woxwidget.Flex).Children[0].(woxwidget.Container).Child.(woxwidget.Constrained).Child.(woxwidget.Flex)
		tips := label.Children[1].(woxwidget.TextBlock)
		if tips.Value != want || tips.Width != labelWidth || tips.Height != 0 {
			t.Fatalf("%s tips = %q width %v height %v, want wrapping TextBlock", name, tips.Value, tips.Width, tips.Height)
		}
	}
	assertWrappingHelp("plan", column.Children[1], planTips, 520)
	assertWrappingHelp("billing", column.Children[2], billingTips, 520)

}

func TestCloudWideFormActionsEndAtContentEdge(t *testing.T) {
	const width = float32(830)
	const labelWidth = float32(520)
	const gap = float32(32)

	accountCard := cloudAccountCard(CloudAccountProps{LoggedIn: true, LabelWidth: labelWidth, SupportLabel: "Contact Support", SupportIcon: &woxui.Image{}}, width, 0, woxcomponent.ControlTheme{}).(woxwidget.Container)
	accountColumn := accountCard.Child.(woxwidget.Flex)
	billingRow := accountColumn.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	supportValue := billingRow.Children[1].(woxwidget.Align)
	if got := labelWidth + gap + supportValue.Width; got != width {
		t.Fatalf("billing row width = %v, want %v", got, width)
	}
	if supportValue.Horizontal != 1 {
		t.Fatal("support button should stay right-aligned")
	}
	supportButton := focusedControlGesture(supportValue.Child).Child.(woxwidget.Container)
	if supportButton.Width != 0 {
		t.Fatalf("support button width = %v, want content-sized", supportButton.Width)
	}

}

func TestCloudSettingsActionsUseSharedButtonHeight(t *testing.T) {
	theme := woxcomponent.ControlTheme{}
	buttonHeight := func(widget woxwidget.Widget) float32 {
		return focusedControlGesture(widget).Child.(woxwidget.Container).Height
	}

	account := cloudAccountCard(CloudAccountProps{
		LoggedIn: true, LabelWidth: 520, SupportLabel: "Contact", SupportIcon: &woxui.Image{}, ActionWidth: 120,
	}, 830, 0, theme).(woxwidget.Container)
	accountRows := account.Child.(woxwidget.Flex)
	billingRow := accountRows.Children[2].(woxwidget.Container).Child.(woxwidget.Flex)
	if got := buttonHeight(billingRow.Children[1].(woxwidget.Align).Child); got != 32 {
		t.Fatalf("support button height = %v, want shared 32", got)
	}

	syncCard := cloudSyncCard(CloudSyncProps{ButtonLabel: "Sync", ActionWidth: 120}, 830, theme).(woxwidget.Container)
	syncRow := syncCard.Child.(woxwidget.Flex)
	if got := buttonHeight(syncRow.Children[2]); got != 32 {
		t.Fatalf("sync button height = %v, want shared 32", got)
	}

	deviceHeader := cloudDeviceHeader(CloudDevicesProps{RefreshLabel: "Refresh", RefreshIcon: &woxui.Image{}, ActionWidth: 120}, 830, theme).(woxwidget.Container)
	section := deviceHeader.Child.(woxwidget.Flex).Children[1].(woxwidget.Container)
	deviceRow := section.Child.(woxwidget.Flex)
	if section.Padding.Top != theme.Scaled(16) || deviceRow.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatal("refresh must be centered beside the complete title and help block with space below the divider")
	}
	if got := buttonHeight(deviceRow.Children[1]); got != 32 {
		t.Fatalf("refresh button height = %v, want shared 32", got)
	}
	for _, button := range []woxwidget.Widget{billingRow.Children[1].(woxwidget.Align).Child, syncRow.Children[2], deviceRow.Children[1]} {
		if got := focusedControlGesture(button).Child.(woxwidget.Container).Width; got != 120 {
			t.Fatalf("cloud action width = %v, want shared width 120", got)
		}
	}
}

func TestCloudRefreshButtonWidthIncludesLeadingIcon(t *testing.T) {
	plain := cloudFormButtonWidth("Refresh", false)
	withIcon := cloudFormButtonWidth("Refresh", true)
	if withIcon <= plain {
		t.Fatalf("refresh width with icon = %v, want greater than plain width %v", withIcon, plain)
	}
	if withIcon-plain != 24 {
		t.Fatalf("refresh width delta = %v, want 24", withIcon-plain)
	}
	cjkWithIcon := cloudFormButtonWidth("刷新", true)
	if cjkWithIcon < 80 {
		t.Fatalf("CJK refresh width with icon = %v, want at least 80", cjkWithIcon)
	}
}

func TestCloudActionMenuUsesPaddedContentSize(t *testing.T) {
	menu := cloudActionMenu(CloudActionMenuProps{Items: []CloudActionMenuItemProps{{ID: "account", Label: "Account", OnTap: func() {}}, {ID: "logout", Label: "Log out", OnTap: func() {}}}}, 200, woxcomponent.ControlTheme{}).(woxwidget.Container)
	builder := menu.Child.(woxwidget.LayoutBuilder)
	scroll := builder.Build(woxui.Size{Width: 188, Height: 80}).(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	rowWidget := scroll.Content.(woxwidget.Flex).Children[0]
	gesture := focusedControlGesture(rowWidget)
	row := gesture.Child.(woxwidget.Container)
	if scroll.Width != 188 || scroll.Height != 80 || row.Width != 188 {
		t.Fatalf("cloud action content = scroll %.0fx%.0f row %.0f, want 188x80/188", scroll.Width, scroll.Height, row.Width)
	}
	if gesture.OnHoverAt == nil {
		t.Fatal("cloud action menu item should expose hover feedback")
	}
}

func TestCloudAccountActionsUseCenteredSharedDropdownIndicator(t *testing.T) {
	theme := woxcomponent.ControlTheme{TextSecondary: woxui.Color{A: 255}}
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

func TestCloudPlanHeaderOmitsRecommendedBadge(t *testing.T) {
	header := cloudPlanHeader(CloudIntroProps{FreeLabel: "Free", ProLabel: "Pro"}, 560, false, woxcomponent.ControlTheme{}).(woxwidget.Container)
	columns := header.Child.(woxwidget.Stack).Children[1].Child.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(columns) != 3 {
		t.Fatalf("plan header columns = %d, want spacer plus Free and Pro labels", len(columns))
	}
	free := columns[1].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	pro := columns[2].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if free.Value != "Free" || pro.Value != "Pro" {
		t.Fatalf("plan header labels = %q / %q, want Free / Pro", free.Value, pro.Value)
	}
	for _, label := range []woxwidget.TextBlock{free, pro} {
		if label.LineHeight != label.Height || label.Height != tableSurfaceHeaderHeight || label.AlignmentY != 0.5 || label.MaxLines != 1 {
			t.Fatal("plan header labels must center within the full header row")
		}
	}
	if _, isBadgeRow := columns[2].(woxwidget.Expanded).Child.(woxwidget.Flex); isBadgeRow {
		t.Fatal("Pro column should not wrap a recommended badge")
	}
}

func TestCloudPlanTooltipOverlayOccupiesOnlyItsVisiblePanel(t *testing.T) {
	overlay, left, top := CloudPlanTooltipOverlay(
		CloudIntroProps{FreeLabel: "Free", ProLabel: "Pro"},
		woxui.Rect{X: 308, Y: 192, Width: 14, Height: 14},
		1152,
		768,
		woxcomponent.ControlTheme{},
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
		Width: 1200, Height: 800, PanelWidth: 648, PanelHeight: CloudPluginExclusionDialogHeight, FieldLabel: "Plugin", Selected: "plugin-a", SelectedName: "Plugin A",
		SelectedIcon: selectedIcon, CancelLabel: "Cancel", SaveLabel: "Save", Theme: woxcomponent.ControlTheme{}, OnCancel: func() {}, OnSave: func() {},
	}).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	if props.Width != 648 || props.Height != CloudPluginExclusionDialogHeight || props.Radius != 20 {
		t.Fatalf("dialog geometry = %vx%v radius %v, want 648x%v radius 20", props.Width, props.Height, props.Radius, CloudPluginExclusionDialogHeight)
	}
	if props.Padding != (woxwidget.Insets{Left: 24, Top: 24, Right: 24, Bottom: 24}) {
		t.Fatalf("dialog padding = %+v, want 24px all around", props.Padding)
	}
	child := props.Child.(woxwidget.Flex)
	if len(child.Children) != 2 || child.Gap != 12 {
		t.Fatalf("dialog content = %d children gap %v, want field/actions with 12px gap", len(child.Children), child.Gap)
	}
	field := child.Children[0].(woxwidget.Container)
	actionsFooter := child.Children[1].(woxwidget.Container)
	if actionsFooter.Height != SettingsDialogActionsHeight || actionsFooter.Padding.Top != SettingsDialogActionsHeight-settingsDialogActionHeight {
		t.Fatalf("plugin exclusion footer = height %v padding %+v, want shared settings actions", actionsFooter.Height, actionsFooter.Padding)
	}
	actions := actionsFooter.Child.(woxwidget.Align)
	if actions.Horizontal != 1 || actions.Height != settingsDialogActionHeight {
		t.Fatalf("plugin exclusion actions = height %v alignment %v, want right-aligned compact row", actions.Height, actions.Horizontal)
	}
	if field.Height+child.Gap+actionsFooter.Height+props.Padding.Top+props.Padding.Bottom != props.Height {
		t.Fatal("plugin exclusion dialog should not reserve extra space below its actions")
	}
	fieldLayout := field.Child.(woxwidget.Flex)
	selectSemantics := fieldLayout.Children[1].(woxwidget.Flex).Children[0].(woxwidget.Semantics)
	selectTrigger := focusedControlGesture(selectSemantics).Child.(woxwidget.Container)
	selectContent := selectTrigger.Child.(woxwidget.Flex)
	if selectContent.Children[0].(woxwidget.Align).Child.(woxwidget.Image).Source != selectedIcon {
		t.Fatal("selected plugin icon is not forwarded to the closed dropdown")
	}

	empty := cloudPluginExclusionsCard(CloudPluginExclusionsProps{
		SectionLabel: "Plugin Sync Exclusions", Tips: "Plugins added to this table will not sync their data or settings.",
		AddLabel: "Add", ColumnLabel: "Plugin",
	}, 700, FormTableFieldHeight(true, "Plugins added to this table will not sync their data or settings.", 0, 260), woxcomponent.ControlTheme{})
	emptyChildren := empty.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(emptyChildren) != 1 {
		t.Fatalf("empty exclusions children = %d, want only the header", len(emptyChildren))
	}

	rowIcon := &woxui.Image{}
	card := cloudPluginExclusionsCard(CloudPluginExclusionsProps{
		SectionLabel: "Exclusions", ColumnLabel: "Plugin", Tips: "Tips", Items: []CloudPluginExclusionProps{{Name: "Plugin A", Icon: rowIcon}},
	}, 700, 140, woxcomponent.ControlTheme{}).(woxwidget.Container)
	cardFlex := card.Child.(woxwidget.Flex)
	grid := cardFlex.Children[1].(woxwidget.Stateful).Widget.(formTableGridProps)
	if grid.field.Rows[0].Cells[0].Icon != rowIcon || grid.field.Rows[0].Cells[0].IconSize != 18 {
		t.Fatal("plugin table row does not preserve the 18px plugin icon")
	}

	choiceDialog := CloudPluginExclusionDialog(CloudPluginExclusionDialogProps{
		Width: 1200, Height: 800, PanelWidth: 648, PanelHeight: CloudPluginExclusionDialogHeight, FieldLabel: "Plugin", Selected: "plugin-a", SelectedName: "Plugin A", ChoiceOpen: true,
		Choices: []SettingsChoice{{Value: "plugin-a", Label: "Plugin A"}}, Theme: woxcomponent.ControlTheme{}, OnCancel: func() {}, OnSave: func() {},
	}).(woxwidget.Stack)
	if len(choiceDialog.Children) != 2 {
		t.Fatalf("choice dialog layers = %d, want dialog and anchored choice menu", len(choiceDialog.Children))
	}
	choice := choiceDialog.Children[1].Child.(woxwidget.Stateful).Widget.(SettingsChoiceProps)
	if choice.ID != "cloud-plugin-exclusion-choice" || choice.CurrentValue != "plugin-a" {
		t.Fatalf("choice props = %+v, want cloud plugin selector with current value", choice)
	}
}

// TestCloudDeviceRowsKeepIdentityAndActivity verifies narrow layouts and full-date tooltips.
func TestCloudDeviceRowsKeepIdentityAndActivity(t *testing.T) {
	icon := &woxui.Image{}
	var revoked bool
	props := CloudDevicesProps{LastActiveLabel: "Last active", Items: []CloudDeviceProps{
		{ID: "current", Name: "A long current device name", CurrentLabel: "这台设备", Icon: icon, Detail: "Windows", LastSeen: "Today 00:17"},
		{ID: "revoke", Name: "arch", Detail: "Linux", Icon: icon, LastSeen: "Today 00:16", RevokeLabel: "Revoke", ShowRevoke: true, RevokeEnabled: true, OnRevoke: func() { revoked = true }},
	}}
	for _, scale := range []float32{0.9, 1, 1.1} {
		for _, width := range []float32{360, 830} {
			theme := woxcomponent.ControlTheme{DensityScale: scale}
			card := cloudDeviceCard(props, width, theme).(woxwidget.Container)
			rows := card.Child.(woxwidget.Flex)
			rowIndex := 0
			if len(rows.Children) != len(props.Items) {
				t.Fatal("device list must not reserve a separate activity header row")
			}
			row := rows.Children[rowIndex].(woxwidget.Container)
			content := row.Child.(woxwidget.Flex)
			if content.Children[0].(woxwidget.Image).Source != icon {
				t.Fatal("device row lost platform icon")
			}
			labels := content.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
			name := labels.Children[0].(woxwidget.Flex)
			tag := name.Children[1].(woxwidget.Container)
			if tag.BorderWidth != 0 || tag.Child.(woxwidget.TextBlock).Value != "这台设备" {
				t.Fatal("current device must use shared metadata tag")
			}
			// Keep the semantics inside Expanded so the probe does not change flex allocation.
			content.Children[1] = woxwidget.Expanded{Child: woxwidget.Semantics{Key: "identity", Child: labels}}
			row.Child = content
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return row })
			host.AttachServices(translatedLabelHostServices{})
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 160}, Scale: 1.5, PixelSize: woxui.PixelSize{Width: int(width * 1.5), Height: 240}})
			bounds, ok := host.BoundsForKey("identity")
			host.Dispose()
			if !ok || bounds.Width <= 0 || bounds.X+bounds.Width > width {
				t.Fatalf("identity overflow at width %v scale %v: %+v", width, scale, bounds)
			}
			other := rows.Children[rowIndex+1].(woxwidget.Container).Child.(woxwidget.Flex)
			focusedControlGesture(other.Children[len(other.Children)-1]).OnTap()
		}
	}
	if !revoked {
		t.Fatal("device revoke callback was lost")
	}
}

// TestCloudSyncStatusAndDetailRemainSeparate allows wrapped errors and progress below the state.
func TestCloudSyncStatusAndDetailRemainSeparate(t *testing.T) {
	card := cloudSyncCard(CloudSyncProps{Label: "Sync error", Detail: "A long network error that must stay readable", ButtonLabel: "Sync now"}, 360, woxcomponent.ControlTheme{}).(woxwidget.Container)
	labels := card.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
	status := labels.Children[0].(woxwidget.TextBlock)
	detail := labels.Children[1].(woxwidget.TextBlock)
	if status.Value != "Sync error" || detail.Value == "" || detail.MaxLines != 0 || card.Height != 0 {
		t.Fatal("status and detail must remain separate and allow wrapping")
	}
}

// TestCloudSignedOutIntroKeepsAuthenticationBeforePlans covers translated copy and responsive layout.
func TestCloudSignedOutIntroKeepsAuthenticationBeforePlans(t *testing.T) {
	intro := CloudIntroProps{Headline: "Keep every device in sync", Description: "Sync settings and plugin configuration across your devices.", FreeLabel: "Free", ProLabel: "Pro",
		Features: []CloudIntroFeatureProps{{Title: "Settings Sync", Description: "A longer description that must wrap without losing any feature information."}, {Title: "Plugin Config", Description: "同步插件开关和配置，减少重复设置。"}, {Title: "Encrypted Sync", Description: "Your data is encrypted locally."}},
		PlanRows: []CloudPlanRowProps{{Label: "Price", FreeValue: "$0/month", ProValue: "$1.99/month · 1 month free trial"}, {Label: "Devices", FreeValue: "Up to 2 active devices", ProValue: "Unlimited devices"}},
	}
	var login, register bool
	account := CloudAccountProps{LoginLabel: "登录", RegisterLabel: "创建账号", ActionsEnabled: true, OnLogin: func() { login = true }, OnRegister: func() { register = true }}
	for _, density := range []float32{0.9, 1, 1.1} {
		for _, width := range []float32{360, 900} {
			theme := woxcomponent.ControlTheme{DensityScale: density, Text: woxui.Color{R: 255, G: 255, B: 255, A: 255}, Border: woxui.Color{R: 180, G: 180, B: 180, A: 255}}
			root := cloudIntro(intro, account, width, theme).(woxwidget.Container)
			content := root.Child.(woxwidget.Flex)
			hero := content.Children[0].(woxwidget.Container).Child.(woxwidget.Flex)
			header := hero
			actions := hero.Children[len(hero.Children)-1].(woxwidget.Flex)
			if hero.Axis == woxwidget.Vertical {
				header = hero.Children[0].(woxwidget.Flex)
			}
			focusedControlGesture(actions.Children[0]).OnTap()
			focusedControlGesture(actions.Children[1]).OnTap()
			grid := content.Children[1].(woxwidget.Grid)
			featureRow := grid.Children[0].(woxwidget.Flex)
			if header.Gap != featureRow.Gap || header.Children[0].(woxwidget.Align).Width != featureRow.Children[0].(woxwidget.Align).Width {
				t.Fatal("hero and features must share icon and text alignment")
			}
			for _, feature := range grid.Children {
				copy := feature.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex)
				if description := copy.Children[1].(woxwidget.TextBlock); description.MaxLines != 0 || description.Height != 0 {
					t.Fatal("feature copy must wrap without truncation")
				}
			}
			plan := content.Children[2].(woxwidget.Container)
			style := newTableSurfaceStyle(theme)
			if plan.BorderWidth != tableSurfaceBorderWidth || plan.BorderColor != style.border || plan.Radius != woxcomponent.SettingsTableRadius || plan.Height != 0 || plan.Color != style.bodyBackground {
				t.Fatal("plan comparison must retain intrinsic height and use shared Settings table chrome")
			}
			rows := plan.Child.(woxwidget.Flex).Children
			if rows[1].(woxwidget.Container).Color != style.border || rows[3].(woxwidget.Container).Color != style.rowDivider {
				t.Fatal("plan separators must use the shared header and body border colors")
			}
			content.Children[2] = woxwidget.Semantics{Key: "plans", Child: plan}
			root.Child = content
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return root })
			host.AttachServices(translatedLabelHostServices{})
			host.Frame(&woxui.DisplayList{}, woxui.FrameInfo{Size: woxui.Size{Width: width, Height: 1600}, Scale: 1.5, PixelSize: woxui.PixelSize{Width: int(width * 1.5), Height: 2400}})
			loginBounds, loginOK := host.BoundsForKey("cloud-login")
			registerBounds, registerOK := host.BoundsForKey("cloud-register")
			planBounds, planOK := host.BoundsForKey("plans")
			host.Dispose()
			if right := registerBounds.X + registerBounds.Width; right < width-0.1 || right > width+0.1 {
				t.Fatalf("authentication must align with the content right edge: got %v, want %v", right, width)
			}
			if !loginOK || !registerOK || !planOK || loginBounds.Y+loginBounds.Height > planBounds.Y || registerBounds.Y+registerBounds.Height > planBounds.Y || registerBounds.X+registerBounds.Width > width || planBounds.X+planBounds.Width > width {
				t.Fatalf("width %v density %v: authentication must fit before plan comparison: login=%+v register=%+v plans=%+v", width, density, loginBounds, registerBounds, planBounds)
			}
		}
	}
	if !login || !register {
		t.Fatal("authentication callbacks were lost")
	}
}
