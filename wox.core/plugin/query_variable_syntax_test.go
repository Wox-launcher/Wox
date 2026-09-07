package plugin

import (
	"maps"
	"testing"
)

func TestParseAndFormatQueryVariable(t *testing.T) {
	selected, ok := ParseQueryVariable(QueryVariableSelectedText)
	if !ok || selected.Name != "selected_text" || len(selected.Params) != 0 {
		t.Fatalf("selected text = %+v ok=%v", selected, ok)
	}
	if FormatQueryVariable("selected_text", nil) != QueryVariableSelectedText {
		t.Fatal("selected text format drifted")
	}
	parameter, ok := ParseQueryVariable("{wox:parameter?name=query 123&case=lower}")
	if !ok || parameter.Name != "parameter" || !maps.Equal(parameter.Params, map[string]string{"name": "query 123", "case": "lower"}) {
		t.Fatalf("parameter = %+v ok=%v", parameter, ok)
	}
	if got := FormatQueryVariable("parameter", map[string]string{"case": "lower", "name": "query 123"}); got != "{wox:parameter?name=query 123&case=lower}" {
		t.Fatalf("parameter format = %s", got)
	}
	if ParameterQueryVariable("query") != "{wox:parameter?name=query}" {
		t.Fatal("canonical parameter token drifted")
	}
	for _, token := range []string{"{query}", "{lower_query}", "{upper_query}", "{wox:parameter:query}", "{wox:parameter?}", "{wox:parameter?name}", "{wox:1name}", "query"} {
		if _, ok := ParseQueryVariable(token); ok {
			t.Fatalf("accepted %s", token)
		}
	}
}
