package view

import (
	"fmt"
	"strings"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherToolbarAction contains one translated toolbar action.
type LauncherToolbarAction struct {
	ID           string
	Label        string
	HotkeyLabels []string
	Pinned       bool
	Active       bool
	OnTap        func() `boundary:"stable"`
}

// Equal compares every visual dependency for one prepared toolbar action.
func (a LauncherToolbarAction) Equal(other LauncherToolbarAction) bool {
	if a.ID != other.ID || a.Label != other.Label || a.Pinned != other.Pinned || a.Active != other.Active || len(a.HotkeyLabels) != len(other.HotkeyLabels) {
		return false
	}
	for index := range a.HotkeyLabels {
		if a.HotkeyLabels[index] != other.HotkeyLabels[index] {
			return false
		}
	}
	return true
}

// LauncherToolbarProps contains the launcher status and available result actions.
type LauncherToolbarProps struct {
	Width              float32
	Height             float32
	Padding            woxwidget.Insets
	Theme              woxcomponent.Theme
	Window             *woxui.Window
	DensityScale       float32
	Label              string
	Icon               *woxui.Image
	Progress           int
	HasProgress        bool
	Indeterminate      bool
	Actions            []LauncherToolbarAction
	StatusHotkeyLabels []string
	StatusInteractive  bool
	StatusActive       bool
	OnStatusTap        func() `boundary:"stable"`
	MenuVisible        bool
	MenuActive         bool
	MenuLabel          string
	MenuIcon           *woxui.Image
	OnMenuTap          func()                 `boundary:"stable"`
	OnMenuHoverAt      func(bool, woxui.Rect) `boundary:"stable"`
	OnDragStart        func()                 `boundary:"stable"`
}

// Equal compares every render dependency for the launcher toolbar section.
func (p LauncherToolbarProps) Equal(other LauncherToolbarProps) bool {
	if p.Width != other.Width || p.Height != other.Height || p.Padding != other.Padding || p.Theme != other.Theme || p.Window != other.Window || p.DensityScale != other.DensityScale || p.Label != other.Label || p.Icon != other.Icon || p.Progress != other.Progress || p.HasProgress != other.HasProgress || p.Indeterminate != other.Indeterminate || len(p.Actions) != len(other.Actions) || p.StatusInteractive != other.StatusInteractive || p.StatusActive != other.StatusActive || len(p.StatusHotkeyLabels) != len(other.StatusHotkeyLabels) || p.MenuVisible != other.MenuVisible || p.MenuActive != other.MenuActive || p.MenuLabel != other.MenuLabel || p.MenuIcon != other.MenuIcon {
		return false
	}
	for index := range p.StatusHotkeyLabels {
		if p.StatusHotkeyLabels[index] != other.StatusHotkeyLabels[index] {
			return false
		}
	}
	for index := range p.Actions {
		if !p.Actions[index].Equal(other.Actions[index]) {
			return false
		}
	}
	return true
}

// LauncherToolbarBoundary composes the footer. Hoverable action chrome stays
// outside a Boundary so WOX_DEBUG_REPAINT=verify does not rebuild a full-width
// strip on every caret blink.
func LauncherToolbarBoundary(props LauncherToolbarProps) woxwidget.Widget {
	return LauncherToolbarView(props)
}

const launcherToolbarMoreActionID = "result-toolbar-more"

type measuredLauncherToolbarAction struct {
	id     string
	widget woxwidget.Widget
	width  float32
	pinned bool
}

// LauncherToolbarView builds the status footer and the actions that fit its current width.
func LauncherToolbarView(props LauncherToolbarProps) woxwidget.Widget {
	layout := launcherToolbarLayoutFor(props)
	contentHeight, fontSize := layout.contentHeight, layout.fontSize
	contentWidth, leftWidth, rightWidth, labelWidth := layout.contentWidth, layout.leftWidth, layout.rightWidth, layout.labelWidth
	progressVisible := props.HasProgress || props.Indeterminate
	statusHotkey := layout.statusHotkey
	rightChildren := make([]woxwidget.Widget, 0, len(layout.shown))
	for _, action := range layout.shown {
		rightChildren = append(rightChildren, action.widget)
	}
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if props.MenuVisible {
		leftWidgets = append(leftWidgets, launcherToolbarAboutMenuButton(props, contentHeight, contentHeight))
	} else {
		if props.Icon != nil {
			iconSize := scaledLauncherSize(18, props.DensityScale)
			leftWidgets = append(leftWidgets, woxwidget.Align{
				Width: iconSize, Height: contentHeight, Vertical: 0.5, Child: woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize},
			})
		}
		if props.Label != "" && (labelWidth > 0 || !props.StatusInteractive) {
			leftWidgets = append(leftWidgets, woxwidget.Align{
				Width: labelWidth, Height: contentHeight, Vertical: 0.5,
				Child: woxwidget.TextBlock{Value: props.Label, Width: labelWidth, Height: contentHeight, LineHeight: contentHeight, MaxLines: 1, HardEllipsis: true, AlignmentY: 0.5, Style: woxui.TextStyle{Size: fontSize}, Color: props.Theme.ToolbarText},
			})
		}
		if progressVisible {
			progressSize := scaledLauncherSize(14, props.DensityScale)
			progressValue := "loading"
			if props.HasProgress && !props.Indeterminate {
				progressValue = fmt.Sprintf("%d%%", min(max(props.Progress, 0), 100))
			}
			leftWidgets = append(leftWidgets, woxwidget.Semantics{
				Key: "launcher-toolbar-progress-key", AutomationID: "launcher.toolbar.progress", Role: woxui.AccessibilityRoleProgressBar, Label: props.Label, Value: progressValue, ReadOnly: true,
				Child: woxwidget.Align{Width: progressSize, Height: contentHeight, Vertical: 0.5, Child: woxcomponent.WoxProgressIndicator(progressSize, props.Progress, props.Indeterminate, props.Theme.ToolbarText)},
			})
		}
	}
	if props.StatusInteractive && !props.MenuVisible {
		leftWidgets = append(leftWidgets, statusHotkey)
		content := leftWidgets
		leftWidgets = []woxwidget.Widget{woxcomponent.Hoverable("launcher-toolbar-status-hover", false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
			background := woxui.Color{}
			if hovered || props.StatusActive {
				background = woxcomponent.ControlHoverColor(props.Theme.ToolbarBackground, props.Theme.ToolbarText)
			}
			return woxwidget.Gesture{ID: "launcher-toolbar-status", OnTap: props.OnStatusTap, OnHoverAt: onHoverAt, Child: woxwidget.Container{Width: leftWidth, Height: contentHeight, Radius: scaledLauncherSize(4, props.DensityScale), Color: background, Padding: woxwidget.Insets{Left: scaledLauncherSize(8, props.DensityScale), Right: scaledLauncherSize(8, props.DensityScale)}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: content}}}
		})}
	}
	if props.Label != "" && !props.MenuVisible {
		role := woxui.AccessibilityRoleGroup
		var semanticActions []woxui.AccessibilityAction
		if props.StatusInteractive {
			role = woxui.AccessibilityRoleButton
			semanticActions = []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}
		}
		leftWidgets = []woxwidget.Widget{woxwidget.Semantics{
			Key: "launcher-toolbar-status-key", AutomationID: "launcher.toolbar.status", Role: role, Actions: semanticActions, Expanded: props.StatusActive, OnAction: func(action woxui.AccessibilityAction, _ string) error {
				if action != woxui.AccessibilityActionActivate || props.OnStatusTap == nil {
					return fmt.Errorf("unsupported status action %q", action)
				}
				props.OnStatusTap()
				return nil
			},
			Label: props.Label, Value: props.Label, LiveRegion: woxui.AccessibilityLiveRegionPolite,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), Children: leftWidgets},
		}}
	}
	body := woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.ToolbarBackground, Floating: true, Material: props.Theme.ToolbarMaterial, Surface: props.Theme.Surfaces.Get("Toolbar"),
		Padding: woxwidget.Insets{Left: props.Padding.Left, Right: props.Padding.Right},
		Child: woxwidget.Align{Height: props.Height, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), Children: leftWidgets}},
			woxwidget.Gesture{ID: "launcher-toolbar-drag-area", OnDragStart: props.OnDragStart, Child: woxwidget.Container{
				Width: max(float32(0), contentWidth-leftWidth-rightWidth), Height: contentHeight,
			}},
			woxwidget.Container{Width: rightWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 0, Children: rightChildren}},
		}}},
	}
	var surface woxwidget.Widget = body
	if radius := min(max(0, props.Theme.AppContentBorderRadius), min(props.Width/2, props.Height)); radius > 0 {
		// Keep both material and texture on the rounded layer; a texture on body would cover the bottom corners.
		body.Color, body.Floating, body.Surface = woxui.Color{}, false, nil
		surface = woxwidget.Clip{Width: props.Width, Height: props.Height, Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
			{Top: -radius, Child: woxwidget.Container{Width: props.Width, Height: props.Height + radius, Radius: radius, Color: props.Theme.ToolbarBackground, Floating: true, Material: props.Theme.ToolbarMaterial, Surface: props.Theme.Surfaces.Get("Toolbar")}},
			{Child: body},
		}}}
	}
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		// Block result hit targets throughout the glass, including blank gutters.
		{Child: woxwidget.Semantics{AutomationID: "launcher.toolbar", Role: woxui.AccessibilityRoleGroup, Label: "Toolbar", Child: woxwidget.Gesture{ID: "launcher-toolbar-shield", OnTap: func() {}, OnSecondaryTapDown: func(woxui.Point) {}, OnHover: func(bool) {}, OnPointer: func(woxui.PointerEvent) bool { return true }, OnScroll: func(woxui.Point) {}, Child: woxwidget.Container{Width: props.Width, Height: props.Height}}}},
		{Child: surface},
		{Child: woxwidget.Painter{Width: props.Width, Height: min(props.Height, max(float32(0), props.Theme.ToolbarBorderWidth)), Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			if props.Theme.ToolbarBorderWidth > 0 {
				displayList.FillRect(bounds, props.Theme.ToolbarBorder)
			}
		}}},
	}}
}

