package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"wox/common"
	"wox/network"
	"wox/setting"
	"wox/util"

	"github.com/tmc/langchaingo/jsonschema"
)

func TestConvertToolsOmitsNonObjectProperties(t *testing.T) {
	provider := &OpenAIBaseProvider{}
	tools := provider.convertTools([]common.Tool{{
		Name:        "bash",
		Description: "run a command",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"command": {Type: jsonschema.String, Description: "shell command"},
				"timeout": {Type: jsonschema.Integer, Description: "seconds"},
				"names": {
					Type:        jsonschema.Array,
					Description: "tool names",
					Items:       &jsonschema.Definition{Type: jsonschema.String},
				},
			},
			Required: []string{"command"},
		},
	}})
	if len(tools) != 1 {
		t.Fatalf("converted tools = %d, want 1", len(tools))
	}

	data, err := json.Marshal(tools[0])
	if err != nil {
		t.Fatal(err)
	}
	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatal(err)
	}
	function, _ := payload["function"].(map[string]any)
	parameters, _ := function["parameters"].(map[string]any)
	properties, _ := parameters["properties"].(map[string]any)
	command, _ := properties["command"].(map[string]any)
	timeout, _ := properties["timeout"].(map[string]any)
	names, _ := properties["names"].(map[string]any)
	if _, ok := command["properties"]; ok {
		t.Fatalf("string property must not include properties: %s", data)
	}
	if _, ok := timeout["properties"]; ok {
		t.Fatalf("integer property must not include properties: %s", data)
	}
	if command["type"] != "string" || timeout["type"] != "integer" {
		t.Fatalf("property types = %s", data)
	}
	items, _ := names["items"].(map[string]any)
	if names["type"] != "array" || items["type"] != "string" {
		t.Fatalf("array property = %s", data)
	}
	if _, ok := items["properties"]; ok {
		t.Fatalf("array item string must not include properties: %s", data)
	}
}

func TestDeepSeekConversationReplayPreservesReasoning(t *testing.T) {
	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "deepseek", Host: "https://api.deepseek.com"})
	messages, convertErr := provider.convertConversations([]common.Conversation{
		{Role: common.ConversationRoleUser, Text: "weather"},
		{Role: common.ConversationRoleAssistant, Text: "Let me check.", Reasoning: "I need the weather tool."},
		{Role: common.ConversationRoleTool, ToolCallInfo: common.ToolCallInfo{Id: "call-1", Name: "weather", Delta: `{}`, Response: "sunny"}},
		{Role: common.ConversationRoleAssistant, Text: "It is sunny.", Reasoning: "The tool returned sunny."},
	})

	if convertErr != nil {
		t.Fatal(convertErr)
	}
	if len(messages) != 4 {
		t.Fatalf("converted messages = %d, want 4", len(messages))
	}

	for index, expectedReasoning := range map[int]string{1: "I need the weather tool.", 3: "The tool returned sunny."} {
		data, err := json.Marshal(messages[index])
		if err != nil {
			t.Fatal(err)
		}
		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatal(err)
		}
		if message["reasoning_content"] != expectedReasoning {
			t.Fatalf("message %d reasoning_content = %v, want %q", index, message["reasoning_content"], expectedReasoning)
		}
	}

	data, err := json.Marshal(messages[1])
	if err != nil {
		t.Fatal(err)
	}
	var toolCallMessage map[string]any
	if err := json.Unmarshal(data, &toolCallMessage); err != nil {
		t.Fatal(err)
	}
	if toolCallMessage["content"] != "Let me check." {
		t.Fatalf("tool-call content = %v, want %q", toolCallMessage["content"], "Let me check.")
	}
	if len(toolCallMessage["tool_calls"].([]any)) != 1 {
		t.Fatalf("tool_calls = %v, want one call", toolCallMessage["tool_calls"])
	}
}

