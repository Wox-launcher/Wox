package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestTextEditContextMenuUsesOpaqueThemeSurface(t *testing.T) {
	menu := BuildTextEditContextMenu(TextEditContextMenuProps{
		ID: "menu", CanPaste: true,
		Theme: Theme{
			QueryBackground:  woxui.Color{R: 10, G: 10, B: 10, A: 40},
			ActionBackground: woxui.Color{R: 40, G: 44, B: 52, A: 120},
			ActionText:       woxui.Color{R: 240, G: 240, B: 240, A: 255},
			ResultSubtitle:   woxui.Color{R: 160, G: 160, B: 160, A: 255},
		},
	}).(woxwidget.Container)
	if menu.Color.A != 255 {
		t.Fatalf("menu background alpha = %d, want opaque 255", menu.Color.A)
	}
	if menu.Color.R != 40 || menu.Color.G != 44 || menu.Color.B != 52 {
		t.Fatalf("menu background = %#v, want ActionBackground RGB", menu.Color)
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
