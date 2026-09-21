//go:build wox_ui_smoke

package query

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/test/smoke"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	queryRequirementFieldID = "requirement-form-field-0"
	queryRequirementSaveID  = "requirement-form-save"
	queryRequirementErrorID = "requirement-form-field-0-error"
)

// writeQueryRequirementPlugin installs one single-file Python plugin used by query-requirement cases.
func writeQueryRequirementPlugin(t *testing.T, fileName, source string) {
	t.Helper()
	userDir := strings.TrimSpace(os.Getenv(automationdriver.SharedUserDataDirectoryEnvironment))
	if userDir == "" {
		t.Fatalf("%s is not configured; run smoke through make smoke", automationdriver.SharedUserDataDirectoryEnvironment)
	}
	directory := filepath.Join(userDir, "plugins", "single-file")
	if err := os.MkdirAll(directory, 0755); err != nil {
		t.Fatalf("create single-file plugin directory: %v", err)
	}
	path := filepath.Join(directory, fileName)
	if err := os.WriteFile(path, []byte(source), 0644); err != nil {
		t.Fatalf("write query requirement plugin %s: %v", path, err)
	}
	t.Cleanup(func() {
		_ = os.Remove(path)
	})
}

// queryRequirementAnyQueryPluginSource returns a plugin that blocks every query until accessKey is set.
func queryRequirementAnyQueryPluginSource(id, name, trigger string) string {
	return `# {
#   "Id": "` + id + `",
#   "Name": "` + name + `",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["` + trigger + `"],
#   "SupportedOS": ["Windows", "Linux", "Macos"],
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "accessKey",
#         "Label": "Access Key",
#         "DefaultValue": "",
#         "Validators": [{ "Type": "not_empty", "Value": {} }]
#       }
#     }
#   ],
#   "QueryRequirements": {
#     "AnyQuery": [
#       {
#         "SettingKey": "accessKey",
#         "Message": "Access key is required before this plugin can search."
#       }
#     ]
#   }
# }

from wox_plugin import PluginInitParams, Query, QueryResponse, Result, WoxImage

class SmokePlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        key = await self.api.get_setting(ctx, "accessKey")
        return QueryResponse(results=[
            Result(
                title="query requirement ready:" + key,
                icon=WoxImage.new_emoji("🧪"),
            )
        ])

plugin = SmokePlugin()
`
}

// queryRequirementCommandPluginSource returns a plugin that blocks only the named command until downloadPath is set.
func queryRequirementCommandPluginSource(id, name, trigger, command string) string {
	return `# {
#   "Id": "` + id + `",
#   "Name": "` + name + `",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["` + trigger + `"],
#   "Commands": [
#     { "Command": "` + command + `", "Description": "Download" }
#   ],
#   "SupportedOS": ["Windows", "Linux", "Macos"],
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "downloadPath",
#         "Label": "Download Path",
#         "DefaultValue": "",
#         "Validators": [{ "Type": "not_empty", "Value": {} }]
#       }
#     }
#   ],
#   "QueryRequirements": {
#     "QueryWithCommand": {
#       "` + command + `": [
#         {
#           "SettingKey": "downloadPath",
#           "Message": "Download path is required before this command can run."
#         }
#       ]
#     }
#   }
# }

from wox_plugin import PluginInitParams, Query, QueryResponse, Result, WoxImage

class SmokePlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        path = await self.api.get_setting(ctx, "downloadPath")
        return QueryResponse(results=[
            Result(
                title="query requirement command:" + (query.command or "") + ":" + path,
                icon=WoxImage.new_emoji("🧪"),
            )
        ])

plugin = SmokePlugin()
`
}

// queryRequirementTwoFieldPluginSource returns a plugin that blocks every query until accessKey and clientId are set.
func queryRequirementTwoFieldPluginSource(id, name, trigger string) string {
	return `# {
#   "Id": "` + id + `",
#   "Name": "` + name + `",
#   "Version": "1.0.0",
#   "MinWoxVersion": "2.4.2",
#   "Runtime": "PYTHON",
#   "TriggerKeywords": ["` + trigger + `"],
#   "SupportedOS": ["Windows", "Linux", "Macos"],
#   "SettingDefinitions": [
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "accessKey",
#         "Label": "Access Key",
#         "DefaultValue": "",
#         "Validators": [{ "Type": "not_empty", "Value": {} }]
#       }
#     },
#     {
#       "Type": "textbox",
#       "Value": {
#         "Key": "clientId",
#         "Label": "Client ID",
#         "DefaultValue": "",
#         "Validators": [{ "Type": "not_empty", "Value": {} }]
#       }
#     }
#   ],
#   "QueryRequirements": {
#     "AnyQuery": [
#       {
#         "SettingKey": "accessKey",
#         "Message": "Access key is required before this plugin can search."
#       },
#       {
#         "SettingKey": "clientId",
#         "Message": "Client ID is required before this plugin can search."
#       }
#     ]
#   }
# }

from wox_plugin import PluginInitParams, Query, QueryResponse, Result, WoxImage

class SmokePlugin:
    async def init(self, ctx, params: PluginInitParams):
        self.api = params.api

    async def query(self, ctx, query: Query):
        key = await self.api.get_setting(ctx, "accessKey")
        client = await self.api.get_setting(ctx, "clientId")
        return QueryResponse(results=[
            Result(
                title="query requirement ready:" + key + ":" + client,
                icon=WoxImage.new_emoji("🧪"),
            )
        ])

plugin = SmokePlugin()
`
}

