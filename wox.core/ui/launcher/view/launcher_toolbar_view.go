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
	OnTap        func() `boundary:"stable"`
}

// Equal compares every visual dependency for one prepared toolbar action.
func (a LauncherToolbarAction) Equal(other LauncherToolbarAction) bool {
	if a.ID != other.ID || a.Label != other.Label || len(a.HotkeyLabels) != len(other.HotkeyLabels) {
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
	Width         float32
	Height        float32
	Padding       woxwidget.Insets
	Theme         woxcomponent.Theme
	Window        *woxui.Window
	DensityScale  float32
	Label         string
	Icon          *woxui.Image
	Progress      int
	HasProgress   bool
	Indeterminate bool
	Actions       []LauncherToolbarAction
	OnDragStart   func() `boundary:"stable"`
}

// Equal compares every render dependency for the launcher toolbar section.
func (p LauncherToolbarProps) Equal(other LauncherToolbarProps) bool {
	if p.Width != other.Width || p.Height != other.Height || p.Padding != other.Padding || p.Theme != other.Theme || p.Window != other.Window || p.DensityScale != other.DensityScale || p.Label != other.Label || p.Icon != other.Icon || p.Progress != other.Progress || p.HasProgress != other.HasProgress || p.Indeterminate != other.Indeterminate || len(p.Actions) != len(other.Actions) {
		return false
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
}

// LauncherToolbarView builds the status footer and the actions that fit its current width.
func LauncherToolbarView(props LauncherToolbarProps) woxwidget.Widget {
	contentHeight := launcherToolbarContentHeight(props.DensityScale)
	fontSize := scaledLauncherSize(woxcomponent.ToolbarFontSize, props.DensityScale)
	// Hover padding is 8px on each side, so a 0 Flex gap keeps 16px between
	// action contents. Stacking the previous 16px gap on that padding doubled
	// the visible spacing.
	actionGap := float32(0)
	statusActionGap := scaledLauncherSize(16, props.DensityScale)
	contentWidth := max(float32(0), props.Width-props.Padding.Left-props.Padding.Right)
	progressVisible := props.HasProgress || props.Indeterminate
	measured := make([]measuredLauncherToolbarAction, 0, len(props.Actions))
	for _, action := range props.Actions {
		widget, width := launcherToolbarActionView(action, props.Theme, props.Window, props.DensityScale)
		measured = append(measured, measuredLauncherToolbarAction{id: action.ID, widget: widget, width: width})
	}
	naturalLeft := float32(0)
	labelWidth := float32(0)
	if props.Label != "" {
		metrics, _ := props.Window.MeasureText(props.Label, woxui.TextStyle{Size: fontSize})
		labelWidth = metrics.Size.Width
		naturalLeft = labelWidth
		if props.Icon != nil {
			naturalLeft += scaledLauncherSize(26, props.DensityScale)
		}
		if progressVisible {
			naturalLeft += scaledLauncherSize(22, props.DensityScale)
		}
	}
	leftWidth, rightAvailable := launcherToolbarSplit(contentWidth, naturalLeft, toolbarActionsWidth(measured, actionGap), statusActionGap, scaledLauncherSize(200, props.DensityScale))
	shown, rightWidth := fitLauncherToolbarActions(measured, actionGap, rightAvailable)
	rightChildren := make([]woxwidget.Widget, 0, len(shown))
	for _, action := range shown {
		rightChildren = append(rightChildren, action.widget)
	}
	extraWidth := float32(0)
	if props.Icon != nil {
		extraWidth += scaledLauncherSize(26, props.DensityScale)
	}
	if progressVisible {
		extraWidth += scaledLauncherSize(22, props.DensityScale)
	}
	labelWidth = max(float32(0), leftWidth-extraWidth)
	leftWidgets := make([]woxwidget.Widget, 0, 3)
	if props.Icon != nil {
		iconSize := scaledLauncherSize(18, props.DensityScale)
		leftWidgets = append(leftWidgets, woxwidget.Align{
			Width: iconSize, Height: contentHeight, Vertical: 0.5, Child: woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize},
		})
	}
	if props.Label != "" {
		leftWidgets = append(leftWidgets, woxwidget.Align{
			Width: labelWidth, Height: contentHeight, Vertical: 0.5,
			Child: woxwidget.Text{Value: props.Label, Style: woxui.TextStyle{Size: fontSize}, Color: props.Theme.ToolbarText},
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
	if props.Label != "" {
		leftWidgets = []woxwidget.Widget{woxwidget.Semantics{
			Key: "launcher-toolbar-status-key", AutomationID: "launcher.toolbar.status", Role: woxui.AccessibilityRoleGroup,
			Label: props.Label, Value: props.Label, LiveRegion: woxui.AccessibilityLiveRegionPolite,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), Children: leftWidgets},
		}}
	}
	body := woxwidget.Container{
		Width: props.Width, Height: props.Height, Color: props.Theme.ToolbarBackground,
		Padding: woxwidget.Insets{Left: props.Padding.Left, Right: props.Padding.Right},
		Child: woxwidget.Align{Height: props.Height, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Container{Width: leftWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scaledLauncherSize(8, props.DensityScale), Children: leftWidgets}},
			woxwidget.Gesture{ID: "launcher-toolbar-drag-area", OnDragStart: props.OnDragStart, Child: woxwidget.Container{
				Width: max(float32(0), contentWidth-leftWidth-rightWidth), Height: contentHeight,
			}},
			woxwidget.Container{Width: rightWidth, Height: contentHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: actionGap, Children: rightChildren}},
		}}},
	}
	border := props.Theme.ToolbarText
	border.A = min(border.A, uint8(26))
	return woxwidget.Stack{Width: props.Width, Height: props.Height, Children: []woxwidget.StackChild{
		{Child: body},
		{Child: woxwidget.Painter{Width: props.Width, Height: 1, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRect(bounds, border)
		}}},
	}}
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
	labelMetrics, _ := window.MeasureText(action.Label, labelStyle)
	background := woxui.Color{}
	chipBackground := theme.ToolbarBackground
	if hovered {
		background = woxcomponent.ControlHoverColor(theme.ToolbarBackground, theme.ToolbarText)
		chipBackground = background
	}
	chip, chipWidth := woxcomponent.WoxHotkey(woxcomponent.HotkeyProps{
		Labels: action.HotkeyLabels, Foreground: theme.ToolbarText, Background: chipBackground,
		FontSize: scaledLauncherSize(woxcomponent.TailFontSize, densityScale), Compact: densityScale < 1, Window: window,
	})
	gap := scaledLauncherSize(8, densityScale)
	horizontalPadding := scaledLauncherSize(8, densityScale)
	verticalPadding := scaledLauncherSize(2, densityScale)
	contentHeight := launcherToolbarContentHeight(densityScale)
	children := []woxwidget.Widget{chip}
	width := horizontalPadding*2 + chipWidth
	// Skip empty labels. Align{Width: 0} means fill-available, so a blank
	// doctor/default action would push Enter onto the next toolbar action.
	if strings.TrimSpace(action.Label) != "" {
		children = []woxwidget.Widget{
			woxwidget.Text{Value: action.Label, Style: labelStyle, Color: theme.ToolbarText},
			chip,
		}
		width += labelMetrics.Size.Width + gap
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

// launcherToolbarSplit lets prepared actions keep their measured width before the
// status label consumes the leftover, so a long toolbar message does not hide
// result and message actions that still fit the window.
func launcherToolbarSplit(contentWidth, naturalLeft, wantedRight, statusGap, minRight float32) (leftWidth, rightAvailable float32) {
	rightNeed := wantedRight
	if rightNeed > 0 {
		rightNeed = max(rightNeed, minRight)
	}
	if naturalLeft > 0 && rightNeed > 0 {
		leftWidth = min(naturalLeft, max(float32(0), contentWidth-rightNeed-statusGap))
		return leftWidth, max(float32(0), contentWidth-leftWidth-statusGap)
	}
	if rightNeed > 0 {
		return 0, contentWidth
	}
	return min(naturalLeft, contentWidth), max(float32(0), contentWidth-min(naturalLeft, contentWidth))
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

// fitLauncherToolbarActions keeps More visible and then fills remaining width
// from the start of the prepared list so result actions are not dropped first.
func fitLauncherToolbarActions(measured []measuredLauncherToolbarAction, gap, available float32) ([]measuredLauncherToolbarAction, float32) {
	if len(measured) == 0 || available <= 0 {
		return nil, 0
	}
	moreIndex := -1
	for index, action := range measured {
		if action.id == launcherToolbarMoreActionID {
			moreIndex = index
			break
		}
	}
	reserved := float32(0)
	if moreIndex >= 0 {
		reserved = measured[moreIndex].width
		if len(measured) > 1 {
			reserved += gap
		}
	}
	shown := make([]measuredLauncherToolbarAction, 0, len(measured))
	used := float32(0)
	for index, action := range measured {
		if index == moreIndex {
			continue
		}
		nextWidth := action.width
		if len(shown) > 0 {
			nextWidth += gap
		}
		if used+nextWidth > available-reserved {
			break
		}
		shown = append(shown, action)
		used += nextWidth
	}
	if moreIndex >= 0 {
		nextWidth := measured[moreIndex].width
		if len(shown) > 0 {
			nextWidth += gap
		}
		if used+nextWidth <= available {
			shown = append(shown, measured[moreIndex])
			used += nextWidth
		}
	}
	return shown, used
}
