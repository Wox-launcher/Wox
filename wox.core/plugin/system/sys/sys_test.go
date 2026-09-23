package sys

import (
	"context"
	"fmt"
	"testing"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/util/fuzzymatch"
)

func TestReleasePreparedSearchTextClearsCommandIndex(t *testing.T) {
	plugin := &SysPlugin{
		commandSearchIndexes: map[i18n.LangCode][][]*fuzzymatch.PreparedText{
			"en_US": {{fuzzymatch.PrepareText("lock")}},
		},
	}
	plugin.ReleasePreparedSearchText()
	if plugin.commandSearchIndexes != nil {
		t.Fatalf("released command index has %d languages, want none", len(plugin.commandSearchIndexes))
	}
}

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

func TestCommandsIncludeConfetti(t *testing.T) {
	for _, command := range (&SysPlugin{}).buildCommands() {
		if command.ID == "confetti" {
			if command.Action == nil {
				t.Fatal("confetti command must be executable")
			}
			return
		}
	}

	t.Fatal("confetti command is missing")
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

func TestSetVolumeCommandAcceptsInlinePercentage(t *testing.T) {
	percent, valid := parseVolumePercent("set volume 30")
	if !valid || percent != 30 {
		t.Fatalf("parsed volume = %d, %t, want 30, true", percent, valid)
	}

	plugin := &SysPlugin{}
	matched, _ := plugin.commandMatches(SysCommand{ID: "set-volume"}, "set volume 30", nil, nil, false)
	fixedPresetMatched, _ := plugin.commandMatches(SysCommand{ID: "set-volume-25"}, "set volume 25", nil, nil, false)
	bareNumberMatched, _ := plugin.commandMatches(SysCommand{ID: "set-volume"}, "30", nil, nil, false)
	if !matched || fixedPresetMatched || bareNumberMatched {
		t.Fatalf("matches = %t, preset matches = %t, bare number matches = %t, want true, false, false", matched, fixedPresetMatched, bareNumberMatched)
	}

	command := SysCommand{ID: "set-volume", Title: "Set Volume"}
	incompleteAction := plugin.buildCommandAction(command, nil)
	completeAction := plugin.buildCommandAction(command, map[string]string{sysCommandVolumeContextKey: "30"})
	if !incompleteAction.PreventHideAfterAction || incompleteAction.Action == nil || completeAction.PreventHideAfterAction {
		t.Fatal("incomplete volume commands must continue query input; complete commands must execute normally")
	}
}

func TestStructuredVolumeUsesArgumentAndValidatesRange(t *testing.T) {
	for _, value := range []string{"", "-1", "101", "1.5", "abc", "0", "50", "100", "50%"} {
		structure := &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: common.QueryElementArgument, Value: value}}}
		query := plugin.Query{Search: "set volume 75", QueryHint: structure}
		got := buildSetVolumeContextData(query)
		_, valid := parseVolumePercent(value)
		if valid != (got[sysCommandVolumeContextKey] != "") {
			t.Fatalf("value %q context = %v", value, got)
		}
	}
}

func TestStructuredVolumeOnlyExecutesValidAction(t *testing.T) {
	executed := false
	p := &SysPlugin{commands: []SysCommand{{ID: "set-volume", Title: "Volume", BuildContextData: buildSetVolumeContextData, Action: func(context.Context, plugin.ActionContext) { executed = true }}}}
	for _, value := range []string{"", "101", "50"} {
		q := plugin.Query{QueryHint: &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: "argument", Value: value}}}}
		response := p.Query(context.Background(), q)
		if executed {
			t.Fatal("query executed a volume action")
		}
		if len(response.Results) != 1 {
			t.Fatal("expected volume result only")
		}
		if value != "50" && len(response.Results[0].Actions) != 0 {
			t.Fatal("invalid value exposes an action")
		}
		if value == "50" {
			response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
			if !executed {
				t.Fatal("valid action missing")
			}
		}
	}
}
