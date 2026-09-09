//go:build wox_automation

package system

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"wox/common"
	"wox/plugin"
)

const (
	smokeAutomationListCommand      = "list-500"
	smokeAutomationGridCommand      = "grid-500"
	smokeAutomationChatCommand      = "chat-200"
	smokeAutomationWarmCacheCommand = "warm-cache"
	smokeAutomationListCount        = 500
	smokeAutomationGridCount        = 500
	smokeAutomationChatCount        = 200
	smokeAutomationChatStreamCount  = 50
	smokeAutomationWarmCacheCount   = 8
)

func queryListFixture() plugin.QueryResponse {
	results := make([]plugin.QueryResult, 0, smokeAutomationListCount)
	for index := range smokeAutomationListCount {
		results = append(results, plugin.QueryResult{
			Id:       fmt.Sprintf("perf-list-%04d", index),
			Title:    fmt.Sprintf("Perf list result %04d", index),
			SubTitle: "Deterministic list fixture",
			Icon:     common.PluginAppIcon,
		})
	}
	return plugin.NewQueryResponse(results)
}

func queryGridFixture() plugin.QueryResponse {
	results := make([]plugin.QueryResult, 0, smokeAutomationGridCount)
	for index := range smokeAutomationGridCount {
		group := fmt.Sprintf("Group %02d", index/50)
		results = append(results, plugin.QueryResult{
			Id:         fmt.Sprintf("perf-grid-%04d", index),
			Title:      fmt.Sprintf("Grid %04d", index),
			SubTitle:   group,
			Icon:       common.PluginAppIcon,
			Group:      group,
			GroupScore: int64(1000 - index/50),
		})
	}
	return plugin.QueryResponse{
		Results: results,
		Layout: plugin.QueryLayout{
			GridLayout: &plugin.MetadataFeatureParamsGridLayout{
				Columns: 6, ShowTitle: true, ItemMargin: 6, AspectRatio: 1,
			},
		},
	}
}

// queryChatFixture publishes an observable streaming state and completes it after the last update.
func (p *smokeAutomationPlugin) queryChatFixture() plugin.QueryResponse {
	resultID := "perf-chat-result"
	preview := chatFixturePreview(true, 0)
	api := p.api
	go func() {
		for step := 1; step <= smokeAutomationChatStreamCount; step++ {
			time.Sleep(120 * time.Millisecond)
			// Grow the actual answer so adapter preparation is included in frame costs.
			preview := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeChat, PreviewData: chatFixturePreview(true, step), ScrollPosition: plugin.WoxPreviewScrollPositionBottom}
			api.UpdateResult(context.Background(), plugin.UpdatableResult{Id: resultID, Preview: &preview})
		}
		// Publish the final preview once so the Stop control returns to Send;
		// serial updates ensure no older timer can arrive after completion.
		completed := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeChat, PreviewData: chatFixturePreview(false, smokeAutomationChatStreamCount), ScrollPosition: plugin.WoxPreviewScrollPositionBottom}
		api.UpdateResult(context.Background(), plugin.UpdatableResult{Id: resultID, Preview: &completed})
	}()
	ratio := 0.0
	return plugin.QueryResponse{
		Results: []plugin.QueryResult{{
			Id: resultID, Title: "Perf chat stream 0", Icon: common.PluginAppIcon,
			Preview: plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeChat, PreviewData: preview, ScrollPosition: plugin.WoxPreviewScrollPositionBottom},
		}},
		Layout: plugin.QueryLayout{ChatMode: true, ResultPreviewWidthRatio: &ratio},
	}
}

func queryWarmCacheFixture() plugin.QueryResponse {
	icons := []common.WoxImage{common.PluginAppIcon, common.PluginCalculatorIcon}
	titles := []string{"Warm cache alpha", "Warm cache beta"}
	results := make([]plugin.QueryResult, 0, smokeAutomationWarmCacheCount)
	for index := range smokeAutomationWarmCacheCount {
		results = append(results, plugin.QueryResult{
			Id:       fmt.Sprintf("perf-warm-%d", index),
			Title:    titles[index%len(titles)],
			SubTitle: "Repeated text and image fixture",
			Icon:     icons[index%len(icons)],
		})
	}
	return plugin.NewQueryResponse(results)
}

// chatFixturePreview exercises distinct historical Markdown and a growing final answer.
func chatFixturePreview(streaming bool, step int) string {
	conversations := make([]common.Conversation, 0, smokeAutomationChatCount)
	for index := range smokeAutomationChatCount {
		role := common.ConversationRoleUser
		text := fmt.Sprintf("User message %d", index)
		if index%2 == 1 {
			role = common.ConversationRoleAssistant
			switch index % 6 {
			case 1:
				text = "Short reply."
			case 3:
				text = strings.Repeat("Medium reply line.\n", 3)
			default:
				text = strings.Repeat("Longer streaming-style paragraph for variable height. ", 8)
			}
			text = fmt.Sprintf("**Reply %d**\n\n%s", index, text)
			if index == smokeAutomationChatCount-1 {
				text += strings.Repeat(" More streamed text.", step)
			}
		}
		conversations = append(conversations, common.Conversation{
			Id: fmt.Sprintf("perf-msg-%d", index), Role: role, Text: text, Timestamp: int64(index),
		})
	}
	// End with tools so the round stays visible after streaming stops. Separate
	// summaries with reasoning, matching long agent runs with collapsed payloads.
	conversations = append(conversations, common.Conversation{Id: "perf-tool-request", Role: common.ConversationRoleUser, Text: "Inspect these sources."})
	for index := range 31 {
		id := fmt.Sprintf("perf-tool-%d", index)
		reasoning := strings.Repeat("Inspecting the next source. ", 20)
		if index == 30 {
			// A long active reasoning message uses plain text, bypassing Markdown caching.
			var lines strings.Builder
			for line := range 400 {
				fmt.Fprintf(&lines, "Reasoning line %d: inspect the weather code and return 晴天 ☀️ or 阴天 ☁️.\n", line)
			}
			reasoning += lines.String() + strings.Repeat(" More reasoning.", step)
		}
		conversations = append(conversations,
			common.Conversation{Id: id + "-reasoning", Role: common.ConversationRoleAssistant, Reasoning: reasoning},
			common.Conversation{Id: id, Role: common.ConversationRoleTool, ToolCallInfo: common.ToolCallInfo{
				Id: id, Name: "web_fetch", Source: common.ToolSourceBuiltin, Status: "succeeded",
				Arguments: map[string]any{"url": "https://example.com/fixture"},
				Response:  strings.Repeat("Deterministic hidden tool response. ", 200),
			}},
		)
	}
	raw, err := json.Marshal(common.AIChatPreviewData{
		ActiveChat: common.AIChatData{Id: "perf-chat", Title: "Perf chat", Conversations: conversations, IsStreaming: streaming},
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}