func TestConvertConversationsCombinesOnlyLeadingSystemMessages(t *testing.T) {
	provider := &OpenAIBaseProvider{}
	conversations := []common.Conversation{
		{Role: common.ConversationRoleSystem, Text: "time"},
		{Role: common.ConversationRoleSystem, Text: "project"},
		{Role: common.ConversationRoleSystem, Text: "skills"},
		{Role: common.ConversationRoleSystem, Text: "tools"},
		{Role: common.ConversationRoleUser, Text: "hello"},
		{Role: common.ConversationRoleAssistant, Text: "hi"},
		{Role: common.ConversationRoleSystem, Text: "later instruction"},
	}
	messages, err := provider.convertConversations(conversations)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	var got []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	want := []struct{ role, content string }{
		{"system", "time\n\nproject\n\nskills\n\ntools"},
		{"user", "hello"},
		{"assistant", "hi"},
		{"system", "later instruction"},
	}
	if len(got) != len(want) {
		t.Fatalf("messages = %s", data)
	}
	for i := range want {
		if got[i].Role != want[i].role || got[i].Content != want[i].content {
			t.Fatalf("message %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if conversations[0].Text != "time" || conversations[1].Text != "project" {
		t.Fatal("conversion mutated the original conversations")
	}
}

func TestChatImageAttachmentsReachProviderAsImageContent(t *testing.T) {
	directory := t.TempDir()
	previous := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(directory)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(previous) })
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 3, 2))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(directory, "image.png")
	if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	attachment, err := common.ImportChatAttachment(path)
	if err != nil {
		t.Fatal(err)
	}
	provider := &OpenAIBaseProvider{}
	messages, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Text: "Describe", Attachments: []common.AIChatAttachment{attachment}}})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"type":"image_url"`) || !strings.Contains(string(data), "data:image/png;base64,") || strings.Contains(string(data), "chat-attachment:") {
		t.Fatalf("provider payload = %s", data)
	}
	legacy, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Text: "Describe", Images: []common.WoxImage{common.NewWoxImageAbsolutePath(path)}}})
	if err != nil {
		t.Fatal(err)
	}
	legacyJSON, _ := json.Marshal(legacy)
	if !bytes.Equal(data, legacyJSON) {
		t.Fatal("legacy Images and attachments should use the same provider content")
	}
	if err := os.Remove(common.ChatAttachmentPath(attachment)); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.convertConversations([]common.Conversation{{Role: common.ConversationRoleUser, Attachments: []common.AIChatAttachment{attachment}}}); err == nil {
		t.Fatal("missing image must fail instead of silently sending only text")
	}
}

func TestNonDeepSeekToolCallAssistantMessageOmitsContent(t *testing.T) {
	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: "https://api.openai.com/v1"})
	messages, err := provider.convertConversations([]common.Conversation{
		{Role: common.ConversationRoleUser, Text: "weather"},
		{Role: common.ConversationRoleAssistant, Text: "checking", Reasoning: "hidden"},
		{Role: common.ConversationRoleTool, ToolCallInfo: common.ToolCallInfo{Id: "call-1", Name: "weather", Delta: `{}`, Response: "sunny"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(messages)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 4 {
		t.Fatalf("messages = %s, want 4", data)
	}
	if _, hasContent := decoded[2]["content"]; hasContent {
		t.Fatalf("non-DeepSeek tool_call assistant message should omit content: %s", data)
	}
	if _, hasReasoning := decoded[2]["reasoning_content"]; hasReasoning {
		t.Fatalf("non-DeepSeek tool_call assistant message should omit reasoning_content: %s", data)
	}
}

func TestOpenAIStreamTextReasoningThinkTagsCommentsAndLargeLine(t *testing.T) {
	largePadding := strings.Repeat("a", 70*1024)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Fatalf("path = %q, want /v1/chat/completions", r.URL.Path)
		}
		if gotUA := r.Header.Get("User-Agent"); gotUA != "Wox" {
			t.Fatalf("User-Agent = %q, want Wox", gotUA)
		}
		for headerName := range r.Header {
			if strings.HasPrefix(strings.ToLower(headerName), "x-stainless-") {
				t.Fatalf("unexpected SDK fingerprint header %q", headerName)
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		chunks := []string{
			": OPENROUTER PROCESSING\n\n",
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning\":\"step1 \"}}]}\n\n",
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"reasoning_content\":\"step2\"}}]}\n\n",
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"<thi\"}}]}\n\n",
			"data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"nk>inner thought</think>\\n\\n\"}}]}\n\n",
			fmt.Sprintf("data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":\"hello %s\"}}]}\n\n", largePadding),
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			_, _ = io.WriteString(w, c)
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))
	defer server.Close()

	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: server.URL + "/v1", ApiKey: "test-key"})
	stream, err := provider.ChatStream(context.Background(), common.Model{Name: "gpt-4o"}, []common.Conversation{
		{Role: common.ConversationRoleUser, Text: "hi"},
	}, common.EmptyChatOptions)
	if err != nil {
		t.Fatal(err)
	}

	var final common.ChatStreamData
	for {
		data, err := stream.Receive(context.Background())
		if errors.Is(err, ChatStreamNoContentErr) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if data.Status == common.ChatStreamStatusStreamed {
			final = data
			break
		}
	}

	wantReasoning := "step1 step2inner thought"
	if final.Reasoning != wantReasoning {
		t.Fatalf("Reasoning = %q, want %q", final.Reasoning, wantReasoning)
	}
	wantData := "hello " + largePadding
	if final.Data != wantData {
		t.Fatalf("Data len = %d, want %d", len(final.Data), len(wantData))
	}
}

func TestOpenAIStreamMultiToolCallsAndArgumentNormalization(t *testing.T) {
	toolName := "test_norm_tool"
	GetToolRegistry().Register(common.Tool{
		Name:   toolName,
		Source: common.ToolSourceBuiltin,
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"sequence_number": {Type: jsonschema.Integer},
				"label":           {Type: jsonschema.String},
			},
			Required: []string{"sequence_number", "label"},
		},
	})
	t.Cleanup(func() { GetToolRegistry().Unregister(toolName) })

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var reqMap map[string]any
		if err := json.Unmarshal(body, &reqMap); err != nil {
			t.Fatal(err)
		}
		if reqMap["tool_choice"] != "auto" {
			t.Fatalf("tool_choice = %v, want auto", reqMap["tool_choice"])
		}
		w.Header().Set("Content-Type", "text/event-stream")
		chunks := []string{
			fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":%q,"arguments":"{\"sequence"}}]}}]}`, toolName) + "\n\n",
			`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"Number\":42}"}}]}}]}` + "\n\n",
			fmt.Sprintf(`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_2","type":"function","function":{"name":%q,"arguments":"{\"sequence_number\":7,\"label\":\"ok\"}"}}]}}]}`+"\n\n", toolName),
			"data: [DONE]\n\n",
		}
		for _, c := range chunks {
			_, _ = io.WriteString(w, c)
		}
	}))
	defer server.Close()

	provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: server.URL + "/v1", ApiKey: "k"})
	tool, _ := GetToolRegistry().Get(toolName)
	stream, err := provider.ChatStream(context.Background(), common.Model{Name: "gpt-4o"}, []common.Conversation{
		{Role: common.ConversationRoleUser, Text: "run"},
	}, common.ChatOptions{Tools: []common.Tool{tool}})
	if err != nil {
		t.Fatal(err)
	}

	var streamingToolSnapshot []common.ToolCallInfo
	var final common.ChatStreamData
	for {
		data, err := stream.Receive(context.Background())
		if errors.Is(err, ChatStreamNoContentErr) {
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if data.Status == common.ChatStreamStatusStreaming && len(data.ToolCalls) == 2 {
			streamingToolSnapshot = data.ToolCalls
		}
		if data.Status == common.ChatStreamStatusStreamed {
			final = data
			break
		}
	}

	if len(streamingToolSnapshot) != 2 || streamingToolSnapshot[0].Status != common.ToolCallStatusPending || streamingToolSnapshot[1].Status != common.ToolCallStatusStreaming {
		t.Fatalf("streaming tool statuses = %+v", streamingToolSnapshot)
	}
	if len(final.ToolCalls) != 2 {
		t.Fatalf("final ToolCalls = %d, want 2", len(final.ToolCalls))
	}
	if final.ToolCalls[0].Id != "call_1" || final.ToolCalls[0].Arguments["sequence_number"] != int64(42) || final.ToolCalls[0].Arguments["label"] != "" {
		t.Fatalf("normalized call 0 = %+v", final.ToolCalls[0])
	}
	if final.ToolCalls[1].Id != "call_2" || final.ToolCalls[1].Arguments["sequence_number"] != int64(7) || final.ToolCalls[1].Arguments["label"] != "ok" {
		t.Fatalf("normalized call 1 = %+v", final.ToolCalls[1])
	}
}