// launcherToolbarLayout is shared by drawing and truncation detection so the shortcut uses the same geometry as the rendered message.
type launcherToolbarLayout struct {
	contentHeight, fontSize, contentWidth, leftWidth, rightWidth, labelWidth, naturalLabelWidth float32
	statusHotkey                                                                                woxwidget.Widget
	shown                                                                                       []measuredLauncherToolbarAction
}

// LauncherToolbarStatusTruncated reports whether the message will be ellipsized at this width.
func LauncherToolbarStatusTruncated(props LauncherToolbarProps) bool {
	if props.Label == "" || props.MenuVisible {
		return false
	}
	layout := launcherToolbarLayoutFor(props)
	return strings.ContainsAny(props.Label, "\r\n") || layout.naturalLabelWidth > layout.labelWidth
}

// launcherToolbarLayoutFor measures the footer once using logical launcher dimensions.
func launcherToolbarLayoutFor(props LauncherToolbarProps) launcherToolbarLayout {
	contentHeight := launcherToolbarContentHeight(props.DensityScale)
	fontSize := scaledLauncherSize(woxcomponent.ToolbarFontSize, props.DensityScale)
	actionGap := float32(0)
	statusActionGap := scaledLauncherSize(16, props.DensityScale)
	contentWidth := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
	progressVisible := props.HasProgress || props.Indeterminate
	measured := make([]measuredLauncherToolbarAction, 0, len(props.Actions))
	for _, action := range props.Actions {
		widget, width := launcherToolbarActionView(action, props.Theme, props.Window, props.DensityScale)
		measured = append(measured, measuredLauncherToolbarAction{id: action.ID, widget: widget, width: width, pinned: action.Pinned})
	}
	naturalLeft, naturalLabelWidth := float32(0), float32(0)
	if props.MenuVisible {
		naturalLeft = contentHeight
	} else if props.Label != "" {
		metrics, _ := props.Window.MeasureText(props.Label, woxui.TextStyle{Size: fontSize})
		naturalLabelWidth = metrics.Size.Width
		naturalLeft = naturalLabelWidth
		if props.Icon != nil {
			naturalLeft += scaledLauncherSize(26, props.DensityScale)
		}
		if progressVisible {
			naturalLeft += scaledLauncherSize(22, props.DensityScale)
		}
	}
	var statusHotkey woxwidget.Widget
	statusHotkeyWidth := float32(0)
	if props.StatusInteractive && !props.MenuVisible {
		statusHotkey, statusHotkeyWidth = woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{Theme: &props.Theme, Toolbar: true, Labels: props.StatusHotkeyLabels, Foreground: props.Theme.ToolbarText, Background: props.Theme.ToolbarBackground, FontSize: scaledLauncherSize(woxcomponent.TailFontSize, props.DensityScale), Compact: props.DensityScale < 1, Window: props.Window})
		statusHotkeyWidth += scaledLauncherSize(24, props.DensityScale)
		naturalLeft += statusHotkeyWidth
	}
	leftWidth, rightAvailable := launcherToolbarSplit(contentWidth, naturalLeft, toolbarReservedActionsWidth(measured, actionGap), statusActionGap)
	minimumStatusWidth := statusHotkeyWidth
	if props.Icon != nil {
		minimumStatusWidth += scaledLauncherSize(26, props.DensityScale)
	}
	if progressVisible {
		minimumStatusWidth += scaledLauncherSize(22, props.DensityScale)
	}
	if props.StatusInteractive && !props.MenuVisible && leftWidth < min(contentWidth, minimumStatusWidth) {
		leftWidth = min(contentWidth, minimumStatusWidth)
		rightAvailable = max(float32(0), contentWidth-leftWidth-statusActionGap)
	}
	shown, rightWidth := fitLauncherToolbarActions(measured, actionGap, rightAvailable)
	extraWidth := float32(0)
	if props.Icon != nil {
		extraWidth += scaledLauncherSize(26, props.DensityScale)
	}
	if progressVisible {
		extraWidth += scaledLauncherSize(22, props.DensityScale)
	}
	labelWidth := max(float32(0), leftWidth-extraWidth-statusHotkeyWidth)
	return launcherToolbarLayout{contentHeight, fontSize, contentWidth, leftWidth, rightWidth, labelWidth, naturalLabelWidth, statusHotkey, shown}
}

