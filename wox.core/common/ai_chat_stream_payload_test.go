package common

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPluginAIChatStreamPayload(t *testing.T) {
	var model Model
	if err := json.Unmarshal([]byte(`{"Name":"deepseek-chat","Provider":"deepseek","ProviderAlias":"work"}`), &model); err != nil {
		t.Fatal(err)
	}
	if model.Name != "deepseek-chat" || model.Provider != "deepseek" || model.ProviderAlias != "work" {
		t.Fatalf("model = %#v", model)
	}

	var conversations []Conversation
	conversationsJSON := `[{"Role":"user","Text":"Hello","Timestamp":1,"Images":[{"ImageType":"base64","ImageData":"data:image/png;base64,cG5n"}]}]`
	if err := json.Unmarshal([]byte(conversationsJSON), &conversations); err != nil {
		t.Fatal(err)
	}
	if len(conversations) != 1 || conversations[0].Role != ConversationRoleUser || conversations[0].Text != "Hello" || conversations[0].Timestamp != 1 {
		t.Fatalf("conversations = %#v", conversations)
	}
	if len(conversations[0].Images) != 1 || conversations[0].Images[0].ImageType != WoxImageTypeBase64 {
		t.Fatalf("image = %#v", conversations[0].Images)
	}

	var options ChatOptions
	optionsJSON := `{"ThinkingMode":"thinking","LoopPolicy":{"MaxIterations":-1,"RetryOnFailure":true,"Timeout":1500000000}}`
	if err := json.Unmarshal([]byte(optionsJSON), &options); err != nil {
		t.Fatal(err)
	}
	if options.ThinkingMode != ChatThinkingModeThinking {
		t.Fatalf("thinking mode = %s", options.ThinkingMode)
	}
	if options.LoopPolicy.MaxIterations != -1 || !options.LoopPolicy.RetryOnFailure || options.LoopPolicy.Timeout != 1500*time.Millisecond {
		t.Fatalf("loop policy = %#v", options.LoopPolicy)
	}

	var empty ChatOptions
	if err := json.Unmarshal([]byte(`{}`), &empty); err != nil {
		t.Fatal(err)
	}
	if empty.ThinkingMode != "" || empty.LoopPolicy != (LoopPolicy{}) {
		t.Fatalf("empty options = %#v", empty)
	}
}
