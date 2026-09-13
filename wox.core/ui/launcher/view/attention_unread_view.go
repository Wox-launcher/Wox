package view

import (
	"strconv"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AttentionUnreadBoundaryKey identifies the retained launcher unread-attention badge.
const AttentionUnreadBoundaryKey woxwidget.Key = "query-attention-unread-boundary"

const (
	attentionUnreadHeight   = float32(30)
	attentionUnreadIconSize = float32(16)
	attentionUnreadRadius   = float32(5)
	attentionUnreadPadding  = float32(8)
	attentionUnreadIconGap  = float32(5)
	attentionUnreadMinWidth = float32(44)
	attentionUnreadMaxWidth = float32(78)
)

// AttentionUnreadCountText caps the Flutter-era badge label at 99+.
func AttentionUnreadCountText(count int) string {
	if count > 99 {
		return "99+"
	}
	if count < 0 {
		return "0"
	}
	return strconv.Itoa(count)
}

// AttentionUnreadWidth sizes the inbox+count accessory with the same padding as Glance.
func AttentionUnreadWidth(countTextWidth, densityScale float32) float32 {
	width := scaledLauncherSize(attentionUnreadPadding*2+attentionUnreadIconSize+attentionUnreadIconGap, densityScale) + countTextWidth
	return min(scaledLauncherSize(attentionUnreadMaxWidth, densityScale), max(scaledLauncherSize(attentionUnreadMinWidth, densityScale), width))
}

// AttentionUnreadProps contains the display state and actions for unread attention items.
type AttentionUnreadProps struct {
	Width        float32
	Icon         *woxui.Image
	Tooltip      string
	CountText    string
	UnreadCount  int
	Theme        woxcomponent.Theme
	DensityScale float32
	OnTap        func()                         `boundary:"stable"`
	OnHover      func(bool, string, woxui.Rect) `boundary:"stable"`
}

// Equal compares every render dependency for the unread-attention badge.
func (p AttentionUnreadProps) Equal(other AttentionUnreadProps) bool {
	return p.Width == other.Width && p.Icon == other.Icon && p.Tooltip == other.Tooltip && p.CountText == other.CountText && p.UnreadCount == other.UnreadCount && p.Theme == other.Theme && p.DensityScale == other.DensityScale
}

type attentionUnreadViewState struct {
	hovered bool
}

// AttentionUnreadView builds the retained compact query-box unread badge.
func AttentionUnreadView(props AttentionUnreadProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: "query-attention-unread-state", Type: (*attentionUnreadViewState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &attentionUnreadViewState{} },
	}
}

// AttentionUnreadBoundary retains the badge until its prepared content changes.
func AttentionUnreadBoundary(props AttentionUnreadProps) woxwidget.Widget {
	return woxwidget.Boundary[AttentionUnreadProps]{
		Key: AttentionUnreadBoundaryKey, Label: "header:attention", Props: props,
		Build: func(props AttentionUnreadProps) woxwidget.Widget { return AttentionUnreadView(props) },
	}
}

// InitState starts the badge outside its transient hover state.
func (s *attentionUnreadViewState) InitState(_ woxwidget.StateContext, _ any) {
	s.hovered = false
}

// DidUpdateWidget preserves hover while immutable unread content is refreshed.
func (s *attentionUnreadViewState) DidUpdateWidget(_ woxwidget.StateContext, _, _ any) {}

// Build composes the inbox accessory with locally owned hover state.
func (s *attentionUnreadViewState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(AttentionUnreadProps)
	width := props.Width
	if width <= 0 {
		width = AttentionUnreadWidth(0, props.DensityScale)
	}
	height := scaledLauncherSize(attentionUnreadHeight, props.DensityScale)
	iconSize := scaledLauncherSize(attentionUnreadIconSize, props.DensityScale)
	padding := scaledLauncherSize(attentionUnreadPadding, props.DensityScale)
	gap := scaledLauncherSize(attentionUnreadIconGap, props.DensityScale)
	_, labelColor, background, border := props.Theme.AttentionBadgeColors(s.hovered)
	countText := props.CountText
	if countText == "" {
		countText = AttentionUnreadCountText(props.UnreadCount)
	}
	children := make([]woxwidget.Widget, 0, 2)
	if props.Icon != nil {
		children = append(children, woxwidget.Image{Source: props.Icon, Width: iconSize, Height: iconSize})
	}
	children = append(children, woxwidget.Text{
		Value: countText, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.AttentionBadgeFontSize, props.DensityScale)}, Color: labelColor,
	})
	borderWidth := float32(0)
	if border.A > 0 {
		borderWidth = scaledLauncherSize(1, props.DensityScale)
	}
	contentWidth := max(float32(0), width-padding*2)
	return woxwidget.Semantics{
		Key: "launcher-query-attention-key", AutomationID: "launcher.query.attention", Role: woxui.AccessibilityRoleButton,
		Label: props.Tooltip, Value: countText, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate && props.OnTap != nil {
				props.OnTap()
			}
			return nil
		},
		Child: woxwidget.Gesture{ID: "query-attention-unread", OnTap: props.OnTap, OnHoverAt: func(inside bool, bounds woxui.Rect) {
			if inside != s.hovered {
				context.SetState(func() { s.hovered = inside })
			}
			if props.OnHover != nil {
				props.OnHover(inside, props.Tooltip, bounds)
			}
		}, Child: woxwidget.Container{
			Width: width, Height: height, Radius: scaledLauncherSize(attentionUnreadRadius, props.DensityScale),
			Color: background, BorderColor: border, BorderWidth: borderWidth, Padding: woxwidget.Insets{Left: padding, Right: padding},
			Child: woxwidget.Align{Width: contentWidth, Height: height, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{
				Axis: woxwidget.Horizontal, Gap: gap, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
			}},
		}},
	}
}

// Dispose releases no resources because tooltip ownership remains with the launcher.
func (s *attentionUnreadViewState) Dispose() {}
