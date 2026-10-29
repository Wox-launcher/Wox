//go:build wox_ui_smoke

package hotkey

import (
	"context"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	navigationSetting = "NavigationBindings"
	navigationQuery   = "wox-smoke quick-select "
	firstResult       = "Quick select first fixture"
	secondResult      = "Quick select second fixture"
)

// Test004SettingNavigationBindings verifies navigation and hold-to-number coexist in the real launcher.
// Flow: select Emacs and Vim in Hotkey settings -> navigate results and actions -> interrupt a modifier hold -> release and hold again -> choose the second fixture.
// Evidence: selection follows the configured keys, interrupted holds show no numbers, and a fresh hold numbers results and runs the selected action.
func Test004SettingNavigationBindings(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previous := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/hotkey", navigationSetting)
		t.Cleanup(func() { smoke.RestoreSettingChoice(t, client, "/hotkey", navigationSetting, previous) })
		emacs := smoke.SelectSettingChoiceByIndex(t, ctx, client, navigationSetting, 1)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, navigationQuery)
		selectFirstNavigationResult(t, ctx, client)
		modifier := woxui.KeyModifierAlt
		if runtime.GOOS == "darwin" {
			modifier = woxui.KeyModifierControl
		}
		pressNavigation(t, ctx, client, "n", modifier, secondResult)
		pressNavigation(t, ctx, client, "p", modifier, firstResult)

		if got := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/hotkey", navigationSetting); got != emacs {
			t.Fatalf("reopened navigation setting = %q, want %q", got, emacs)
		}
		vim := smoke.SelectSettingChoiceByIndex(t, ctx, client, navigationSetting, 2)
		if err := client.Hide(ctx); err != nil {
			t.Fatal(err)
		}
		smoke.ShowLauncher(t, ctx, client)
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, navigationQuery)
		selectFirstNavigationResult(t, ctx, client)
		pressNavigation(t, ctx, client, "j", modifier, secondResult)
		pressNavigation(t, ctx, client, "k", modifier, firstResult)

		const gridQuery = "wox-smoke grid-500 "
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, gridQuery)
		for _, step := range []struct{ key, result string }{{"l", "launcher.result.perf-grid-0001"}, {"h", "launcher.result.perf-grid-0000"}} {
			if err := client.PressKey(ctx, woxui.Key(step.key), modifier); err != nil {
				t.Fatalf("navigate grid with %s: %v", step.key, err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				result, found := automationdriver.Find(snapshot, step.result)
				input, inputFound := automationdriver.Find(snapshot, "launcher.query.input")
				return found && result.Selected && inputFound && input.Value == gridQuery
			}); err != nil {
				t.Fatalf("wait for grid selection %s: %v", step.result, err)
			}
		}

		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "> echo wox-navigation-smoke")
		panel := smoke.OpenResultActionPanel(t, ctx, client)
		selectedAction := navigationSelectedAction(panel)
		if selectedAction == "" {
			t.Fatal("action panel has no selected action")
		}
		if err := client.PressKey(ctx, "j", modifier); err != nil {
			t.Fatalf("navigate action panel: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			selected := navigationSelectedAction(snapshot)
			input, found := automationdriver.Find(snapshot, "action-search")
			return found && input.Value == "" && selected != "" && selected != selectedAction
		}); err != nil {
			t.Fatalf("wait for action selection without changing the filter: %v", err)
		}
		if err := client.PressKey(ctx, woxui.KeyEscape, 0); err != nil {
			t.Fatalf("close action panel: %v", err)
		}
		smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, navigationQuery)
		selectFirstNavigationResult(t, ctx, client)

		holdKey, holdModifier := woxui.KeyAlt, woxui.KeyModifierAlt
		if runtime.GOOS == "darwin" {
			holdKey, holdModifier = woxui.KeyMeta, woxui.KeyModifierMeta
		}
		t.Cleanup(func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = client.SendKey(releaseCtx, holdKey, holdModifier, false)
		})
		started := time.Now()
		if err := client.SendKey(ctx, holdKey, holdModifier, true); err != nil {
			t.Fatalf("hold quick-select modifier: %v", err)
		}
		interruptKey, interruptModifier := woxui.Key("j"), modifier
		if runtime.GOOS == "darwin" {
			interruptKey, interruptModifier = woxui.KeyArrowDown, holdModifier
		}
		if err := client.PressKey(ctx, interruptKey, interruptModifier); err != nil {
			t.Fatalf("interrupt quick-select hold: %v", err)
		}
		if time.Since(started) >= 300*time.Millisecond {
			t.Fatal("interruption arrived too late to test the pending hold")
		}
		if runtime.GOOS != "darwin" {
			waitNavigationResult(t, ctx, client, secondResult)
		}
		assertNoQuickNumbers(t, ctx, client, 700*time.Millisecond)
		if err := client.SendKey(ctx, holdKey, holdModifier, false); err != nil {
			t.Fatalf("release interrupted hold: %v", err)
		}
		if err := client.SendKey(ctx, holdKey, holdModifier, true); err != nil {
			t.Fatalf("start a fresh hold: %v", err)
		}
		snapshot, err := client.WaitFor(ctx, navigationResultsNumbered)
		if err != nil {
			t.Fatalf("wait for numbers on fresh hold: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
		if err := client.PressKey(ctx, interruptKey, interruptModifier); err != nil {
			t.Fatalf("interrupt visible numbering: %v", err)
		}
		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			return resultNumber(snapshot, firstResult) == "" && resultNumber(snapshot, secondResult) == ""
		}); err != nil {
			t.Fatalf("wait for interrupted numbering to disappear: %v", err)
		}
		assertNoQuickNumbers(t, ctx, client, 700*time.Millisecond)
		if err := client.SendKey(ctx, holdKey, holdModifier, false); err != nil {
			t.Fatalf("release second interrupted hold: %v", err)
		}
		if err := client.SendKey(ctx, holdKey, holdModifier, true); err != nil {
			t.Fatalf("start hold before numbered action: %v", err)
		}
		snapshot, err = client.WaitFor(ctx, navigationResultsNumbered)
		if err != nil {
			t.Fatalf("wait for numbering before action: %v", err)
		}
		if err := client.PressKey(ctx, woxui.Key(resultNumber(snapshot, secondResult)), holdModifier); err != nil {
			t.Fatalf("activate numbered result: %v", err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && !state.Visible && state.Lifecycle == "hidden"
		}); err != nil {
			t.Fatalf("wait for numbered result action: %v", err)
		}
		if got := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/hotkey", navigationSetting); got != vim {
			t.Fatalf("reopened navigation setting = %q, want %q", got, vim)
		}
	})
}

