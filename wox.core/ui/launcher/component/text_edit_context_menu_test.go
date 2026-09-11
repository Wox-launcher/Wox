package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestTextEditContextMenuIsFloatingThemeSurface(t *testing.T) {
	theme := Theme{
		QueryBackground:  woxui.Color{R: 10, G: 10, B: 10, A: 40},
		ActionBackground: woxui.Color{R: 40, G: 44, B: 52, A: 120},
		ActionText:       woxui.Color{R: 240, G: 240, B: 240, A: 255},
		ResultSubtitle:   woxui.Color{R: 160, G: 160, B: 160, A: 255},
	}
	menu := BuildTextEditContextMenu(TextEditContextMenuProps{ID: "menu", CanPaste: true, Theme: theme}).(woxwidget.Container)
	if !menu.Floating || menu.Color != theme.ActionBackground {
		t.Fatalf("menu surface = floating %v color %#v, want a floating surface tinted with ActionBackground as the theme wrote it", menu.Floating, menu.Color)
	}
	if menu.BorderWidth != 1 || menu.BorderColor.A == 0 {
		t.Fatalf("menu edge = %#v width %v, want a visible hairline", menu.BorderColor, menu.BorderWidth)
	}

	// Themes without an action surface fall back to an opaque QueryBackground, which is often translucent.
	theme.ActionBackground = woxui.Color{}
	fallback := BuildTextEditContextMenu(TextEditContextMenuProps{ID: "menu", CanPaste: true, Theme: theme}).(woxwidget.Container)
	if fallback.Color != (woxui.Color{R: 10, G: 10, B: 10, A: 255}) {
		t.Fatalf("fallback menu background = %#v, want opaque QueryBackground", fallback.Color)
	}
}

func TestTextEditContextMenuExposesStableMenuItemSemantics(t *testing.T) {
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return BuildTextEditContextMenu(TextEditContextMenuProps{
			ID: "field.menu", CanSelectAll: true, CanPaste: true,
			Theme: Theme{ActionBackground: woxui.Color{A: 255}, ActionText: woxui.Color{A: 255}},
		})
	})
	host.AttachServices(&hotkeyRecorderHostServices{})
	displayList := &woxui.DisplayList{}
	host.Frame(displayList, woxui.FrameInfo{Size: woxui.Size{Width: 200, Height: 160}, PixelSize: woxui.PixelSize{Width: 200, Height: 160}, Scale: 1})
	// Cut/Copy stay disabled without a selection; Paste/Select All are enabled by props.
	wantEnabled := map[string]bool{
		TextEditContextMenuItemID("field.menu", TextEditContextCut):       false,
		TextEditContextMenuItemID("field.menu", TextEditContextCopy):      false,
		TextEditContextMenuItemID("field.menu", TextEditContextPaste):     true,
		TextEditContextMenuItemID("field.menu", TextEditContextSelectAll): true,
	}
	found := map[string]bool{}
	for _, node := range host.Snapshot().Tree.Nodes {
		enabled, ok := wantEnabled[node.AutomationID]
		if !ok {
			continue
		}
		found[node.AutomationID] = true
		if node.Role != woxui.AccessibilityRoleMenuItem {
			t.Fatalf("%s role = %q", node.AutomationID, node.Role)
		}
		if node.Enabled != enabled {
			t.Fatalf("%s enabled = %v, want %v", node.AutomationID, node.Enabled, enabled)
		}
		hasActivate := containsAccessibilityAction(node.Actions, woxui.AccessibilityActionActivate)
		if enabled != hasActivate {
			t.Fatalf("%s actions = %v, enabled %v", node.AutomationID, node.Actions, enabled)
		}
	}
	for id := range wantEnabled {
		if !found[id] {
			t.Fatalf("missing menu item semantics %q", id)
		}
	}
}
