//go:build wox_ui_smoke

package ui

import (
	"context"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

// Test003SettingUIDensity verifies that the UI density choice changes the launcher result-row geometry.
// Flow: select Compact and query the calculator -> select Comfortable and repeat the same query.
// Evidence: the real Comfortable calculator result row is taller than the Compact row.
func Test003SettingUIDensity(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		previousValue := smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "UiDensity")
		t.Cleanup(func() {
			if previousValue != "comfortable" {
				smoke.RestoreSettingChoice(t, client, "/appearance", "UiDensity", previousValue)
			}
		})

		setUIDensity(t, ctx, client, "Compact", 0)
		smoke.ShowLauncher(t, ctx, client)
		compactRowHeight := calculatorResultRowHeight(t, ctx, client)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("hide launcher after verifying Compact density: %v", err)
		}

		setUIDensity(t, ctx, client, "Comfortable", 2)
		smoke.ShowLauncher(t, ctx, client)
		comfortableRowHeight := calculatorResultRowHeight(t, ctx, client)
		if comfortableRowHeight <= compactRowHeight {
			t.Fatalf("comfortable result-row height = %.1f, want greater than compact height %.1f", comfortableRowHeight, compactRowHeight)
		}
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read launcher semantics after verifying Comfortable density: %v", err)
		}
		smoke.AssertNoDiagnostics(t, snapshot)
	})
}

// setUIDensity applies one product-defined density option through the native UI Settings section.
func setUIDensity(t *testing.T, ctx context.Context, client *automationdriver.Client, density string, optionIndex int) {
	t.Helper()
	smoke.OpenSettingsAndReadChoice(t, ctx, client, "/appearance", "UiDensity")
	smoke.SelectSettingChoiceByIndex(t, ctx, client, "UiDensity", optionIndex)
	if err := client.Hide(ctx); err != nil {
		t.Fatalf("close UI settings after selecting %s density: %v", density, err)
	}
}

// calculatorResultRowHeight queries a deterministic calculator result and returns its rendered row height.
func calculatorResultRowHeight(t *testing.T, ctx context.Context, client *automationdriver.Client) float32 {
	t.Helper()
	snapshot := smoke.ReplaceLauncherQuery(t, ctx, client, "1+1")
	snapshot, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
		for _, node := range snapshot.Tree.Nodes {
			if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" && node.Bounds.Height > 0 {
				return resultsFound && results.Value == "complete"
			}
		}
		return false
	})
	if err != nil {
		t.Fatalf("wait for calculator result row: %v", err)
	}
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") && strings.TrimSpace(node.Label) == "2" {
			return node.Bounds.Height
		}
	}
	t.Fatal("calculator result row was missing after the completed query")
	return 0
}
