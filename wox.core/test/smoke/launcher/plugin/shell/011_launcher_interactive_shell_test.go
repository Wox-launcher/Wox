//go:build wox_ui_smoke

package shell

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test011LauncherInteractiveShell drives the preview through the same focus and input path as a user.
func Test011LauncherInteractiveShell(t *testing.T) {
	const screenID = "shell-interactive-screen"
	const openID = "shell-interactive-open"
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		setQuery := func(query string) {
			t.Helper()
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("set query %q: %v", query, err)
			}
		}
		waitForNode := func(id string, predicate func(woxui.AccessibilityNode) bool) {
			t.Helper()
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				node, found := automationdriver.Find(snapshot, id)
				return found && predicate(node)
			}); err != nil {
				t.Fatalf("wait for %s: %v", id, err)
			}
		}
		key := func(key woxui.Key, modifiers woxui.KeyModifiers) {
			t.Helper()
			if err := client.PressKey(ctx, key, modifiers); err != nil {
				t.Fatalf("press %s: %v", key, err)
			}
		}
		setQuery("> ")
		waitForNode(openID, func(woxui.AccessibilityNode) bool { return true })
		key(woxui.KeyEnter, 0)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused && node.Value != "" })

		primary := woxui.KeyModifierControl
		if runtime.GOOS == "darwin" {
			primary = woxui.KeyModifierMeta
		}
		key("k", primary)
		waitForNode("action-search", func(node woxui.AccessibilityNode) bool { return node.Focused })
		key("k", primary)
		waitForNode("launcher.query.input", func(node woxui.AccessibilityNode) bool { return node.Focused })
		key(woxui.KeyEnter, 0)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused })

		command := func(unix, windows string) {
			t.Helper()
			value := unix
			if runtime.GOOS == "windows" {
				value = windows
			}
			if err := client.EnterText(ctx, value); err != nil {
				t.Fatalf("type into terminal: %v", err)
			}
			key(woxui.KeyEnter, 0)
		}
		waitForOutput := func(value string) {
			t.Helper()
			waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return strings.Contains(node.Value, value) })
		}
		command("printf '%s%s\\n' wox-interactive- sleeping; sleep 30", "Write-Output ('wox-interactive-' + 'sleeping'); Start-Sleep -Seconds 30")
		waitForOutput("wox-interactive-sleeping")
		key("c", woxui.KeyModifierControl)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			lines := strings.Split(strings.TrimSpace(node.Value), "\n")
			last := lines[len(lines)-1]
			return last != "" && strings.ContainsRune(">$#%", rune(last[len(last)-1]))
		})
		command("printf '%s%s\\n' wox-interactive- ready", "Write-Output ('wox-interactive-' + 'ready')")
		waitForOutput("wox-interactive-ready")
		key(woxui.KeyEscape, woxui.KeyModifierShift)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused })
		key(woxui.KeyEscape, 0)
		waitForNode("launcher.query.input", func(node woxui.AccessibilityNode) bool { return node.Focused && node.Value == "> " })
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			return !node.Focused && strings.Contains(node.Value, "wox-interactive-ready")
		})

		setQuery("> echo one-shot")
		waitForShellResult(t, ctx, client, "echo one-shot")
		setQuery("> ")
		waitForNode(openID, func(woxui.AccessibilityNode) bool { return true })
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, found := automationdriver.Find(snapshot, screenID); found {
			t.Fatal("returning to the result reopened the previous session")
		}
	})
}