func navigationSelectedAction(snapshot woxwidget.AutomationSnapshot) string {
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "action-result-") && node.Selected {
			return node.AutomationID
		}
	}
	return ""
}

func pressNavigation(t *testing.T, ctx context.Context, client *automationdriver.Client, key woxui.Key, modifier woxui.KeyModifiers, expected string) {
	t.Helper()
	if err := client.PressKey(ctx, key, modifier); err != nil {
		t.Fatalf("press navigation key %s: %v", key, err)
	}
	waitNavigationResult(t, ctx, client, expected)
}

// selectFirstNavigationResult accounts for usage ranking from earlier shared-process cases.
func selectFirstNavigationResult(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		_, first := smoke.FindLauncherResult(snapshot, firstResult)
		_, second := smoke.FindLauncherResult(snapshot, secondResult)
		return first && second
	})
	if err != nil {
		t.Fatalf("wait for navigation fixtures: %v", err)
	}
	id, _ := smoke.FindLauncherResult(snapshot, firstResult)
	first, _ := automationdriver.Find(snapshot, id)
	if !first.Selected {
		if err := client.PressKey(ctx, woxui.KeyArrowDown, 0); err != nil {
			t.Fatalf("select first navigation fixture: %v", err)
		}
	}
	waitNavigationResult(t, ctx, client, firstResult)
}

func waitNavigationResult(t *testing.T, ctx context.Context, client *automationdriver.Client, expected string) {
	t.Helper()
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		id, found := smoke.FindLauncherResult(snapshot, expected)
		result, exists := automationdriver.Find(snapshot, id)
		query, queryFound := automationdriver.Find(snapshot, "launcher.query.input")
		return found && exists && result.Selected && queryFound && query.Value == navigationQuery
	}); err != nil {
		t.Fatalf("wait for selected result %q without changing query: %v", expected, err)
	}
}

func resultNumber(snapshot woxwidget.AutomationSnapshot, label string) string {
	id, found := smoke.FindLauncherResult(snapshot, label)
	if !found {
		return "missing"
	}
	node, found := automationdriver.Find(snapshot, id)
	if !found {
		return "missing"
	}
	return node.Value
}

func navigationResultsNumbered(snapshot woxwidget.AutomationSnapshot) bool {
	first, second := resultNumber(snapshot, firstResult), resultNumber(snapshot, secondResult)
	return (first == "1" && second == "2") || (first == "2" && second == "1")
}

// assertNoQuickNumbers observes the complete hold delay, not just one intermediate frame.
func assertNoQuickNumbers(t *testing.T, ctx context.Context, client *automationdriver.Client, duration time.Duration) {
	t.Helper()
	deadline := time.NewTimer(duration)
	defer deadline.Stop()
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read interrupted hold: %v", err)
		}
		for _, label := range []string{firstResult, secondResult} {
			if value := resultNumber(snapshot, label); strings.TrimSpace(value) != "" {
				t.Fatalf("interrupted hold result %q has number %q", label, value)
			}
		}
		select {
		case <-deadline.C:
			return
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatalf("observe interrupted hold: %v", ctx.Err())
		}
	}
}
