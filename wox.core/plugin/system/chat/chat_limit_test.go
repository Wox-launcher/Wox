package chat

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"sync/atomic"
	"testing"
	"time"
	"wox/ai"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting"
	"wox/util"
)

type chatLimitTestAPI struct {
	plugin.API
	notices chan string
}

func (a chatLimitTestAPI) ShowToastMsg(_ context.Context, option plugin.ShowToastMsgOption) plugin.ShowToastMsgResult {
	a.notices <- option.Message
	return plugin.ShowToastMsgResult{Success: true}
}

type chatLimitTestUI struct {
	common.UI
	terminal chan common.AIChatData
}

func (u chatLimitTestUI) SendChatResponse(_ context.Context, chat common.AIChatData) {
	if !chat.IsStreaming {
		u.terminal <- chat
	}
}

// TestChatRoundLimit checks the setting and terminal behavior through the actual chat loop.
func TestChatRoundLimit(t *testing.T) {
	const childEnv = "WOX_CHAT_ROUND_LIMIT_TEST_CHILD"
	if os.Getenv(childEnv) == "" {
		t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
		t.Setenv(util.TestUserDataDirEnv, t.TempDir())
		t.Setenv(childEnv, "1")
		cmd := exec.Command(os.Args[0], "-test.run=^TestChatRoundLimit$")
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("chat round limit: %v\n%s", err, output)
		}
		return
	}

	ctx := context.Background()
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(ctx); err != nil {
		t.Fatal(err)
	}
	const toolName = "chat_limit_test"
	const availableToolRounds = 101
	registry := ai.GetToolRegistry()
	registry.Register(common.Tool{
		Name: toolName, Source: common.ToolSourceBuiltin,
		InputSchema: map[string]any{"type": "object"},
		Callback: func(context.Context, map[string]any) (common.ToolResult, error) {
			return common.ToolResult{Text: "completed tool output"}, nil
		},
	})
	defer registry.Unregister(toolName)

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		round := requests.Add(1)
		w.Header().Set("Content-Type", "text/event-stream")
		if round > availableToolRounds {
			fmt.Fprint(w, "data: "+`{"id":"final","object":"chat.completion.chunk","choices":[{"index":0,"delta":{"role":"assistant","content":"Task complete."},"finish_reason":"stop"}]}`+"\n\ndata: [DONE]\n\n")
			return
		}
		fmt.Fprintf(w, "data: "+`{"id":"reply-%d","object":"chat.completion.chunk","choices":[{"index":0,"delta":{`+
			`"role":"assistant","content":"progress %d","reasoning_content":"reasoning %d",`+
			`"tool_calls":[{"index":0,"id":"tool-%d","type":"function","function":{"name":%q,"arguments":"{}"}}]`+
			`},"finish_reason":"tool_calls"}]}`+"\n\ndata: [DONE]\n\n", round, round, round, round, toolName)
	}))
	defer server.Close()
	if err := setting.GetSettingManager().GetWoxSetting(ctx).AIProviders.Set([]setting.AIProvider{{
		Name: "deepseek", ApiKey: "local-test", Host: server.URL,
	}}); err != nil {
		t.Fatal(err)
	}

	r := newChatStorageTestPlugin(t)
	metadata := r.GetMetadata()
	defaultValue, ok := metadata.SettingDefinitions.GetDefaultValue(aiChatMaxToolCallRoundsSettingKey)
	if !ok || defaultValue != "100" {
		t.Fatalf("metadata round limit = %q, want 100", defaultValue)
	}
	// Omit metadata defaults in the instance to exercise runtime fallback for an absent setting.
	instance := &plugin.Instance{Plugin: r, Metadata: metadata, Setting: setting.NewPluginSetting(r.store, nil)}
	notices := make(chan string, 1)
	r.api = chatLimitTestAPI{API: plugin.NewAPI(instance), notices: notices}
	terminal := make(chan common.AIChatData, 1)
	manager := plugin.GetPluginManager()
	manager.AttachUI(chatLimitTestUI{terminal: terminal})
	defer manager.AttachUI(nil)

	for _, value := range []string{"", "0", "-2", "1.5", "9999999999999999999999"} {
		if handled, err := r.HandleSettingUpdate(ctx, aiChatMaxToolCallRoundsSettingKey, value); !handled || err == nil {
			t.Fatalf("accepted invalid round limit %q", value)
		}
	}

	for _, tc := range []struct {
		id         string
		value      string
		toolRounds int
		finishes   bool
	}{
		{id: "default", toolRounds: 100},
		{id: "custom", value: "2", toolRounds: 2},
		{id: "unlimited", value: "-1", toolRounds: availableToolRounds, finishes: true},
	} {
		t.Run(tc.id, func(t *testing.T) {
			requests.Store(0)
			if tc.value != "" {
				if handled, err := r.HandleSettingUpdate(ctx, aiChatMaxToolCallRoundsSettingKey, tc.value); handled || err != nil {
					t.Fatalf("valid setting must reach the usual save: %t, %v", handled, err)
				}
				if saved := r.api.SetSetting(ctx, plugin.SetSettingOption{Key: aiChatMaxToolCallRoundsSettingKey, Value: tc.value}); !saved.Success {
					t.Fatal(saved.ErrMsg)
				}
			}
			r.Chat(ctx, common.AIChatData{
				Id: tc.id, Title: "Chat round limit",
				Model:         common.Model{Provider: "deepseek", Name: "deepseek-flash"},
				Conversations: []common.Conversation{{Id: "user", Role: common.ConversationRoleUser, Text: "Call a test tool."}},
			}, 0)
			var snapshot common.AIChatData
			select {
			case snapshot = <-terminal:
			case <-time.After(15 * time.Second):
				t.Fatalf("no terminal chat snapshot after %d requests", requests.Load())
			}
			wantRequests := tc.toolRounds
			if tc.finishes {
				wantRequests++
			} else {
				select {
				case notice := <-notices:
					if want := fmt.Sprintf("Reached the tool-call round limit (%d). Send a follow-up to continue.", tc.toolRounds); notice != want {
						t.Fatalf("notice = %q, want %q", notice, want)
					}
				case <-time.After(5 * time.Second):
					t.Fatal("no limit notification")
				}
			}
			if count := int(requests.Load()); count != wantRequests {
				t.Fatalf("model requests = %d, want %d", count, wantRequests)
			}
			stored, ok := r.loadChat(ctx, tc.id)
			if !ok || stored.IsStreaming || snapshot.IsStreaming {
				t.Fatal("terminal chat was not saved and stopped")
			}
			for _, chat := range []common.AIChatData{snapshot, stored} {
				toolIndex := tc.toolRounds * 2
				wantMessages := toolIndex + 1
				if tc.finishes {
					wantMessages++
				}
				if len(chat.Conversations) != wantMessages {
					t.Fatalf("conversations = %d, want %d", len(chat.Conversations), wantMessages)
				}
				if tc.finishes && chat.Conversations[toolIndex+1].Text != "Task complete." {
					t.Fatalf("final response = %+v", chat.Conversations[toolIndex+1])
				}
				response, tool := chat.Conversations[toolIndex-1], chat.Conversations[toolIndex]
				if response.Role != common.ConversationRoleAssistant || response.Text != fmt.Sprint("progress ", tc.toolRounds) ||
					response.Reasoning != fmt.Sprint("reasoning ", tc.toolRounds) {
					t.Fatalf("last assistant response = %+v", response)
				}
				if tool.Role != common.ConversationRoleTool || tool.ToolCallInfo.Status != common.ToolCallStatusSucceeded ||
					tool.ToolCallInfo.Response != "completed tool output" {
					t.Fatalf("last tool result = %+v", tool)
				}
			}
		})
	}
}
