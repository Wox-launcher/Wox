package emoji

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util/clipboard"

	"github.com/tidwall/gjson"
)

var emojiIcon = common.PluginEmojiIcon

//go:embed emoji-data.json
var emojiFS embed.FS

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &EmojiPlugin{})
}

type EmojiData struct {
	Emoji       string
	Codes       string
	Categories  map[string]string // language code -> category name
	Names       map[string]string // language code -> emoji name
	SearchTerms []string
}

type emojiUsage struct {
	Emoji    string `json:"emoji"`
	UseCount int    `json:"useCount"`
}

type EmojiPlugin struct {
	api    plugin.API
	emojis []EmojiData
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
	data, err := emojiFS.ReadFile("emoji-data.json")
	if err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read emoji data: %v", err))
		return
	}

	jsonResult := gjson.ParseBytes(data)
	if !jsonResult.IsArray() {
		e.api.Log(ctx, plugin.LogLevelError, "Failed to parse emoji data: root is not array")
		return
	}

	seen := make(map[string]bool)
	jsonResult.ForEach(func(_, category gjson.Result) bool {
		categoryNames := resultToStringMap(category.Get("name_i18n"))
		if base := strings.TrimSpace(category.Get("name").String()); base != "" && categoryNames["en"] == "" {
			categoryNames["en"] = base
		}
		category.Get("list").ForEach(func(_, subCategory gjson.Result) bool {
			subCategory.Get("list").ForEach(func(_, emoji gjson.Result) bool {
				char := emoji.Get("char").String()
				if char == "" || seen[char] {
					return true
				}

				names := make(map[string]string)
				if base := strings.TrimSpace(emoji.Get("name").String()); base != "" {
					names["en"] = base
				}
				for lang, name := range resultToStringMap(emoji.Get("name_i18n")) {
					if strings.TrimSpace(name) == "" {
						continue
					}
					names[lang] = name
				}

				e.emojis = append(e.emojis, EmojiData{
					Emoji:       char,
					Codes:       emoji.Get("codes").String(),
					Categories:  categoryNames,
					Names:       names,
					SearchTerms: buildSearchTerms(names, categoryNames),
				})
				seen[char] = true
				return true
			})
			return true
		})
		return true
	})

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
		entry := e.findEmoji(usage.Emoji)
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

		if e.matchEmoji(entry, search) {
			result := e.createEmojiResult(ctx, entry)
			result.Group = e.getCategoryName(ctx, entry)
			result.GroupScore = 50
			results = append(results, result)
			count++
		}

		if count >= 100 {
			break
		}
	}

	return results
}

func (e *EmojiPlugin) createEmojiResult(ctx context.Context, entry EmojiData) plugin.QueryResult {
	emoji := entry.Emoji
	title := e.getDisplayName(ctx, entry)
	subTitle := e.getSecondaryName(title, entry)

	return plugin.QueryResult{
		Title:    title,
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

func (e *EmojiPlugin) findEmoji(emoji string) *EmojiData {
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

func (e *EmojiPlugin) matchEmoji(entry EmojiData, search string) bool {
	if search == "" {
		return true
	}

	// Match emoji character itself
	if strings.Contains(entry.Emoji, search) {
		return true
	}

	// Match any name across languages (pre-lowered)
	for _, keyword := range entry.SearchTerms {
		if strings.Contains(keyword, search) {
			return true
		}
	}

	return false
}

func buildSearchTerms(maps ...map[string]string) []string {
	seen := make(map[string]bool)
	var terms []string
	for _, m := range maps {
		for _, name := range m {
			trimmed := strings.ToLower(strings.TrimSpace(name))
			if trimmed == "" {
				continue
			}
			if seen[trimmed] {
				continue
			}
			seen[trimmed] = true
			terms = append(terms, trimmed)
		}
	}
	return terms
}

func resultToStringMap(result gjson.Result) map[string]string {
	values := make(map[string]string)
	if !result.IsObject() {
		return values
	}

	result.ForEach(func(key, value gjson.Result) bool {
		values[key.String()] = value.String()
		return true
	})
	return values
}

func pickLangValue(i18n map[string]string, fallback string) string {
	if v, ok := i18n["en"]; ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}

	for _, v := range i18n {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}

	return strings.TrimSpace(fallback)
}

func (e *EmojiPlugin) getDisplayName(ctx context.Context, entry EmojiData) string {
	langCode := setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get()
	preferred := e.getPreferredLangKeys(string(langCode))

	for _, key := range preferred {
		if name, ok := entry.Names[key]; ok {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				return trimmed
			}
		}
	}

	for _, name := range entry.Names {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}

	return entry.Emoji
}

func (e *EmojiPlugin) getCategoryName(ctx context.Context, entry EmojiData) string {
	langCode := setting.GetSettingManager().GetWoxSetting(ctx).LangCode.Get()
	preferred := e.getPreferredLangKeys(string(langCode))

	for _, key := range preferred {
		if name, ok := entry.Categories[key]; ok {
			if trimmed := strings.TrimSpace(name); trimmed != "" {
				return trimmed
			}
		}
	}

	for _, name := range entry.Categories {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}

	// use en as fallback
	if name, ok := entry.Categories["en"]; ok {
		if trimmed := strings.TrimSpace(name); trimmed != "" {
			return trimmed
		}
	}

	return ""
}

func (e *EmojiPlugin) getSecondaryName(primary string, entry EmojiData) string {
	primaryLower := strings.ToLower(strings.TrimSpace(primary))

	if name, ok := entry.Names["en"]; ok {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" && strings.ToLower(trimmed) != primaryLower {
			return trimmed
		}
	}

	for _, name := range entry.Names {
		trimmed := strings.TrimSpace(name)
		if trimmed == "" {
			continue
		}
		if strings.ToLower(trimmed) == primaryLower {
			continue
		}
		return trimmed
	}

	return ""
}

func (e *EmojiPlugin) getPreferredLangKeys(langCode string) []string {
	var preferred []string
	if langCode != "" {
		preferred = append(preferred, langCode)
		if strings.Contains(langCode, "_") {
			parts := strings.SplitN(langCode, "_", 2)
			if len(parts) == 2 {
				base := parts[0]
				if base != "" && base != langCode {
					preferred = append(preferred, base)
				}
			}
		}
	}

	preferred = append(preferred, "en")
	return preferred
}