// launcherToolbarAboutMenuButton is the compact left-side About Menu control.
func launcherToolbarAboutMenuButton(props LauncherToolbarProps, contentHeight, width float32) woxwidget.Widget {
	iconSize := scaledLauncherSize(18, props.DensityScale)
	radius := scaledLauncherSize(4, props.DensityScale)
	var icon woxwidget.Widget = woxwidget.Painter{Width: iconSize, Height: iconSize}
	if props.MenuIcon != nil {
		icon = woxwidget.Image{Source: props.MenuIcon, Width: iconSize, Height: iconSize}
	}
	return woxwidget.Semantics{
		Key: "launcher-toolbar-about-menu-semantics", AutomationID: "launcher.toolbar.about-menu",
		Role: woxui.AccessibilityRoleButton, Label: props.MenuLabel, Expanded: props.MenuActive,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(semanticAction woxui.AccessibilityAction, _ string) error {
			if semanticAction != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported about menu action %q", semanticAction)
			}
			if props.OnMenuTap != nil {
				props.OnMenuTap()
			}
			return nil
		},
		Child: woxcomponent.Hoverable("launcher-toolbar-about-menu-hover", false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
			background := woxui.Color{}
			if hovered || props.MenuActive {
				background = woxcomponent.ControlHoverColor(props.Theme.ToolbarBackground, props.Theme.ToolbarText)
			}
			return woxwidget.Gesture{
				ID: "launcher-toolbar-about-menu", OnTap: props.OnMenuTap,
				OnHoverAt: func(inside bool, bounds woxui.Rect) {
					onHoverAt(inside, bounds)
					if props.OnMenuHoverAt != nil {
						props.OnMenuHoverAt(inside, bounds)
					}
				},
				Child: woxwidget.Container{
					Width: width, Height: contentHeight, Radius: radius, Color: background,
					Child: woxwidget.Align{Width: width, Height: contentHeight, Horizontal: 0.5, Vertical: 0.5, Child: icon},
				},
			}
		}),
	}
}

