package ai

import (
	"context"
	"encoding/json"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"wox/common"
)

func TestPluginToolAINameStableAndBounded(t *testing.T) {
	pattern := regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	seen := map[string]bool{}
	for _, pair := range [][2]string{
		{"aaaaaaaa-1111", "create_note"},
		{"aaaaaaaa-2222", "create_note"},
		{"aaaaaaaa-1111", strings.Repeat("a", 63) + "b"},
		{"aaaaaaaa-1111", strings.Repeat("a", 63) + "c"},
	} {
		name := PluginToolAIName(pair[0], pair[1])
		if !pattern.MatchString(name) || seen[name] {
			t.Fatalf("invalid or duplicate name: %q", name)
		}
		seen[name] = true
		if got := PluginToolAIName(strings.ToUpper(pair[0]), pair[1]); got != name {
			t.Fatalf("plugin ID casing changed name: %q != %q", got, name)
		}
	}
}

func TestPluginToolSchemaPreservedByProviders(t *testing.T) {
	var schema map[string]any
	if err := json.Unmarshal([]byte(`{
		"type":"object",
		"$defs":{"count":{"type":"integer","enum":[1,2]}},
		"properties":{"count":{"$ref":"#/$defs/count"},"label":{"anyOf":[{"type":"string","minLength":1},{"type":"null"}]}},
		"required":["count"],"additionalProperties":false
	}`), &schema); err != nil {
		t.Fatal(err)
	}
	tool := common.Tool{Name: PluginToolAIName("notes", "create_note"), InputSchema: schema, Source: common.ToolSourcePlugin}
	converted := (&OpenAIBaseProvider{}).convertTools([]common.Tool{tool})
	data, err := json.Marshal(converted[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	parameters := payload["function"].(map[string]any)["parameters"]
	if !reflect.DeepEqual(parameters, schema) {
		t.Fatalf("provider changed schema: %s", data)
	}

	executed := 0
	bridge := &installedToolBridge{
		ctx: context.Background(), tools: []common.Tool{tool},
		options: common.ChatOptions{ExecuteTool: func(_ context.Context, option common.AgentToolExecutionOption) common.AgentToolExecutionResult {
			executed++
			return common.AgentToolExecutionResult{Call: option.Call}
		}},
	}
	for _, args := range []map[string]any{{"count": 3}, {"count": 1, "label": ""}, {"count": 1, "extra": true}} {
		if _, _, err := bridge.call(t.Context(), nil, installedToolCall{Name: tool.Name, Arguments: args}); err == nil {
			t.Fatalf("CLI bridge accepted invalid arguments: %v", args)
		}
	}
	if executed != 0 {
		t.Fatal("invalid arguments reached executor")
	}
	if _, _, err := bridge.call(t.Context(), nil, installedToolCall{Name: tool.Name, Arguments: map[string]any{"count": 1, "label": nil}}); err != nil || executed != 1 {
		t.Fatalf("valid arguments rejected: calls=%d err=%v", executed, err)
	}
}
