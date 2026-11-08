//go:build wox_ui_smoke

package about

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	"wox/ui/widget"
)

// Test001SettingAboutLicenses checks that the shipped notices can be read from Settings.
func Test001SettingAboutLicenses(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		if err := client.OpenSettings(ctx, "/about"); err != nil {
			t.Fatal(err)
		}
		wait := func(id string, check func(woxui.AccessibilityNode) bool) woxui.AccessibilityNode {
			t.Helper()
			var node woxui.AccessibilityNode
			if _, err := client.WaitFor(ctx, func(snapshot widget.AutomationSnapshot) bool {
				var found bool
				node, found = automationdriver.Find(snapshot, id)
				return found && (check == nil || check(node))
			}); err != nil {
				t.Fatalf("wait for %s: %v", id, err)
			}
			return node
		}
		activate := func(id string) {
			t.Helper()
			wait(id, nil)
			if err := client.Perform(ctx, id, woxui.AccessibilityActionActivate, ""); err != nil {
				t.Fatalf("activate %s: %v", id, err)
			}
		}
		activate("about-open-licenses")
		first := wait("about-license-0", nil)
		var ghostty woxui.AccessibilityNode
		if _, err := client.WaitFor(ctx, func(snapshot widget.AutomationSnapshot) bool {
			for _, node := range snapshot.Tree.Nodes {
				if node.Label == "libghostty-vt" {
					ghostty = node
					return true
				}
			}
			return false
		}); err != nil {
			t.Fatalf("find libghostty-vt in the bundled licenses: %v", err)
		}
		if err := client.Pointer(ctx, woxui.PointerEvent{
			Kind: woxui.PointerScroll, Position: woxui.Point{X: first.Bounds.X + 10, Y: first.Bounds.Y + 10},
			Scroll: woxui.Point{Y: -3000},
		}); err != nil {
			t.Fatalf("scroll license list: %v", err)
		}
		wait(ghostty.AutomationID, func(node woxui.AccessibilityNode) bool { return node.Bounds.Y < ghostty.Bounds.Y })
		activate(ghostty.AutomationID)
		wait("about-license-text", func(node woxui.AccessibilityNode) bool {
			return node.ReadOnly && strings.Contains(node.Value, "Copyright") && strings.Contains(node.Value, "wuffs")
		})
		activate("about-licenses-back")
		wait("about-license-0", nil)
		activate("about-licenses-back")
		wait("about-open-licenses", nil)
	})
}