// launcherToolbarActionView builds one label-and-keycap unit and reports its width.
func launcherToolbarActionView(action LauncherToolbarAction, theme woxcomponent.Theme, window *woxui.Window, densityScale float32) (woxwidget.Widget, float32) {
	_, width := launcherToolbarActionSurface(action, theme, window, densityScale, false)
	return woxwidget.Semantics{
		Key: woxwidget.Key(action.ID + "-semantics"), AutomationID: action.ID, Role: woxui.AccessibilityRoleButton, Label: action.Label,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate}, OnAction: func(semanticAction woxui.AccessibilityAction, _ string) error {
			if semanticAction != woxui.AccessibilityActionActivate {
				return fmt.Errorf("unsupported toolbar action %q", semanticAction)
			}
			if action.OnTap != nil {
				action.OnTap()
			}
			return nil
		},
		// Label and keycaps share one tap target, so hover covers the whole action instead of
		// treating the shortcut chips as a second control.
		Child: woxcomponent.Hoverable(woxwidget.Key(action.ID+"-hover"), false, func(hovered bool, onHoverAt func(bool, woxui.Rect)) woxwidget.Widget {
			content, _ := launcherToolbarActionSurface(action, theme, window, densityScale, hovered)
			return woxwidget.Gesture{ID: action.ID, OnTap: action.OnTap, OnHoverAt: onHoverAt, Child: content}
		}),
	}, width
}

