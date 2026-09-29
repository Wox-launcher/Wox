//go:build wox_ui_smoke

package ui

import (
	"context"
	"math"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test007SettingUIResultGutters verifies that theme gutters never expose a ninth result.
// Flow: choose eight results -> apply every built-in theme -> query 500 rows -> wrap to the last row -> query two rows.
// Evidence: exactly eight whole rows fit without partial extras, the last selection stays visible, and short lists keep only the theme result gutter.
func Test007SettingUIResultGutters(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousTheme := persistedThemeID(t)
		previousLimit := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "MaxResultCount")
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			openInstalledThemes(t, cleanupCtx, client)
			if persistedThemeID(t) != previousTheme {
				restorePreviousTheme(t, cleanupCtx, client, previousTheme)
			}
			smoke.RestoreSettingChoice(t, client, "/appearance", "MaxResultCount", previousLimit)
		})
		smoke.SelectSettingChoiceByLabel(t, ctx, client, "setting-choice-MaxResultCount", "8")
		for _, theme := range []struct {
			id     string
			bottom float32
		}{
			{"00bd6884-bd4c-49c7-9053-1af14f00fe36", 8},
			{"44a933d5-e6de-4c1f-8ee5-b2305c6abdf3", 8},
			{"53c1d0a4-ffc8-4d90-91dc-b408fb0b9a03", 8},
			{"92dc0ea7-a52f-4b0a-9f0d-7cb36a634860", 8},
			{"532238bc-6eda-4011-a080-c365b67486fc", 8},
		} {
			openInstalledThemes(t, ctx, client)
			if persistedThemeID(t) != theme.id {
				restorePreviousTheme(t, ctx, client, theme.id)
			}
			if err := client.Hide(ctx); err != nil {
				t.Fatal(err)
			}
			smoke.ShowLauncher(t, ctx, client)
			snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke list-500 ")
			assertResultGutters(t, snapshot, 8, theme.bottom)
			// Up from the initial selection wraps to the last result, exercising end clamping.
			if err := client.PressKey(ctx, woxui.KeyArrowUp, 0); err != nil {
				t.Fatal(err)
			}
			snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				for _, node := range snapshot.Tree.Nodes {
					if strings.HasPrefix(node.AutomationID, "launcher.result.") && node.Selected && node.Label == "Perf list result 0499" {
						return true
					}
				}
				return false
			})
			if err != nil {
				t.Fatalf("wait for final result: %v", err)
			}
			assertResultGutters(t, snapshot, 8, theme.bottom)
			snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "wox-smoke quick-select ")
			assertResultGutters(t, snapshot, 2, theme.bottom)
		}
	})
}

// assertResultGutters measures logical layout bounds, excluding footer underlay and virtual overscan.
func assertResultGutters(t *testing.T, snapshot woxwidget.AutomationSnapshot, want int, bottom float32) {
	t.Helper()
	area, found := automationdriver.Find(snapshot, "launcher.results")
	if !found {
		t.Fatal("missing results area")
	}
	viewport, found := automationdriver.Find(snapshot, "launcher.results.viewport")
	if !found {
		t.Fatal("missing result viewport")
	}
	// The scroll painter may continue behind a translucent toolbar. The enclosing
	// result area bounds still end at the toolbar and define the usable viewport.
	end := min(viewport.Bounds.Y+viewport.Bounds.Height, area.Bounds.Y+area.Bounds.Height-bottom)
	full, partial := 0, 0
	var selectedVisible bool
	var lastBottom float32
	for _, node := range snapshot.Tree.Nodes {
		if !strings.HasPrefix(node.AutomationID, "launcher.result.") {
			continue
		}
		top, rowEnd := node.Bounds.Y, node.Bounds.Y+node.Bounds.Height
		if rowEnd <= viewport.Bounds.Y+0.1 || top >= end-0.1 {
			continue
		}
		if top < viewport.Bounds.Y-0.1 || rowEnd > end+0.1 {
			partial++
		} else {
			full++
		}
		if node.Selected {
			selectedVisible = top >= viewport.Bounds.Y-0.1 && rowEnd <= end+0.1
		}
		lastBottom = max(lastBottom, rowEnd)
	}
	if full != want || partial != 0 || !selectedVisible {
		t.Fatalf("visible rows full=%d partial=%d selectedVisible=%t, want %d whole rows; area=%+v viewport=%+v", full, partial, selectedVisible, want, area.Bounds, viewport.Bounds)
	}
	gap := area.Bounds.Y + area.Bounds.Height - lastBottom
	if math.Abs(float64(gap-bottom)) > 0.1 {
		t.Fatalf("bottom gutter=%.2f, want %.2f", gap, bottom)
	}
	smoke.AssertNoDiagnostics(t, snapshot)
}
