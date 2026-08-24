//go:build wox_ui_smoke

package query

import (
	"context"
	"runtime"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	quickSelectSmokeQuery    = "wox-smoke quick-select "
	quickSelectFirstFixture  = "Quick select first fixture"
	quickSelectSecondFixture = "Quick select second fixture"
	quickSelectFirstNumber   = "1"
	quickSelectSecondNumber  = "2"
)

// Test015LauncherQuickSelect verifies holding the platform Quick Select modifier numbers visible results and a digit runs that result's default action.
// Flow: query two fixture results -> hold Alt or Command until numbers appear -> press 2.
// Evidence: the second result exposes number 2, then its default action hides the real launcher window.
func Test015LauncherQuickSelect(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, quickSelectSmokeQuery)
		if _, found := smoke.FindLauncherResult(snapshot, quickSelectFirstFixture); !found {
			t.Fatal("quick select first fixture result was not found")
		}
		if _, found := smoke.FindLauncherResult(snapshot, quickSelectSecondFixture); !found {
			t.Fatal("quick select second fixture result was not found")
		}
		smoke.AssertNoDiagnostics(t, snapshot)

		modifier, modifiers := quickSelectModifier()
		if err := client.SendKey(ctx, modifier, modifiers, true); err != nil {
			t.Fatalf("hold Quick Select modifier: %v", err)
		}
		t.Cleanup(func() {
			releaseCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = client.SendKey(releaseCtx, modifier, modifiers, false)
		})

		if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
			first, firstFound := smoke.FindLauncherResult(snapshot, quickSelectFirstFixture)
			second, secondFound := smoke.FindLauncherResult(snapshot, quickSelectSecondFixture)
			if !firstFound || !secondFound {
				return false
			}
			firstNode, firstNodeFound := automationdriver.Find(snapshot, first)
			secondNode, secondNodeFound := automationdriver.Find(snapshot, second)
			return firstNodeFound && secondNodeFound && firstNode.Value == quickSelectFirstNumber && secondNode.Value == quickSelectSecondNumber
		}); err != nil {
			t.Fatalf("wait for Quick Select numbers on fixture results: %v", err)
		}

		if err := client.PressKey(ctx, woxui.Key(quickSelectSecondNumber), 0); err != nil {
			t.Fatalf("activate Quick Select number %s: %v", quickSelectSecondNumber, err)
		}
		if _, err := client.WaitForWindowState(ctx, "primary", func(state automationdriver.WindowState) bool {
			return state.Exists && !state.Visible && state.Lifecycle == "hidden"
		}); err != nil {
			t.Fatalf("wait for launcher to hide after Quick Select number %s: %v", quickSelectSecondNumber, err)
		}
	})
}

func quickSelectModifier() (woxui.Key, woxui.KeyModifiers) {
	if runtime.GOOS == "darwin" {
		return woxui.KeyMeta, woxui.KeyModifierMeta
	}
	return woxui.KeyAlt, woxui.KeyModifierAlt
}