// launcherToolbarActionSurface paints one toolbar action, including the shared hover overlay.
func launcherToolbarActionSurface(action LauncherToolbarAction, theme woxcomponent.Theme, window *woxui.Window, densityScale float32, hovered bool) (woxwidget.Widget, float32) {
	labelStyle := woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.ToolbarFontSize, densityScale)}
	labelColor := theme.ToolbarText
	// Pinned actions are the default/bare-Enter actions reserved by the launcher adapter.
	if action.Pinned && theme.ToolbarPrimaryFontColor != nil {
		labelColor = *theme.ToolbarPrimaryFontColor
	}
	labelMetrics, _ := window.MeasureText(action.Label, labelStyle)
	background := woxui.Color{}
	chipBackground := theme.ToolbarBackground
	// An open popover keeps its trigger highlighted even after the pointer leaves it.
	if hovered || action.Active {
		background = woxcomponent.ControlHoverColor(theme.ToolbarBackground, theme.ToolbarText)
		chipBackground = background
	}
	chip, chipWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
		Theme: &theme, Toolbar: true, Primary: action.Pinned,
		Labels: action.HotkeyLabels, Foreground: theme.ToolbarText, Background: chipBackground,
		FontSize: scaledLauncherSize(woxcomponent.TailFontSize, densityScale), Compact: densityScale < 1, Window: window,
	})
	gap := scaledLauncherSize(8, densityScale)
	horizontalPadding := scaledLauncherSize(8, densityScale)
	verticalPadding := scaledLauncherSize(2, densityScale)
	contentHeight := launcherToolbarContentHeight(densityScale)
	children := make([]woxwidget.Widget, 0, 2)
	width := horizontalPadding * 2
	if strings.TrimSpace(action.Label) != "" {
		children = append(children, woxwidget.Text{Value: action.Label, Style: labelStyle, Color: labelColor})
		width += labelMetrics.Size.Width
	}
	// A zero-width hotkey widget fills the remaining space, so omit it entirely.
	if len(action.HotkeyLabels) > 0 {
		if len(children) > 0 {
			width += gap
		}
		children = append(children, chip)
		width += chipWidth
	}
	return woxwidget.Container{
		Width: width, Height: contentHeight, Radius: scaledLauncherSize(4, densityScale), Color: background,
		Padding: woxwidget.Insets{Left: horizontalPadding, Top: verticalPadding, Right: horizontalPadding, Bottom: verticalPadding},
		Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children},
	}, width
}

