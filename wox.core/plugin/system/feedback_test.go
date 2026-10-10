package system

import (
	"context"
	"net/url"
	"os"
	"strings"
	"testing"
	"wox/plugin"
	"wox/supervisor"
	"wox/updater"
	"wox/util"

	"gopkg.in/yaml.v3"
)

// TestGitHubIssueURLPrefills checks the shared URL path used by bug, feature, and crash actions.
func TestGitHubIssueURLPrefills(t *testing.T) {
	t.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
	t.Setenv("XDG_SESSION_TYPE", "x11")
	t.Setenv("WAYLAND_DISPLAY", "")
	t.Setenv("DISPLAY", ":0")
	for _, template := range []string{feedbackBugTemplate, feedbackFeatureTemplate} {
		issueURL, err := url.Parse(githubIssueURL(template, "Test & details"))
		if err != nil {
			t.Fatal(err)
		}
		query := issueURL.Query()
		formData, err := os.ReadFile("../../../.github/ISSUE_TEMPLATE/" + template)
		if err != nil {
			t.Fatal(err)
		}
		var form struct {
			Body []struct {
				Type, ID    string
				Validations struct{ Required bool }
			}
		}
		if err := yaml.Unmarshal(formData, &form); err != nil {
			t.Fatal(err)
		}
		for _, field := range form.Body {
			if template == feedbackFeatureTemplate && field.ID == "platform" {
				t.Error("feature form must not ask for a platform")
			}
			if field.Type == "dropdown" {
				t.Errorf("%s: %s must be a text field", template, field.ID)
			}
			if field.ID == "linux_environment" && field.Validations.Required {
				t.Error("Linux environment must be optional")
			}
			if field.Type == "input" && field.ID != "linux_environment" && !query.Has(field.ID) {
				t.Errorf("%s: missing prefill for %s", template, field.ID)
			}
		}
		platform := "Linux"
		if util.IsWindows() {
			platform = "Windows"
		} else if util.IsMacOS() {
			platform = "macOS"
		}
		if template == feedbackFeatureTemplate {
			platform = ""
			if query.Has("platform") {
				t.Error("feature URL must not include a platform")
			}
		}
		for key, want := range map[string]string{
			"template": template, "title": "Test & details", "wox_version": updater.CURRENT_VERSION, "platform": platform,
		} {
			if query.Get(key) != want {
				t.Errorf("%s: %s = %q, want %q", template, key, query.Get(key), want)
			}
		}
		wantEnvironment := ""
		if template == feedbackBugTemplate && util.IsLinux() {
			wantEnvironment = "GNOME / X11 (Xorg)"
		}
		if query.Get("linux_environment") != wantEnvironment {
			t.Errorf("%s: Linux environment = %q, want %q", template, query.Get("linux_environment"), wantEnvironment)
		}
		for _, key := range []string{"desktop_environment", "display_server"} {
			if query.Has(key) {
				t.Errorf("%s: obsolete parameter %s", template, key)
			}
		}
	}
}

type feedbackTestAPI struct {
	plugin.API
}

func (feedbackTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

func TestQueryShowsCommonOperationsByDefault(t *testing.T) {
	previousEnv := util.ProdEnv
	util.ProdEnv = ""
	t.Cleanup(func() { util.ProdEnv = previousEnv })
	if err := util.GetLocation().Init(); err != nil {
		t.Fatalf("init location: %v", err)
	}

	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{})
	if len(response.Results) != 3 {
		t.Fatalf("default feedback query should show 3 operations, got %d", len(response.Results))
	}

	wantTitles := []string{
		"i18n:plugin_feedback_bug_title",
		"i18n:plugin_feedback_feature_title",
		"i18n:plugin_feedback_clear_logs_title",
	}
	for i, want := range wantTitles {
		if response.Results[i].Title != want {
			t.Fatalf("default result %d title = %q, want %q", i, response.Results[i].Title, want)
		}
		if response.Results[i].Preview.PreviewData != "" {
			t.Fatalf("default result %d should not have a preview", i)
		}
	}
}

func TestFeedbackRestartOnlyShownInProduction(t *testing.T) {
	previousEnv := util.ProdEnv
	t.Cleanup(func() { util.ProdEnv = previousEnv })
	p := &FeedbackPlugin{api: feedbackTestAPI{}}
	for _, production := range []bool{false, true} {
		util.ProdEnv = ""
		if production {
			util.ProdEnv = "true"
		}
		results := p.Query(context.Background(), plugin.Query{}).Results
		want := 3
		if production {
			want = 4
		}
		if len(results) != want {
			t.Fatalf("production=%t: got %d results, want %d", production, len(results), want)
		}
		if production {
			result := results[3]
			if result.Title != "i18n:plugin_feedback_restart_without_plugins_title" || len(result.Actions) != 1 || !result.Actions[0].IsDefault || result.Actions[0].Action == nil {
				t.Fatalf("invalid troubleshooting restart result: %+v", result)
			}
		}
	}
}

func TestFeedbackRestartRestoresPluginsInTroubleshootingMode(t *testing.T) {
	previousArgs := os.Args
	t.Cleanup(func() { os.Args = previousArgs })
	p := &FeedbackPlugin{api: feedbackTestAPI{}}
	for _, disabled := range []bool{false, true} {
		os.Args = []string{"wox.exe"}
		title := "i18n:plugin_feedback_restart_without_plugins_title"
		subtitle := "i18n:plugin_feedback_restart_without_plugins_subtitle"
		if disabled {
			os.Args = append(os.Args, util.ArgNoThirdPartyPlugins)
			title = "i18n:plugin_feedback_restart_title"
			subtitle = "i18n:plugin_feedback_restart_subtitle"
		}
		result := p.buildRestartResult()
		if result.Title != title || result.SubTitle != subtitle || result.Actions[0].Name != title {
			t.Fatalf("disabled=%t: unexpected restart result: %+v", disabled, result)
		}
		if !strings.Contains(result.Icon.ImageData, `stroke="var(--wox-theme-icon-color)"`) {
			t.Fatal("restart result icon must follow the launcher theme")
		}
	}
}

func TestQueryUnknownCommandReturnsNoResults(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{Command: "unknown"})
	if len(response.Results) != 0 {
		t.Fatalf("unknown command should return no results, got %d", len(response.Results))
	}
}

func TestQueryCrashCommandListsCrashOrEmptyState(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	response := pluginInstance.Query(context.Background(), plugin.Query{Command: feedbackCommandCrash})
	if len(response.Results) == 0 {
		t.Fatal("crash command should return at least one result")
	}
	if response.Results[0].Title == "i18n:plugin_feedback_bug_title" {
		t.Fatal("crash command should not show the default bug operation")
	}
}

// TestBuildCrashIncidentResultPreservesNewestFirstOrdering verifies the score used by the launcher cache.
func TestBuildCrashIncidentResultPreservesNewestFirstOrdering(t *testing.T) {
	pluginInstance := &FeedbackPlugin{api: feedbackTestAPI{}}

	older := pluginInstance.buildCrashIncidentResult(context.Background(), supervisor.CrashIncident{
		ID:         "older",
		DetectedAt: 100,
	})
	newer := pluginInstance.buildCrashIncidentResult(context.Background(), supervisor.CrashIncident{
		ID:         "newer",
		DetectedAt: 200,
	})

	if newer.Score <= older.Score {
		t.Fatalf("newer crash should have a higher result score, newer=%d older=%d", newer.Score, older.Score)
	}
}
