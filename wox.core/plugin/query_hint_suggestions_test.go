package plugin

import (
	"encoding/json"
	"reflect"
	"testing"
	"wox/common"
)

// Automatic hints use current declarations without taking over explicitly owned inputs.
func TestAutomaticCommandSuggestions(t *testing.T) {
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"gh", "*"}, Commands: []MetadataCommand{
		{Command: "issues", Aliases: []string{"issue"}}, {Command: "prs"}, {Command: "ISSUES"}, {Command: " "},
	}}}
	instances := []*Instance{instance}
	hint, owner := MatchQueryHint("gh ", instances)
	if hint == nil || owner != instance || !reflect.DeepEqual(hint.Elements[1].Suggestions, []string{"issues", "prs"}) || hint.PlainText() != "gh " {
		t.Fatalf("unexpected automatic hint: %+v", hint)
	}
	instance.RuntimeQueryCommands = []MetadataCommand{{Command: "search"}}
	updated, _ := MatchQueryHint("gh ", instances)
	if len(updated.Elements[1].Suggestions) != 3 || len(hint.Elements[1].Suggestions) != 2 {
		t.Fatal("runtime commands or isolation lost")
	}
	for _, text := range []string{"", " ", "gh", "gh i", "unknown "} {
		if got, _ := MatchQueryHint(text, instances); got != nil {
			t.Fatalf("unexpected hint for %q", text)
		}
	}
	if got, _ := MatchQueryHint("gh ", append(instances, &Instance{Metadata: Metadata{TriggerKeywords: []string{"gh"}}})); got != nil {
		t.Fatal("ambiguous trigger decorated")
	}
	explicit := &common.QueryHint{Elements: []common.QueryElement{{Id: "filter", Kind: common.QueryElementArgument, Placeholder: "filter"}}}
	// Exercise the same feature declaration a third-party plugin loads from plugin.json.
	if err := json.Unmarshal([]byte(`{"Features":[{"Name":"disableAutoCommandHint"}]}`), &instance.Metadata); err != nil {
		t.Fatal(err)
	}
	if got, _ := MatchQueryHint("gh ", instances); got != nil {
		t.Fatal("disabled automatic command hints still appeared")
	}
	instance.Metadata.TriggerQueryHints = map[string]*common.QueryHint{"gh": explicit}
	got, _ := MatchQueryHint("gh ", instances)
	if got.Elements[1].Placeholder != "filter" || len(got.Elements[1].Suggestions) != 0 {
		t.Fatal("explicit hint lost precedence")
	}
	instance.Metadata.Commands[0].QueryHint = explicit
	for _, text := range []string{"gh issues ", "gh issue "} {
		got, _ := MatchQueryHint(text, instances)
		if got == nil || got.Elements[0].Text != text || got.Elements[1].Id != "filter" {
			t.Fatalf("command/alias hint lost: %q", text)
		}
	}
}