// launcherToolbarContentHeight includes a 2px optical inset above and below the 28px action content.
func launcherToolbarContentHeight(densityScale float32) float32 {
	return scaledLauncherSize(28, densityScale) + scaledLauncherSize(2, densityScale)*2
}

// launcherToolbarSplit gives the status message its natural width first, then
// leaves the leftover for shortcuts. Enter and More stay reserved so a long
// message ellipsizes instead of pushing those two off the footer.
func launcherToolbarSplit(contentWidth, naturalLeft, reservedRight, statusGap float32) (leftWidth, rightAvailable float32) {
	if naturalLeft > 0 && reservedRight > 0 {
		leftWidth = min(naturalLeft, max(float32(0), contentWidth-reservedRight-statusGap))
		return leftWidth, max(float32(0), contentWidth-leftWidth-statusGap)
	}
	if reservedRight > 0 {
		return 0, contentWidth
	}
	return min(naturalLeft, contentWidth), max(float32(0), contentWidth-min(naturalLeft, contentWidth))
}

func toolbarReservedActionsWidth(measured []measuredLauncherToolbarAction, gap float32) float32 {
	return toolbarActionsWidth(toolbarReservedActions(measured), gap)
}

// toolbarReservedActions returns default Enter and More, which always keep a footer slot.
func toolbarReservedActions(measured []measuredLauncherToolbarAction) []measuredLauncherToolbarAction {
	reserved := make([]measuredLauncherToolbarAction, 0, 2)
	for _, action := range measured {
		if launcherToolbarActionReserved(action) {
			reserved = append(reserved, action)
		}
	}
	return reserved
}

func toolbarActionsWidth(measured []measuredLauncherToolbarAction, gap float32) float32 {
	total := float32(0)
	for index, action := range measured {
		total += action.width
		if index > 0 {
			total += gap
		}
	}
	return total
}

func launcherToolbarActionReserved(action measuredLauncherToolbarAction) bool {
	return action.pinned || action.id == launcherToolbarMoreActionID
}

// fitLauncherToolbarActions keeps Enter and More, then adds the remaining
// hotkey actions in source order until the leftover width is gone.
func fitLauncherToolbarActions(measured []measuredLauncherToolbarAction, gap, available float32) ([]measuredLauncherToolbarAction, float32) {
	if len(measured) == 0 || available <= 0 {
		return nil, 0
	}
	var more *measuredLauncherToolbarAction
	shown := make([]measuredLauncherToolbarAction, 0, len(measured))
	used := float32(0)
	appendIfFits := func(action measuredLauncherToolbarAction) bool {
		nextWidth := action.width
		if len(shown) > 0 {
			nextWidth += gap
		}
		if used+nextWidth > available {
			return false
		}
		shown = append(shown, action)
		used += nextWidth
		return true
	}
	for index := range measured {
		action := measured[index]
		if action.id == launcherToolbarMoreActionID {
			copy := action
			more = &copy
			continue
		}
		if !action.pinned {
			continue
		}
		if !appendIfFits(action) {
			return shown, used
		}
	}
	moreWidth := float32(0)
	if more != nil {
		moreWidth = more.width
		if len(shown) > 0 {
			moreWidth += gap
		}
	}
	for _, action := range measured {
		if launcherToolbarActionReserved(action) {
			continue
		}
		nextWidth := action.width
		if len(shown) > 0 {
			nextWidth += gap
		}
		if used+nextWidth+moreWidth > available {
			break
		}
		shown = append(shown, action)
		used += nextWidth
	}
	if more != nil {
		appendIfFits(*more)
	}
	return shown, used
}
