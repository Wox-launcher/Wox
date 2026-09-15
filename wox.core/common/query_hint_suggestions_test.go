package common

import (
	"encoding/json"
	"reflect"
	"testing"
)

// Suggestions decorate values and must not alias shared declarations or snapshots.
func TestQueryHintSuggestionsRoundTrip(t *testing.T) {
	hint := &QueryHint{Elements: []QueryElement{{Id: "filter", Kind: QueryElementArgument, Value: "cr", Suggestions: []string{"created", "assigned"}}}}
	if err := hint.Validate(); err != nil {
		t.Fatal(err)
	}
	clone := hint.Clone()
	clone.Elements[0].Suggestions[0] = "changed"
	if hint.Elements[0].Suggestions[0] != "created" {
		t.Fatal("clone shares suggestions")
	}
	hint.CommandSuggestions = true
	if !hint.Clone().CommandSuggestions {
		t.Fatal("clone lost the core command marker")
	}
	data, err := json.Marshal(hint)
	if err != nil {
		t.Fatal(err)
	}
	var restored QueryHint
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.CommandSuggestions {
		t.Fatal("core command marker leaked into plugin JSON")
	}
	hint.CommandSuggestions = false
	if !reflect.DeepEqual(hint, &restored) || restored.PlainText() != "cr" || restored.Argument("filter") != "cr" {
		t.Fatal("round trip changed query content")
	}
	for _, kind := range []string{QueryElementText, QueryElementBlock} {
		invalid := &QueryHint{Elements: []QueryElement{{Id: "invalid", Kind: kind, Suggestions: []string{"created"}}}}
		if invalid.Validate() == nil {
			t.Fatalf("suggestions accepted for %s", kind)
		}
	}
}
