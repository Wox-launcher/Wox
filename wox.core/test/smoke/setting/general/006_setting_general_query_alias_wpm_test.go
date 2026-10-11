//go:build wox_ui_smoke

package general

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxwidget "wox/ui/widget"
)

const (
	wpmInstallAlias        = "g"
	wpmInstallQuery        = "wpm install"
	wpmInstallExpansionLog = "expand query alias: g -> wpm install"
)

// Test006SettingGeneralQueryAliasWpmInstall verifies a one-character alias expands into the WPM query.
// Flow: confirm g does not already list the WPM commands -> save alias g = wpm install in General settings -> query g.
// Evidence: the input stays g, the completed rows match a direct wpm install query, and the runtime log records the expansion.
func Test006SettingGeneralQueryAliasWpmInstall(t *testing.T) {
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		t.Cleanup(func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), smoke.CaseTimeout)
			defer cancel()
			removeQueryAlias(t, cleanupCtx, client, wpmInstallAlias)
		})

		removeQueryAlias(t, ctx, client, wpmInstallAlias)
		smoke.ShowLauncher(t, ctx, client)
		baseline := smoke.ReplaceLauncherQuery(t, ctx, client, wpmInstallAlias)
		if labels := launcherResultLabels(baseline); hasWPMCommandRows(labels) {
			t.Fatalf("g already shows WPM commands %q", labels)
		}

		addQueryAlias(t, ctx, client, wpmInstallAlias, wpmInstallQuery)
		waitForQueryAlias(t, ctx, wpmInstallAlias, true)
		if err := client.Hide(ctx); err != nil {
			t.Fatalf("close settings before alias query: %v", err)
		}
		smoke.ShowLauncher(t, ctx, client)
		installSnapshot := smoke.ReplaceLauncherQuery(t, ctx, client, wpmInstallQuery)
		installLabels := launcherResultLabels(installSnapshot)
		if !hasWPMCommandRows(installLabels) {
			t.Fatalf("wpm install results = %q, want the install and uninstall commands", installLabels)
		}

		logPath := filepath.Join(os.Getenv(automationdriver.SharedDataDirectoryEnvironment), "log", "wox.log")
		logOffset := queryAliasLogSize(t, logPath)
		aliasSnapshot := smoke.ReplaceLauncherQuery(t, ctx, client, wpmInstallAlias)
		input, inputFound := automationdriver.Find(aliasSnapshot, "launcher.query.input")
		results, resultsFound := automationdriver.Find(aliasSnapshot, "launcher.results")
		if !inputFound || input.Value != wpmInstallAlias || !resultsFound || results.Value != "complete" {
			t.Fatalf("alias query input=%q found=%v results=%q found=%v", input.Value, inputFound, results.Value, resultsFound)
		}
		aliasLabels := launcherResultLabels(aliasSnapshot)
		if !reflect.DeepEqual(aliasLabels, installLabels) {
			t.Fatalf("alias results = %q, want the wpm install results %q", aliasLabels, installLabels)
		}
		smoke.WaitForFile(t, ctx, logPath, func(data []byte) bool {
			return int64(len(data)) >= logOffset && strings.Contains(string(data[logOffset:]), wpmInstallExpansionLog)
		})
		smoke.AssertNoDiagnostics(t, aliasSnapshot)
	})
}

// hasWPMCommandRows reports the command list shown for "wpm install" before the trailing space commits the install command.
func hasWPMCommandRows(labels []string) bool {
	found := map[string]bool{}
	for _, label := range labels {
		found[label] = true
	}
	return found["install"] && found["uninstall"]
}

func launcherResultLabels(snapshot woxwidget.AutomationSnapshot) []string {
	labels := make([]string, 0)
	for _, node := range snapshot.Tree.Nodes {
		if strings.HasPrefix(node.AutomationID, "launcher.result.") {
			labels = append(labels, strings.TrimSpace(node.Label))
		}
	}
	sort.Strings(labels)
	return labels
}

func queryAliasLogSize(t *testing.T, path string) int64 {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat Wox log %q: %v", path, err)
	}
	return info.Size()
}
