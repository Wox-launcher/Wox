package ui

import (
	"context"
	"strings"
	"testing"

	"wox/ui/contract"
	"wox/updater"
	"wox/util"
)

func TestCoreServicesVersion(t *testing.T) {
	version, err := NewCoreServices().Version(context.Background(), "session")
	if err != nil {
		t.Fatalf("Version returned error: %v", err)
	}
	if version != updater.CURRENT_VERSION {
		t.Fatalf("Version = %q, want %q", version, updater.CURRENT_VERSION)
	}
}

func TestCoreServicesUpdateChannelVersions(t *testing.T) {
	previousProvider := updateChannelVersionsProvider
	t.Cleanup(func() {
		updateChannelVersionsProvider = previousProvider
	})

	seenSessionID := ""
	updateChannelVersionsProvider = func(ctx context.Context) []updater.UpdateChannelVersion {
		seenSessionID = util.GetContextSessionId(ctx)
		return []updater.UpdateChannelVersion{
			{Channel: "stable", LatestVersion: "2.4.0"},
			{Channel: "beta", Error: "unavailable"},
		}
	}

	versions, err := NewCoreServices().UpdateChannelVersions(context.Background(), "settings-session")
	if err != nil {
		t.Fatalf("UpdateChannelVersions returned error: %v", err)
	}
	if seenSessionID != "settings-session" {
		t.Fatalf("provider session = %q, want settings-session", seenSessionID)
	}
	if len(versions) != 2 || versions[0].Channel != "stable" || versions[0].LatestVersion != "2.4.0" || versions[1].Error != "unavailable" {
		t.Fatalf("UpdateChannelVersions = %+v", versions)
	}
}

func TestCoreServicesRestartRuntimeRejectsUnsupportedRuntime(t *testing.T) {
	err := NewCoreServices().RestartRuntime(context.Background(), "session", "GO")
	if err == nil || !strings.Contains(err.Error(), "does not support restart") {
		t.Fatalf("RestartRuntime error = %v, want unsupported runtime error", err)
	}
}

func TestCoreServicesRejectsUnsupportedPluginOperation(t *testing.T) {
	err := NewCoreServices().OperatePlugin(context.Background(), "session", "plugin", contract.PluginOperation("unknown"))
	if err == nil || !strings.Contains(err.Error(), "unsupported plugin operation") {
		t.Fatalf("OperatePlugin error = %v, want unsupported operation error", err)
	}
}

func TestCoreServicesRejectsUnsupportedThemeOperation(t *testing.T) {
	err := NewCoreServices().OperateTheme(context.Background(), "session", "theme", contract.ThemeOperation("unknown"))
	if err == nil || !strings.Contains(err.Error(), "unsupported theme operation") {
		t.Fatalf("OperateTheme error = %v, want unsupported operation error", err)
	}
}

func TestCoreServicesRejectsUnsupportedBillingSession(t *testing.T) {
	_, err := NewCoreServices().BillingSession(context.Background(), "session", contract.BillingSessionKind("unknown"))
	if err == nil || !strings.Contains(err.Error(), "unsupported billing session kind") {
		t.Fatalf("BillingSession error = %v, want unsupported kind error", err)
	}
}

func TestDecodeTrayQueriesPreservesMissingQueryAsEmpty(t *testing.T) {
	queries, err := decodeTrayQueries(`[{"HideQueryBox":true,"Width":12}]`)
	if err != nil {
		t.Fatalf("decodeTrayQueries returned error: %v", err)
	}
	if len(queries) != 1 || queries[0].Query != "" || !queries[0].HideQueryBox || queries[0].Width != 12 {
		t.Fatalf("decodeTrayQueries = %+v", queries)
	}
}

func TestCoreServicesRejectsUnsupportedLanguage(t *testing.T) {
	_, err := NewCoreServices().LanguageJSON(context.Background(), "session", "unsupported")
	if err == nil || !strings.Contains(err.Error(), "unsupported lang code") {
		t.Fatalf("LanguageJSON error = %v, want unsupported language error", err)
	}
}

func TestCoreServicesRejectsUnsupportedManagedModelKind(t *testing.T) {
	_, err := NewCoreServices().ManagedModelStatuses(context.Background(), "session", contract.ManagedModelKind("unknown"))
	if err == nil || !strings.Contains(err.Error(), "unsupported managed model kind") {
		t.Fatalf("ManagedModelStatuses error = %v, want unsupported kind error", err)
	}
}

func TestCoreServicesValidatesHotkeyInteractionInput(t *testing.T) {
	_, err := NewCoreServices().StartHotkeyRecording(context.Background(), "session", "", nil)
	if err == nil || !strings.Contains(err.Error(), "purpose is required") {
		t.Fatalf("StartHotkeyRecording error = %v, want purpose validation error", err)
	}
	if err := NewCoreServices().SubmitHotkeyRecordingCandidate(context.Background(), "session", " "); err == nil || !strings.Contains(err.Error(), "hotkey is required") {
		t.Fatalf("SubmitHotkeyRecordingCandidate error = %v, want hotkey validation error", err)
	}
}

func TestCoreServicesValidatesRuntimeInteractionInput(t *testing.T) {
	service := NewCoreServices()
	if err := service.ShowTooltip(context.Background(), "session", contract.TooltipOptions{}); err == nil || !strings.Contains(err.Error(), "tooltip name and text") {
		t.Fatalf("ShowTooltip error = %v, want tooltip validation error", err)
	}
	if _, err := service.ResultPreview(context.Background(), "session", "", "", ""); err == nil || !strings.Contains(err.Error(), "sessionId, queryId and id") {
		t.Fatalf("ResultPreview error = %v, want preview identity validation error", err)
	}
	if err := service.AnswerAIQuestion(context.Background(), "session", "", "answer"); err == nil || !strings.Contains(err.Error(), "questionId is required") {
		t.Fatalf("AnswerAIQuestion error = %v, want question validation error", err)
	}
}
