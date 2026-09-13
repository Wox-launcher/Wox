package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestAttentionUnreadCountTextCapsAtNinetyNine(t *testing.T) {
	if got := AttentionUnreadCountText(1); got != "1" {
		t.Fatalf("count text = %q, want 1", got)
	}
	if got := AttentionUnreadCountText(100); got != "99+" {
		t.Fatalf("count text = %q, want 99+", got)
	}
}

func TestAttentionUnreadWidthClampsGlanceAccessoryRange(t *testing.T) {
	if got := AttentionUnreadWidth(8, 1); got != 45 {
		t.Fatalf("single-digit badge width = %.0f, want 45", got)
	}
	if got := AttentionUnreadWidth(0, 1); got != 44 {
		t.Fatalf("empty badge width = %.0f, want 44", got)
	}
	if got := AttentionUnreadWidth(80, 1); got != 78 {
		t.Fatalf("wide badge width = %.0f, want 78", got)
	}
}

func TestAttentionUnreadViewRendersInboxCountBadge(t *testing.T) {
	icon := &woxui.Image{}
	theme := woxcomponent.Theme{QueryText: woxui.Color{R: 240, A: 255}, Cursor: woxui.Color{R: 80, G: 160, B: 255, A: 255}}
	state := &attentionUnreadViewState{}
	built := state.Build(woxwidget.StateContext{}, AttentionUnreadProps{
		Width: 50, Icon: icon, Tooltip: "Attention items (Ctrl+U)", CountText: "2", UnreadCount: 2, Theme: theme, DensityScale: 1,
	}).(woxwidget.Semantics)
	if built.AutomationID != "launcher.query.attention" || built.Role != woxui.AccessibilityRoleButton || built.Label != "Attention items (Ctrl+U)" || built.Value != "2" {
		t.Fatalf("attention unread semantics = %#v", built)
	}
	if len(built.Actions) != 1 || built.Actions[0] != woxui.AccessibilityActionActivate || built.OnAction == nil {
		t.Fatalf("attention unread actions = %#v, want activate", built)
	}
	slot := built.Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	_, wantLabel, wantBackground, wantBorder := theme.AttentionBadgeColors(false)
	if slot.Width != 50 || slot.Height != 30 || slot.Radius != 5 || slot.BorderWidth != 0 || slot.Color != wantBackground || slot.BorderColor != wantBorder {
		t.Fatalf("attention unread badge = %#v", slot)
	}
	row := slot.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if row.Gap != 5 || len(row.Children) != 2 {
		t.Fatalf("attention unread row = %#v, want icon plus count", row)
	}
	image, ok := row.Children[0].(woxwidget.Image)
	if !ok || image.Source != icon || image.Width != 16 || image.Height != 16 {
		t.Fatalf("attention unread icon = %#v, want 16px inbox", row.Children[0])
	}
	label, ok := row.Children[1].(woxwidget.Text)
	if !ok || label.Value != "2" || label.Style.Size != woxcomponent.AttentionBadgeFontSize || label.Style.Weight != woxui.FontWeightRegular || label.Color != wantLabel {
		t.Fatalf("attention unread count = %#v", row.Children[1])
	}
}

func TestAttentionUnreadHoverUsesGlanceChrome(t *testing.T) {
	theme := woxcomponent.Theme{QueryText: woxui.Color{R: 240, A: 255}, Cursor: woxui.Color{R: 80, G: 160, B: 255, A: 255}}
	idle := (&attentionUnreadViewState{}).Build(woxwidget.StateContext{}, AttentionUnreadProps{
		Width: 50, Theme: theme, DensityScale: 1, UnreadCount: 1,
	}).(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	hovered := (&attentionUnreadViewState{hovered: true}).Build(woxwidget.StateContext{}, AttentionUnreadProps{
		Width: 50, Theme: theme, DensityScale: 1, UnreadCount: 1,
	}).(woxwidget.Semantics).Child.(woxwidget.Gesture).Child.(woxwidget.Container)
	_, _, wantIdle, wantIdleBorder := theme.AttentionBadgeColors(false)
	_, _, wantHover, wantHoverBorder := theme.AttentionBadgeColors(true)
	if idle.Color != wantIdle || idle.BorderColor != wantIdleBorder || hovered.Color != wantHover || hovered.BorderColor != wantHoverBorder {
		t.Fatalf("attention unread colors = idle %#v/%#v hover %#v/%#v", idle.Color, idle.BorderColor, hovered.Color, hovered.BorderColor)
	}
}

func TestAttentionUnreadBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, AttentionUnreadProps{})
}
