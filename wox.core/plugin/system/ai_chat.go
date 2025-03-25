package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"wox/common"
	"wox/plugin"
)

var aiChatIcon = plugin.PluginAIChatIcon
var aiChatsSettingKey = "ai_chats"

type AIChatData struct {
	Id            string
	Title         string
	Conversations []common.Conversation
	Model         common.Model

	CreatedAt int64
	UpdatedAt int64
}

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &AIChatPlugin{})
}

type AIChatPlugin struct {
	chats []AIChatData
	api   plugin.API
}

func (r *AIChatPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:              "a9cfd85a-6e53-415c-9d44-68777aa6323d",
		Name:            "Wox AI Chat",
		Author:          "Wox Launcher",
		Website:         "https://github.com/Wox-launcher/Wox",
		Version:         "1.0.0",
		MinWoxVersion:   "2.0.0",
		Runtime:         "Go",
		Description:     "Chat with AI",
		Icon:            aiChatIcon.String(),
		TriggerKeywords: []string{"chat"},
		SupportedOS:     []string{"Windows", "Macos", "Linux"},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureIgnoreAutoScore,
			},
		},
	}
}

func (r *AIChatPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	r.api = initParams.API

	chats, err := r.loadChats(ctx)
	if err != nil {
		r.chats = []AIChatData{}
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to load chats: %s", err.Error()))
	} else {
		r.chats = chats
	}
}

func (r *AIChatPlugin) loadChats(ctx context.Context) ([]AIChatData, error) {
	chats := []AIChatData{}
	chatsJson := r.api.GetSetting(ctx, aiChatsSettingKey)
	if chatsJson == "" {
		return []AIChatData{}, nil
	}

	err := json.Unmarshal([]byte(chatsJson), &chats)
	if err != nil {
		return []AIChatData{}, err
	}

	sort.Slice(chats, func(i, j int) bool {
		return chats[i].UpdatedAt > chats[j].UpdatedAt
	})

	return chats, nil
}

func (r *AIChatPlugin) saveChats(ctx context.Context) {
	chatsJson, err := json.Marshal(r.chats)
	if err != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chats: %s", err.Error()))
		return
	}

	r.api.SaveSetting(ctx, aiChatsSettingKey, string(chatsJson), false)
}

func (r *AIChatPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	results = append(results, plugin.QueryResult{
		Title:    "New Chat",
		SubTitle: "Create a new chat",
		Icon:     aiChatIcon,
		Preview: plugin.WoxPreview{
			PreviewType:    plugin.WoxPreviewTypeChat,
			PreviewData:    "{}",
			ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
		},
		Actions: []plugin.QueryResultAction{
			{
				Name:                   "Start Chat",
				PreventHideAfterAction: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					// focus to chat input
					plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
				},
			},
		},
		Group:      "New Chat",
		GroupScore: 100,
	})

	for i, chat := range r.chats {
		previewData, err := json.Marshal(plugin.WoxPreviewChatData{
			Conversations: chat.Conversations,
			Model:         chat.Model,
		})
		if err != nil {
			r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to marshal chat preview data: %s", err.Error()))
			continue
		}

		results = append(results, plugin.QueryResult{
			Title:    chat.Title,
			SubTitle: fmt.Sprintf("%d messages", len(chat.Conversations)),
			Icon:     aiChatIcon,
			Preview: plugin.WoxPreview{
				PreviewType:    plugin.WoxPreviewTypeChat,
				PreviewData:    string(previewData),
				ScrollPosition: plugin.WoxPreviewScrollPositionBottom,
			},
			Actions: []plugin.QueryResultAction{
				{
					Name:                   "Continue Chat",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// focus to chat input
						plugin.GetPluginManager().GetUI().FocusToChatInput(ctx)
					},
				},
				{
					Name:                   "Delete Chat",
					PreventHideAfterAction: true,
					Action: func(ctx context.Context, actionContext plugin.ActionContext) {
						// delete chat
						r.chats = append(r.chats[:i], r.chats[i+1:]...)
						r.saveChats(ctx)
					},
				},
			},
			Group:      "History",
			GroupScore: 10,
		})
	}

	return results
}
