package sys

import (
	"fmt"
	"testing"

	"wox/plugin"
)

func TestDevCommandsIncludeToolbarProgressPreview(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "test_toolbar_progress" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("toolbar progress preview must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("toolbar progress preview command is missing")
}

func TestDevCommandsIncludeOpenOnboarding(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildDevCommands() {
		if command.ID == "open_onboarding" {
			if command.Action == nil || !command.PreventHideAfterAction {
				t.Fatal("open onboarding must stay visible and executable")
			}
			return
		}
	}

	t.Fatal("open onboarding command is missing")
}

func TestLimitSysQueryResultsKeepsHighestScoresStable(t *testing.T) {
	results := []plugin.QueryResult{{Title: "first", Score: 100}, {Title: "second", Score: 100}}
	for score := int64(31); score >= 0; score-- {
		results = append(results, plugin.QueryResult{Title: fmt.Sprintf("score-%d", score), Score: score})
	}

	limited := limitSysQueryResults(results)
	if len(limited) != sysQueryResultLimit {
		t.Fatalf("result count = %d, want %d", len(limited), sysQueryResultLimit)
	}
	if limited[0].Title != "first" || limited[1].Title != "second" || limited[len(limited)-1].Score != 4 {
		t.Fatalf("limited results did not preserve the highest scores and stable ties: %#v", limited)
	}
}