func queryRequirementFieldAutomationID(index int) string {
	return "requirement-form-field-" + strconv.Itoa(index)
}

func queryRequirementErrorAutomationID(index int) string {
	return queryRequirementFieldAutomationID(index) + "-error"
}

// fillQueryRequirementField commits one setup-form textbox without saving.
func fillQueryRequirementField(t *testing.T, ctx context.Context, client *automationdriver.Client, fieldID, value string) {
	t.Helper()
	if err := client.Perform(ctx, fieldID, woxui.AccessibilityActionSetValue, value); err != nil {
		t.Fatalf("enter query requirement value on %s: %v", fieldID, err)
	}
	if _, err := client.WaitFor(ctx, func(snapshot woxwidget.AutomationSnapshot) bool {
		field, found := automationdriver.Find(snapshot, fieldID)
		return found && field.Value == value
	}); err != nil {
		t.Fatalf("wait for query requirement value %q on %s: %v", value, fieldID, err)
	}
}

// saveQueryRequirementForm activates the setup form save action.
func saveQueryRequirementForm(t *testing.T, ctx context.Context, client *automationdriver.Client) {
	t.Helper()
	if err := client.Perform(ctx, queryRequirementSaveID, woxui.AccessibilityActionActivate, ""); err != nil {
		t.Fatalf("save query requirement settings: %v", err)
	}
}

// waitForQueryRequirementForm keeps issuing query until core shows the setup form.
func waitForQueryRequirementForm(t *testing.T, ctx context.Context, client *automationdriver.Client, query string) woxwidget.AutomationSnapshot {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var last woxwidget.AutomationSnapshot
	for {
		last = smoke.ReplaceLauncherQuery(t, ctx, client, query)
		if queryRequirementFormVisible(last) {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for query requirement form from %q: %s: %v", query, describeQueryRequirementSnapshot(last), ctx.Err())
		case <-ticker.C:
		}
	}
}

// waitForQueryRequirementResult keeps issuing query until the plugin result is visible and the form is gone.
func waitForQueryRequirementResult(t *testing.T, ctx context.Context, client *automationdriver.Client, query, title string) woxwidget.AutomationSnapshot {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var last woxwidget.AutomationSnapshot
	for {
		last = smoke.ReplaceLauncherQuery(t, ctx, client, query)
		if queryRequirementResultVisible(last, title) {
			return last
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for query requirement result %q from %q: %s: %v", title, query, describeQueryRequirementSnapshot(last), ctx.Err())
		case <-ticker.C:
		}
	}
}

// waitForQueryRequirementRefresh waits for the save path to persist settings and re-query the plugin.
func waitForQueryRequirementRefresh(t *testing.T, ctx context.Context, client *automationdriver.Client, title string) woxwidget.AutomationSnapshot {
	t.Helper()
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	var last woxwidget.AutomationSnapshot
	for {
		snapshot, err := client.Snapshot(ctx)
		if err != nil {
			t.Fatalf("read launcher after saving query requirement settings: %v", err)
		}
		last = snapshot
		if queryRequirementResultVisible(snapshot, title) {
			return snapshot
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for query requirement plugin result %q: %s: %v", title, describeQueryRequirementSnapshot(last), ctx.Err())
		case <-ticker.C:
		}
	}
}

// fillAndSaveQueryRequirementField commits one required text field through the setup form save action.
func fillAndSaveQueryRequirementField(t *testing.T, ctx context.Context, client *automationdriver.Client, value string) {
	t.Helper()
	fillQueryRequirementField(t, ctx, client, queryRequirementFieldID, value)
	saveQueryRequirementForm(t, ctx, client)
}

func queryRequirementFormVisible(snapshot woxwidget.AutomationSnapshot) bool {
	results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
	_, fieldFound := automationdriver.Find(snapshot, queryRequirementFieldID)
	_, saveFound := automationdriver.Find(snapshot, queryRequirementSaveID)
	return resultsFound && results.Value == "complete" && fieldFound && saveFound
}

func queryRequirementResultVisible(snapshot woxwidget.AutomationSnapshot, title string) bool {
	results, resultsFound := automationdriver.Find(snapshot, "launcher.results")
	_, formFound := automationdriver.Find(snapshot, queryRequirementSaveID)
	return resultsFound && results.Value == "complete" && !formFound && smoke.HasLauncherResultLabel(snapshot, title)
}

func describeQueryRequirementSnapshot(snapshot woxwidget.AutomationSnapshot) string {
	return automationdriver.DescribeNodes(snapshot, "launcher.results", queryRequirementFieldID, queryRequirementFieldAutomationID(1), queryRequirementSaveID, queryRequirementErrorID, queryRequirementErrorAutomationID(1), "launcher.query.input")
}
