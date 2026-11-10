//go:build wox_ui_smoke

package shell

import (
	"context"
	"fmt"
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
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		setQuery := func(query string) {
			t.Helper()
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatalf("set query %q: %v", query, err)
			}
		}
		waitForNode := func(id string, predicate func(woxui.AccessibilityNode) bool) woxui.AccessibilityNode {
			t.Helper()
			var node woxui.AccessibilityNode
			if _, err := client.WaitForReason(ctx, func(snapshot woxwidget.AutomationSnapshot) (bool, string) {
				var found bool
				node, found = automationdriver.Find(snapshot, id)
				return found && predicate(node), fmt.Sprintf("label=%q description=%q focused=%t", node.Label, node.Description, node.Focused)
			}); err != nil {
				t.Fatalf("wait for %s: %v", id, err)
			}
			return node
		}
		key := func(key woxui.Key, modifiers woxui.KeyModifiers) {
			t.Helper()
			if err := client.PressKey(ctx, key, modifiers); err != nil {
				t.Fatalf("press %s: %v", key, err)
			}
		}
		setQuery("> ")
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return !node.Focused && node.Value != "" })
		waitForNode("launcher.query.input", func(node woxui.AccessibilityNode) bool { return node.Focused })
		key(woxui.KeyEnter, 0)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused && node.Value != "" })

		command := func(unix, windows string) string {
			t.Helper()
			value := unix
			if runtime.GOOS == "windows" {
				value = windows
			}
			if err := client.EnterText(ctx, value); err != nil {
				t.Fatalf("type into terminal: %v", err)
			}
			key(woxui.KeyEnter, 0)
			return value
		}
		waitForOutput := func(value string) {
			t.Helper()
			waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return strings.Contains(node.Value, value) })
		}
		slow := command("printf '%s%s\\n' wox-interactive- sleeping; sleep 30", "Write-Output ('wox-interactive-' + 'sleeping'); Start-Sleep -Seconds 30")
		waitForOutput("wox-interactive-sleeping")
		sessionRow := waitForShellResult(t, ctx, client, slow)
		running := waitForNode(sessionRow, func(node woxui.AccessibilityNode) bool { return node.Description != "" })
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused })
		key("c", woxui.KeyModifierControl)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			lines := strings.Split(strings.TrimSpace(node.Value), "\n")
			last := lines[len(lines)-1]
			return last != "" && strings.ContainsRune(">$#%", rune(last[len(last)-1]))
		})
		fast := command("wox_session_test=wox-live-''value; printf '%s%s\\n' wox-interactive- ready-历史",
			"$woxSessionTest = 'wox-live-' + 'value'; Write-Output ('wox-interactive-' + 'ready-历史')")
		waitForOutput("wox-interactive-ready-历史")
		waitForNode(sessionRow, func(node woxui.AccessibilityNode) bool {
			return node.Description != "" && node.Description != running.Description
		})
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read session history after two commands: %v", err)
		}
		if _, found := shellResult(snapshot, fast); found {
			t.Fatal("the second command created a separate live history row")
		}
		key(woxui.KeyEscape, woxui.KeyModifierShift)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused })
		key(woxui.KeyEscape, 0)
		waitForNode("launcher.query.input", func(node woxui.AccessibilityNode) bool { return node.Focused && node.Value == "> " })
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			return !node.Focused && strings.Contains(node.Value, "wox-interactive-ready")
		})
		selectShellResult(t, ctx, client, sessionRow)
		key(woxui.KeyEnter, 0)
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool { return node.Focused })
		command("printf '%s\\n' \"$wox_session_test\"", "Write-Output $woxSessionTest")
		waitForOutput("wox-live-value")
		key(woxui.KeyEscape, 0)
		waitForNode("launcher.query.input", func(node woxui.AccessibilityNode) bool { return node.Focused })

		if err := client.EnterText(ctx, "echo one-shot"); err != nil {
			t.Fatalf("type into query: %v", err)
		}
		waitForShellResult(t, ctx, client, "echo one-shot")
		key(woxui.KeyEnter, 0)
		waitForTerminalStatus(t, ctx, client, "completed")
		key(woxui.KeyEscape, 0)
		waitForOutput("wox-interactive-ready")

		setQuery("> echo second")
		waitForShellResult(t, ctx, client, "echo second")
		key(woxui.KeyEnter, 0)
		waitForTerminalStatus(t, ctx, client, "completed")
		key(woxui.KeyEscape, 0)
		setQuery("> ")
		second := waitForShellResult(t, ctx, client, "echo second")
		first := waitForShellResult(t, ctx, client, "echo one-shot")
		selectShellResult(t, ctx, client, second)
		key(woxui.KeyArrowDown, 0)
		waitForNode(first, func(node woxui.AccessibilityNode) bool { return node.Selected })
		waitForOutput("wox-interactive-ready")
		key(woxui.KeyEnter, 0)
		waitForNode("launcher.preview.terminal.output", func(node woxui.AccessibilityNode) bool {
			return node.Label == "echo one-shot" && strings.Contains(node.Value, "one-shot")
		})
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read Shell history preview: %v", err)
		}
		if _, found := automationdriver.Find(snapshot, "shell-return-to-terminal"); found {
			t.Fatal("history still displays the separate terminal preview controls")
		}
		key(woxui.KeyArrowUp, 0)
		waitForNode(second, func(node woxui.AccessibilityNode) bool { return node.Selected })
		waitForNode("launcher.preview.terminal.output", func(node woxui.AccessibilityNode) bool {
			return strings.Contains(node.Value, "one-shot")
		})
		key(woxui.KeyEscape, 0)
		waitForOutput("wox-interactive-ready")
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, ">")
		if _, found := automationdriver.Find(snapshot, screenID); found {
			t.Fatal("bare > kept the Shell plugin preview after the query completed")
		}
		if _, found := shellResult(snapshot, "echo one-shot"); found {
			t.Fatal("bare > kept Shell history after the query completed")
		}
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> ")
		saved := waitForShellResult(t, ctx, client, slow)
		if saved != sessionRow {
			t.Fatal("the persisted session did not keep the live row's identity")
		}
		if _, found := shellResult(snapshot, fast); found {
			t.Fatal("the second command created a separate persisted history row")
		}
		selectShellResult(t, ctx, client, saved)
		key(woxui.KeyEnter, 0)
		screen := waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			return node.Focused && strings.Contains(node.Value, "wox-interactive-sleeping") &&
				strings.Contains(node.Value, "wox-interactive-ready-历史") && strings.Contains(node.Value, "wox-live-value")
		})
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read restored shell: %v", err)
		}
		if _, found := automationdriver.Find(snapshot, "launcher.preview.terminal.output"); found {
			t.Fatal("interactive history still has a separate output panel")
		}
		reopened := command("for ((i=0;i<60;i++)); do echo output; done; printf '%s%s\\n' wox-reopen- ready",
			"1..60 | ForEach-Object { Write-Output 'output' }; Write-Output ('wox-reopen-' + 'ready')")
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			return strings.Contains(node.Value, "wox-reopen-ready") && !strings.Contains(node.Value, "wox-interactive-ready")
		})
		position := woxui.Point{X: screen.Bounds.X + screen.Bounds.Width/2, Y: screen.Bounds.Y + screen.Bounds.Height/2}
		for range 24 {
			if err := client.Pointer(ctx, woxui.PointerEvent{Kind: woxui.PointerScroll, Position: position, Scroll: woxui.Point{Y: 60}}); err != nil {
				t.Fatalf("scroll up in restored shell: %v", err)
			}
		}
		waitForOutput("wox-interactive-ready-历史")
		afterScroll := command("printf '%s%s\\n' wox-after- scroll", "Write-Output ('wox-after-' + 'scroll')")
		waitForNode(screenID, func(node woxui.AccessibilityNode) bool {
			text := strings.TrimSpace(node.Value)
			return strings.Contains(text, "wox-after-scroll") && strings.ContainsRune(">$#%", rune(text[len(text)-1]))
		})
		snapshot, err = client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read results after continuing history: %v", err)
		}
		if _, found := shellResult(snapshot, reopened); found {
			t.Fatal("continuing a saved Shell added a live result")
		}
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher with the Shell query: %v", err)
		}
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, ">")
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> ")
		for _, continued := range []string{reopened, afterScroll} {
			if _, found := shellResult(snapshot, continued); found {
				t.Fatalf("continuing a saved Shell created a new history result for %q", continued)
			}
		}
		if err := client.Perform(ctx, saved, woxui.AccessibilityActionActivate, ""); err != nil {
			t.Fatalf("open the saved Shell history after re-show: %v", err)
		}
		waitForOutput("wox-reopen-ready")
		waitForOutput("wox-after-scroll")
	})
}
