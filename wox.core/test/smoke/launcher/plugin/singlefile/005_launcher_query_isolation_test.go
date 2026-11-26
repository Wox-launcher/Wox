//go:build wox_ui_smoke

package singlefile

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"wox/resource"
	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Test005LauncherQueryIsolation drives a cooperative slow plugin through the real Node host.
func Test005LauncherQueryIsolation(t *testing.T) {
	writeSingleFilePlugin(t, "Wox.Plugin.SmokeSlow.js", slowNodePluginSource)
	smoke.Case(t, func(ctx context.Context, client *automationdriver.Client) {
		smoke.ShowLauncher(t, ctx, client)
		loadCtx, cancelLoad := context.WithTimeout(ctx, 12*time.Second)
		defer cancelLoad()
		waitForPluginResult(t, loadCtx, client, "phase2slow ", "Started 0, canceled 0")
		for attempt := 1; attempt <= 3; attempt++ {
			query := fmt.Sprintf("100+%d", attempt)
			title := fmt.Sprint(100 + attempt)
			if err := client.Perform(ctx, "launcher.query.input", woxui.AccessibilityActionSetValue, query); err != nil {
				t.Fatal(err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				input, hasInput := automationdriver.Find(snapshot, "launcher.query.input")
				results, hasResults := automationdriver.Find(snapshot, "launcher.results")
				_, showsCalculator := resultByLabel(snapshot, title)
				return hasInput && input.Value == query && hasResults && results.Value != "complete" && showsCalculator
			}); err != nil {
				t.Fatalf("calculator result was not visible while the slow plugin was running: %v", err)
			}
			if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
				input, hasInput := automationdriver.Find(snapshot, "launcher.query.input")
				results, hasResults := automationdriver.Find(snapshot, "launcher.results")
				return hasInput && input.Value == query && hasResults && results.Value == "complete"
			}); err != nil {
				t.Fatalf("global query did not finish after the slow plugin's deadline: %v", err)
			}
		}
		snapshot := smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "100+4")
		if _, found := resultByLabel(snapshot, "104"); !found {
			t.Fatal("calculator result missing while the third-party circuit is open")
		}
		// The fourth global query must skip the plugin. An explicit query can
		// still probe it and verify that every previous deadline reached Node.
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "phase2slow ")
		if _, found := resultByLabel(snapshot, "Started 3, canceled 3"); !found {
			t.Fatalf("unexpected Node activity after the circuit opened: %+v", launcherResults(snapshot))
		}
		snapshot = smoke.SetLauncherQueryAndWaitComplete(t, ctx, client, "phase2slow wait")
		for _, lang := range []string{"en_US", "zh_CN"} {
			data, err := resource.GetLangJson(ctx, lang)
			if err != nil {
				t.Fatal(err)
			}
			var catalog map[string]string
			if err := json.Unmarshal(data, &catalog); err != nil {
				t.Fatal(err)
			}
			if _, found := resultByLabel(snapshot, catalog["plugin_manager_query_timeout"]); found {
				return
			}
		}
		t.Fatalf("explicit timeout did not produce a result: %+v", launcherResults(snapshot))
	})
}

const slowNodePluginSource = `// {
//   "Id": "com.wox.smoke.singlefile.slow",
//   "Name": "Single-file Slow Node Smoke",
//   "Version": "1.0.0",
//   "MinWoxVersion": "2.4.2",
//   "Runtime": "NODEJS",
//   "TriggerKeywords": ["*", "phase2slow"],
//   "SupportedOS": ["Windows", "Linux", "Macos"]
// }

let started = 0
let canceled = 0

module.exports.plugin = {
  async init() {},
  async query(ctx, query) {
    if ((query.IsGlobalQuery() && /^100\+\d+$/.test(query.RawQuery)) || query.RawQuery === "phase2slow wait") {
      started++
      await new Promise((resolve, reject) => {
        const timer = setTimeout(resolve, 10000)
        ctx.Signal?.addEventListener("abort", () => {
          canceled++
          clearTimeout(timer)
          reject(ctx.Signal.reason)
        }, { once: true })
      })
    }
    if (query.IsGlobalQuery()) return { Results: [] }
    return { Results: [{ Title: "Started " + started + ", canceled " + canceled }] }
  }
}
`
