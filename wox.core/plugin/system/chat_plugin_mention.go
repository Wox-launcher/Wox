package system

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"wox/ai"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
)

// ListMentionablePlugins returns enabled plugins that currently expose at least one tool.
func (r *AIChatPlugin) ListMentionablePlugins(ctx context.Context) []common.AIPluginMention {
	listed := r.api.ListPluginTools(ctx, plugin.ListPluginToolsOption{})
	if listed.Error != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: failed to list plugin tools for mentions: %s", listed.Error.Error()))
		return nil
	}

	mentions := make([]common.AIPluginMention, 0)
	seen := map[string]bool{}
	for _, item := range listed.Tools {
		pluginID := strings.TrimSpace(item.PluginId)
		if pluginID == "" || pluginID == common.AIChatPluginID || seen[pluginID] {
			continue
		}
		name := strings.TrimSpace(item.PluginName)
		instance := plugin.GetPluginManager().GetPluginInstanceById(pluginID)
		if instance != nil {
			if localized := strings.TrimSpace(instance.GetName(ctx)); localized != "" {
				name = localized
			}
		}
		if name == "" {
			continue
		}
		seen[pluginID] = true
		mention := common.AIPluginMention{Id: pluginID, Name: name, NameEn: name}
		if instance != nil {
			if english := strings.TrimSpace(instance.Metadata.GetNameEn(ctx)); english != "" {
				mention.NameEn = english
			}
			mention.Icon = instance.Metadata.GetIconOrDefault(instance.PluginDirectory, icons.Get(icons.BrandWox))
		}
		mentions = append(mentions, mention)
	}
	sort.SliceStable(mentions, func(i, j int) bool {
		return strings.ToLower(mentions[i].Name) < strings.ToLower(mentions[j].Name)
	})
	return mentions
}

// mentionIDs keeps explicitly selected capabilities available across later chat turns.
func mentionIDs(conversations []common.Conversation, kind common.AIMentionKind) []string {
	ids := make([]string, 0)
	seen := map[string]bool{}
	for _, conversation := range conversations {
		for _, ref := range conversation.Mentions {
			if ref.Kind != kind {
				continue
			}
			id := strings.TrimSpace(ref.Id)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

// pluginToolsForMentions exposes only selected plugins and preserves their registered schemas.
func (r *AIChatPlugin) pluginToolsForMentions(ctx context.Context, pluginIDs []string) []common.Tool {
	if len(pluginIDs) == 0 {
		return nil
	}
	wanted := map[string]bool{}
	for _, pluginID := range pluginIDs {
		wanted[pluginID] = true
	}
	listed := r.api.ListPluginTools(ctx, plugin.ListPluginToolsOption{})
	if listed.Error != nil {
		r.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("AI: failed to list mentioned plugin tools: %s", listed.Error.Error()))
		return nil
	}

	tools := make([]common.Tool, 0)
	for _, item := range listed.Tools {
		if !wanted[item.PluginId] {
			continue
		}
		pluginID := item.PluginId
		toolName := item.Tool.Name
		pluginName := item.PluginName
		aiName := ai.PluginToolAIName(pluginID, toolName)
		tools = append(tools, common.Tool{
			Name:        aiName,
			Description: fmt.Sprintf("[%s] %s", pluginName, item.Tool.Description),
			InputSchema: item.Tool.InputSchema,
			Source:      common.ToolSourcePlugin,
			PluginId:    pluginID,
			PluginName:  pluginName,
			Callback: func(ctx context.Context, args map[string]any) (common.ToolResult, error) {
				return invokeMentionedPluginTool(ctx, r.api, pluginID, toolName, args)
			},
		})
	}
	return tools
}

// invokeMentionedPluginTool retains the shared plugin validation and lifecycle boundary.
func invokeMentionedPluginTool(ctx context.Context, api plugin.API, pluginID, name string, args map[string]any) (common.ToolResult, error) {
	result := api.InvokePluginTool(ctx, plugin.InvokePluginToolOption{
		PluginId:  pluginID,
		Name:      name,
		Arguments: args,
	})
	if result.Error != nil {
		return common.ToolResult{}, fmt.Errorf("%s: %s", result.Error.Code, result.Error.Message)
	}
	if len(result.Output) == 0 {
		return common.ToolResult{Text: "{}"}, nil
	}
	encoded, err := json.Marshal(result.Output)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: string(encoded)}, nil
}

// formatMentionedPluginsPrompt explains the user's plugin selections without exposing UI tags.
func formatMentionedPluginsPrompt(refs []common.AIMentionRef) string {
	var builder strings.Builder
	seen := map[string]bool{}
	for _, ref := range refs {
		if ref.Kind != common.AIMentionKindPlugin {
			continue
		}
		name := strings.TrimSpace(ref.Name)
		if name == "" {
			name = strings.TrimSpace(ref.Id)
		}
		if name == "" || seen[name] {
			continue
		}
		if builder.Len() == 0 {
			builder.WriteString("The user @mentioned these Wox plugins. Their tools are already enabled and callable now. Prefer these plugin tools for work those plugins can do.")
		}
		seen[name] = true
		builder.WriteString("\n- ")
		builder.WriteString(name)
	}
	return builder.String()
}
