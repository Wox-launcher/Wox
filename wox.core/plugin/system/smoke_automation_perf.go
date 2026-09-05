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
	preview := chatFixturePreview(true)
	api := p.api
	go func() {
		for step := 1; step <= smokeAutomationChatStreamCount; step++ {
			time.Sleep(20 * time.Millisecond)
			title := fmt.Sprintf("Perf chat stream %d", step)
			// Title-only updates keep the 200-message preview retained; replacing
			// PreviewData on every tick rebuilds the whole conversation on the UI thread.
			api.UpdateResult(context.Background(), plugin.UpdatableResult{Id: resultID, Title: &title})
		}
		// Hidden result titles are not a completion signal in fullscreen chat.
		// Publish the final preview once so the Stop control returns to Send;
		// serial updates ensure no older timer can arrive after completion.
		completed := plugin.WoxPreview{PreviewType: plugin.WoxPreviewTypeChat, PreviewData: chatFixturePreview(false), ScrollPosition: plugin.WoxPreviewScrollPositionBottom}
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

// chatFixturePreview keeps message content fixed across the streaming completion transition.
func chatFixturePreview(streaming bool) string {
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
		}
		conversations = append(conversations, common.Conversation{
			Id: fmt.Sprintf("perf-msg-%d", index), Role: role, Text: text, Timestamp: int64(index),
		})
	}
	raw, err := json.Marshal(common.AIChatPreviewData{
		ActiveChat: common.AIChatData{Id: "perf-chat", Title: "Perf chat", Conversations: conversations, IsStreaming: streaming},
	})
	if err != nil {
		return "{}"
	}
	return string(raw)
}