func TestOpenAIErrorsAndRetryPolicy(t *testing.T) {
	t.Run("siliconflow numeric code body preserved", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"code":20015,"message":"model does not support parameter"}`)
		}))
		defer server.Close()

		provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "siliconflow", Host: server.URL + "/v1", ApiKey: "k"})
		_, err := provider.ChatStream(context.Background(), common.Model{Name: "m"}, []common.Conversation{{Role: common.ConversationRoleUser, Text: "hi"}}, common.EmptyChatOptions)
		if err == nil {
			t.Fatal("expected error")
		}
		var apiErr *ProviderAPIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 400 {
			t.Fatalf("unexpected error = %v", err)
		}
		if !strings.Contains(err.Error(), "20015") || !strings.Contains(err.Error(), "400") {
			t.Fatalf("error missing status or body: %s", err.Error())
		}
	})

	t.Run("401 does not retry and returns ProviderAPIError", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"unauthorized"}`)
		}))
		defer server.Close()

		provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: server.URL + "/v1", ApiKey: "bad"})
		err := provider.Ping(context.Background())
		var apiErr *ProviderAPIError
		if !errors.As(err, &apiErr) || apiErr.StatusCode != 401 {
			t.Fatalf("err = %v, want 401 ProviderAPIError", err)
		}
		if attempts.Load() != 1 {
			t.Fatalf("attempts = %d, want 1", attempts.Load())
		}
	})

	t.Run("429 retries online and does not retry offline", func(t *testing.T) {
		var attempts atomic.Int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if attempts.Add(1) == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"m1"}]}`)
		}))
		defer server.Close()

		provider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: server.URL + "/v1", ApiKey: "k"})
		models, err := provider.Models(context.Background())
		if err != nil {
			t.Fatalf("expected retry to succeed, got %v", err)
		}
		if len(models) != 1 || models[0].Name != "m1" || attempts.Load() != 2 {
			t.Fatalf("models=%+v attempts=%d", models, attempts.Load())
		}

		wasOffline := network.Default.IsOffline()
		network.Default.SetOffline(true)
		t.Cleanup(func() { network.Default.SetOffline(wasOffline) })

		attempts.Store(0)
		offlineServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts.Add(1)
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer offlineServer.Close()

		offlineProvider := NewOpenAIBaseProvider(setting.AIProvider{Name: "openai", Host: offlineServer.URL + "/v1", ApiKey: "k"})
		if _, err := offlineProvider.Models(context.Background()); err == nil {
			t.Fatal("expected offline 429 to fail without retrying")
		}
		if attempts.Load() != 1 {
			t.Fatalf("offline attempts = %d, want 1", attempts.Load())
		}
	})
}

func TestOpenAICompatibleProviderSpecificOptionsAndURLJoining(t *testing.T) {
	t.Run("google trailing slash host", func(t *testing.T) {
		p := NewGoogleProvider(context.Background(), setting.AIProvider{Name: "google", Host: "https://generativelanguage.googleapis.com/v1beta/openai/"}).(*GoogleProvider)
		got, err := p.buildEndpointURL("chat/completions", nil)
		if err != nil {
			t.Fatal(err)
		}
		want := "https://generativelanguage.googleapis.com/v1beta/openai/chat/completions"
		if got != want {
			t.Fatalf("url = %q, want %q", got, want)
		}
	})

	t.Run("deepseek thinking extra body", func(t *testing.T) {
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_ = json.NewDecoder(r.Body).Decode(&captured)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = io.WriteString(w, "data: [DONE]\n\n")
		}))
		defer server.Close()

		p := NewDeepSeekProvider(context.Background(), setting.AIProvider{Name: "deepseek", Host: server.URL, ApiKey: "k"})
		stream, err := p.ChatStream(context.Background(), common.Model{Name: "deepseek-chat"}, []common.Conversation{{Role: common.ConversationRoleUser, Text: "hi"}}, common.ChatOptions{ThinkingMode: common.ChatThinkingModeThinking})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = stream.Receive(context.Background())
		thinking, _ := captured["thinking"].(map[string]any)
		if thinking["type"] != "enabled" {
			t.Fatalf("thinking payload = %v", captured["thinking"])
		}
	})

	t.Run("openrouter headers and siliconflow sub_type + enable_thinking", func(t *testing.T) {
		var sawReferer, sawTitle, sawSubType string
		var captured map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/models":
				sawSubType = r.URL.Query().Get("sub_type")
				sawReferer = r.Header.Get("HTTP-Referer")
				sawTitle = r.Header.Get("X-Title")
				w.Header().Set("Content-Type", "application/json")
				_, _ = io.WriteString(w, `{"data":[{"id":"qwen"}]}`)
			case "/chat/completions":
				_ = json.NewDecoder(r.Body).Decode(&captured)
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
			}
		}))
		defer server.Close()

		or := NewOpenRouterProvider(context.Background(), setting.AIProvider{Name: "openrouter", Host: server.URL, ApiKey: "k"})
		if _, err := or.Models(context.Background()); err != nil {
			t.Fatal(err)
		}
		if sawReferer != "https://github.com/Wox-launcher/Wox" || sawTitle != "Wox" {
			t.Fatalf("openrouter headers referer=%q title=%q", sawReferer, sawTitle)
		}

		sf := NewSiliconFlowProvider(context.Background(), setting.AIProvider{Name: "siliconflow", Host: server.URL, ApiKey: "k"})
		if _, err := sf.Models(context.Background()); err != nil {
			t.Fatal(err)
		}
		if sawSubType != "chat" {
			t.Fatalf("siliconflow sub_type = %q, want chat", sawSubType)
		}
		stream, err := sf.ChatStream(context.Background(), common.Model{Name: "Qwen/Qwen3-8B"}, []common.Conversation{{Role: common.ConversationRoleUser, Text: "hi"}}, common.ChatOptions{ThinkingMode: common.ChatThinkingModeNonThinking})
		if err != nil {
			t.Fatal(err)
		}
		_, _ = stream.Receive(context.Background())
		if captured["enable_thinking"] != false {
			t.Fatalf("enable_thinking = %v, want false", captured["enable_thinking"])
		}
	})

	t.Run("minimax ping and reasoning_split", func(t *testing.T) {
		var pingPayload, chatPayload map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["stream"] == true {
				chatPayload = body
				w.Header().Set("Content-Type", "text/event-stream")
				_, _ = io.WriteString(w, "data: [DONE]\n\n")
				return
			}
			pingPayload = body
			w.Header().Set("Content-Type", "application/json")
			_, _ = io.WriteString(w, `{"id":"ok","choices":[]}`)
		}))
		defer server.Close()

		mm := NewMiniMaxProvider(context.Background(), setting.AIProvider{Name: "minimax", Host: server.URL, ApiKey: "k"})
		if err := mm.Ping(context.Background()); err != nil {
			t.Fatal(err)
		}
		if pingPayload["model"] != miniMaxModels[0] || pingPayload["max_completion_tokens"] != float64(1) {
			t.Fatalf("minimax ping payload = %+v", pingPayload)
		}
		stream, err := mm.ChatStream(context.Background(), common.Model{Name: miniMaxModels[0]}, []common.Conversation{{Role: common.ConversationRoleUser, Text: "hi"}}, common.EmptyChatOptions)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = stream.Receive(context.Background())
		if chatPayload["reasoning_split"] != true {
			t.Fatalf("minimax reasoning_split = %v, want true", chatPayload["reasoning_split"])
		}
	})
}
