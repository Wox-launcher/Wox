package plugin

import (
	"encoding/json"
	"strings"
	"testing"
	"wox/common"
)

// TestTriggerQueryHints covers static templates, runtime updates, conflicts and isolated in-flight hints.
func TestTriggerQueryHints(t *testing.T) {
	static := &common.QueryHint{Elements: []common.QueryElement{{Id: "input", Kind: common.QueryElementArgument, Placeholder: "Static"}}}
	instance := &Instance{Metadata: Metadata{TriggerKeywords: []string{"g"}, TriggerQueryHints: map[string]*common.QueryHint{"g": static}}}
	manager := &Manager{instances: []*Instance{instance}}
	if hint, _ := MatchQueryHint("g", manager.instances); hint != nil {
		t.Fatal("bare trigger received a static hint before entering plugin scope")
	}
	hint, owner := MatchQueryHint("g ", manager.instances)
	if hint == nil || owner != instance || hint.Elements[1].Placeholder != "Static" {
		t.Fatal("static trigger hint not matched")
	}
	global := &Instance{Metadata: Metadata{TriggerKeywords: []string{"*"}, Commands: []MetadataCommand{{Command: "g", QueryHint: static}}}}
	if _, owner := MatchQueryHint("g ", []*Instance{global, instance}); owner != instance {
		t.Fatal("global command decorated a scoped trigger owned by another plugin")
	}
	runtime := static.Clone()
	runtime.Elements[0].Placeholder = "Runtime"
	if !manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "g", QueryHint: runtime}) {
		t.Fatal("runtime registration failed")
	}
	runtime.Elements[0].Placeholder = "mutated"
	if hint, _ := MatchQueryHint("g", manager.instances); hint != nil {
		t.Fatal("bare trigger received a runtime hint before entering plugin scope")
	}
	current, _ := MatchQueryHint("g ", manager.instances)
	if current.Elements[1].Placeholder != "Runtime" || hint.Elements[1].Placeholder != "Static" {
		t.Fatal("registered or active hint was mutated")
	}
	if manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "g", QueryHint: &common.QueryHint{}}) {
		t.Fatal("invalid update was accepted")
	}
	instance.Metadata.Commands = []MetadataCommand{{Command: "find", QueryHint: static}}
	if command, _ := MatchQueryHint("g find ", manager.instances); command == nil || command.Elements[0].Text != "g find " {
		t.Fatal("command hint lost precedence")
	}
	conflict := &Instance{Metadata: Metadata{TriggerKeywords: []string{"g"}}}
	if ambiguous, _ := MatchQueryHint("g ", []*Instance{instance, conflict}); ambiguous != nil {
		t.Fatal("conflicting keyword received a hint")
	}
	manager.registerTriggerKeyword(instance, RegisterTriggerKeywordOption{Keyword: "g"})
	if cleared, _ := MatchQueryHint("g ", manager.instances); cleared != nil {
		t.Fatal("nil update did not clear the runtime hint")
	}
	instance.unregisterTriggerKeyword("g")
	if restored, _ := MatchQueryHint("g ", manager.instances); restored == nil || restored.Elements[1].Placeholder != "Static" {
		t.Fatal("unregistration did not restore metadata hint")
	}
	for _, text := range []string{"g test", "gg "} {
		if got, _ := MatchQueryHint(text, manager.instances); got != nil {
			t.Fatalf("non-prefix query %q received hint", text)
		}
	}
}

func TestMatchQueryHint(t *testing.T) {
	template := &common.QueryHint{Elements: []common.QueryElement{{Id: "volume", Kind: "argument", Placeholder: "Volume"}}}
	instance := &Instance{Metadata: Metadata{Id: "sys", TriggerKeywords: []string{"*"}, Commands: []MetadataCommand{{Command: "set-volume", Aliases: []string{"set volume", "volume"}, QueryHint: template}}}}
	for _, text := range []string{"set-volume ", "set volume ", "volume ", "SET VOLUME "} {
		got, source := MatchQueryHint(text, []*Instance{instance})
		if got == nil || source != instance || len(got.Elements) != 2 {
			t.Fatalf("%q did not create slots", text)
		}
		got.Elements[1].Value = "50"
		if template.Argument("volume") != "" {
			t.Fatal("instance mutated declaration")
		}
	}
	for _, text := range []string{"set-volume", "set volume", "volume", "SET VOLUME", "set volum", "set volumex", "set volume 50"} {
		if hint, _ := MatchQueryHint(text, []*Instance{instance}); hint != nil {
			t.Fatalf("%q was parsed unexpectedly", text)
		}
	}
	if hint, _ := MatchQueryHint("volume ", []*Instance{instance, instance}); hint != nil {
		t.Fatal("ambiguous match selected an owner")
	}
	instance.Metadata.TriggerKeywords = []string{"gh"}
	instance.Metadata.Commands[0].Command = "issues"
	if hint, _ := MatchQueryHint("gh issues", []*Instance{instance}); hint != nil {
		t.Fatal("scoped command received a hint before its context separator")
	}
	if hint, _ := MatchQueryHint("gh issues ", []*Instance{instance}); hint == nil {
		t.Fatal("plugin trigger was not matched")
	}
}

// TestQueryHintDoesNotCarryRoutingIdentity covers old persisted payloads as well as new hints.
func TestQueryHintDoesNotCarryRoutingIdentity(t *testing.T) {
	var hint common.QueryHint
	if err := json.Unmarshal([]byte(`{"PluginId":"github","Elements":[{"Id":"command","Kind":"text","Text":"issues "},{"Id":"issue","Kind":"block","Value":"owner/repo#6"}]}`), &hint); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(hint.Clone())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(encoded), "PluginId") {
		t.Fatal("hint retained a routing identity")
	}
	github := &Instance{Metadata: Metadata{Id: "github", TriggerKeywords: []string{"gh"}, Commands: []MetadataCommand{{Command: "issues"}}}}
	query, owner := newQueryInputWithPlugins(hint.PlainText(), []*Instance{github})
	if owner != nil || !query.Scope.IsEmpty() {
		t.Fatal("hint without a trigger acquired a plugin scope")
	}
	hint.Elements[0].Text = "gh issues "
	query, owner = newQueryInputWithPlugins(hint.PlainText(), []*Instance{github})
	if owner != github || query.Command != "issues" || !query.Scope.IsEmpty() {
		t.Fatal("explicit trigger did not retain ordinary routing")
	}
}
