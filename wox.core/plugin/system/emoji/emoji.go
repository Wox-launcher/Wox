package emoji

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/resource"
	"wox/util/clipboard"
)

var emojiIcon = common.PluginEmojiIcon

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &EmojiPlugin{})
}

type EmojiEntry struct {
	Emoji    string
	Keywords []string
}

type emojiUsage struct {
	Emoji    string `json:"emoji"`
	UseCount int    `json:"useCount"`
}

type EmojiPlugin struct {
	api    plugin.API
	emojis []EmojiEntry
}

func (e *EmojiPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a5c7d25d-7a3b-4c45-8bd4-6e2d2c2f9e3a",
		Name:          "Emoji",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "Search and copy emojis",
		Icon:          emojiIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"emoji",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureGridLayout,
				Params: map[string]string{
					"Columns":     "12",
					"ItemPadding": "12",
					"ItemMargin":  "6",
					"ShowTitle":   "false",
				},
			},
		},
	}
}

func (e *EmojiPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	e.api = initParams.API
	e.loadEmojis(ctx)
}

func (e *EmojiPlugin) loadEmojis(ctx context.Context) {
	data, err := resource.GetEmojiJson(ctx)
	if err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read emoji data: %v", err))
		return
	}

	var emojiMap map[string][]string
	if err := json.Unmarshal(data, &emojiMap); err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse emoji data: %v", err))
		return
	}

	for emoji, keywords := range emojiMap {
		e.emojis = append(e.emojis, EmojiEntry{
			Emoji:    emoji,
			Keywords: keywords,
		})
	}

	e.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("Loaded %d emojis", len(e.emojis)))
}

func (e *EmojiPlugin) Query(ctx context.Context, query plugin.Query) []plugin.QueryResult {
	var results []plugin.QueryResult
	search := strings.ToLower(strings.TrimSpace(query.Search))

	// Get frequently used emojis
	frequentlyUsed := e.getFrequentlyUsed(ctx)
	frequentlyUsedSet := make(map[string]bool)
	for _, usage := range frequentlyUsed {
		frequentlyUsedSet[usage.Emoji] = true
	}

	// Add frequently used emojis first (as a group)
	frequentlyUsedCount := 0
	for _, usage := range frequentlyUsed {
		entry := e.findEmojiEntry(usage.Emoji)
		if entry == nil {
			continue
		}
		if !e.matchEmoji(*entry, search) {
			continue
		}

		result := e.createEmojiResult(ctx, *entry)
		result.Group = "i18n:plugin_emoji_frequently_used"
		result.GroupScore = 100
		results = append(results, result)
		frequentlyUsedCount++
	}

	// Add other emojis
	// If there are frequently used emojis in results, group other emojis under "Emojis"
	count := 0
	for _, entry := range e.emojis {
		if frequentlyUsedSet[entry.Emoji] {
			continue // Already added in frequently used group
		}
		if !e.matchEmoji(entry, search) {
			continue
		}

		result := e.createEmojiResult(ctx, entry)
		if frequentlyUsedCount > 0 {
			result.Group = "i18n:plugin_emoji_all"
			result.GroupScore = 50
		}
		results = append(results, result)
		count++
		if count >= 100 {
			break
		}
	}

	return results
}

func (e *EmojiPlugin) createEmojiResult(ctx context.Context, entry EmojiEntry) plugin.QueryResult {
	emoji := entry.Emoji
	subTitle := ""
	if len(entry.Keywords) > 0 {
		subTitle = entry.Keywords[0]
	}

	return plugin.QueryResult{
		Title:    strings.Join(entry.Keywords, ", "),
		SubTitle: subTitle,
		Icon:     common.NewWoxImageEmoji(emoji),
		Actions: []plugin.QueryResultAction{
			{
				Name:      "i18n:plugin_emoji_copy",
				Icon:      common.CopyIcon,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					clipboard.WriteText(emoji)
					e.recordUsage(ctx, emoji)
				},
			},
		},
	}
}

func (e *EmojiPlugin) findEmojiEntry(emoji string) *EmojiEntry {
	for _, entry := range e.emojis {
		if entry.Emoji == emoji {
			return &entry
		}
	}
	return nil
}

func (e *EmojiPlugin) getFrequentlyUsed(ctx context.Context) []emojiUsage {
	data := e.api.GetSetting(ctx, "frequentlyUsed")
	if data == "" {
		return nil
	}

	var usages []emojiUsage
	if err := json.Unmarshal([]byte(data), &usages); err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse frequently used data: %v", err))
		return nil
	}

	// Sort by use count descending
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].UseCount > usages[j].UseCount
	})

	// Limit to top 16 (2 rows in grid)
	if len(usages) > 16 {
		usages = usages[:16]
	}

	return usages
}

func (e *EmojiPlugin) recordUsage(ctx context.Context, emoji string) {
	usages := e.getFrequentlyUsed(ctx)
	if usages == nil {
		usages = []emojiUsage{}
	}

	// Find existing or add new
	found := false
	for i := range usages {
		if usages[i].Emoji == emoji {
			usages[i].UseCount++
			found = true
			break
		}
	}
	if !found {
		usages = append(usages, emojiUsage{Emoji: emoji, UseCount: 1})
	}

	// Keep only top 50 to limit storage
	sort.Slice(usages, func(i, j int) bool {
		return usages[i].UseCount > usages[j].UseCount
	})
	if len(usages) > 50 {
		usages = usages[:50]
	}

	data, err := json.Marshal(usages)
	if err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to serialize frequently used data: %v", err))
		return
	}
	e.api.SaveSetting(ctx, "frequentlyUsed", string(data), false)
}

func (e *EmojiPlugin) matchEmoji(entry EmojiEntry, search string) bool {
	if search == "" {
		return true
	}

	// Match emoji character itself
	if strings.Contains(entry.Emoji, search) {
		return true
	}

	// Match any keyword
	for _, keyword := range entry.Keywords {
		if strings.Contains(strings.ToLower(keyword), search) {
			return true
		}
	}

	return false
}
