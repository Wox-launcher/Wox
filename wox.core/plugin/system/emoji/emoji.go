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
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"

	"github.com/google/uuid"
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

	// emoji -> custom descriptions added by user
	customDescriptions map[string][]string
}

func (e *EmojiPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "a5c7d25d-7a3b-4c45-8bd4-6e2d2c2f9e3a",
		Name:          "i18n:plugin_emoji_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_emoji_plugin_description",
		Icon:          emojiIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"emoji",
		},
		Commands: []plugin.MetadataCommand{},
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeCheckBox,
				Value: &definition.PluginSettingValueCheckBox{
					Key:          "aiMatchEnabled",
					Label:        "i18n:plugin_emoji_setting_ai_enable_label",
					Tooltip:      "i18n:plugin_emoji_setting_ai_enable_tooltip",
					DefaultValue: "false",
				},
			},
			{
				Type: definition.PluginSettingDefinitionTypeSelectAIModel,
				Value: &definition.PluginSettingValueSelectAIModel{
					Key:     "aiModel",
					Label:   "i18n:plugin_emoji_setting_ai_model_label",
					Tooltip: "i18n:plugin_emoji_setting_ai_model_tooltip",
				},
			},
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureGridLayout,
				Params: map[string]any{
					"Columns":     12,
					"ItemPadding": 12,
					"ItemMargin":  6,
					"ShowTitle":   false,
				},
			},
			{
				Name: plugin.MetadataFeatureAI,
			},
		},
	}
}

func (e *EmojiPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	e.api = initParams.API
	e.customDescriptions = make(map[string][]string)
	e.loadCustomDescriptions(ctx)
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

	e.applyCustomDescriptions(ctx)
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

		result := e.createEmojiResult(ctx, *entry, true)
		result.Group = "i18n:plugin_emoji_frequently_used"
		result.GroupScore = 100
		results = append(results, result)
		frequentlyUsedCount++
	}

	// Add other emojis
	// If there are frequently used emojis in results, group other emojis under "Emojis"
	count := 0
	existingEmojiSet := make(map[string]bool)
	for _, r := range results {
		if r.Icon.ImageType == common.WoxImageTypeEmoji {
			existingEmojiSet[r.Icon.ImageData] = true
		}
	}
	for _, entry := range e.emojis {
		if frequentlyUsedSet[entry.Emoji] {
			continue // Already added in frequently used group
		}

		if e.matchEmoji(entry, search) {
			result := e.createEmojiResult(ctx, entry, false)
			result.Group = e.getCategoryName(ctx, entry)
			result.GroupScore = 50
			results = append(results, result)
			existingEmojiSet[entry.Emoji] = true
			count++
		}

		if count >= 100 {
			break
		}
	}

	e.maybeStartAIMatch(ctx, query, search, existingEmojiSet, &results)

	return results
}

func (e *EmojiPlugin) maybeStartAIMatch(ctx context.Context, query plugin.Query, search string, existingEmojiSet map[string]bool, results *[]plugin.QueryResult) {
	if query.Id == "" {
		return
	}
	if !e.isAIMatchEnabled(ctx) {
		return
	}
	if len(search) < 2 {
		return
	}

	model, ok := e.getAIModel(ctx)
	if !ok {
		return
	}

	aiGeneratingResult := e.createAIPlaceholderResult()
	*results = append(*results, aiGeneratingResult)

	util.Go(ctx, "emoji ai match", func() {
		systemPrompt := "You are an emoji matcher. Return JSON only."
		userPrompt := fmt.Sprintf("Query: %s\nReturn JSON: {\"emojis\": [\"😀\", \"😄\"]}. Return up to 12 emojis.", search)
		conversations := []common.Conversation{
			{Role: common.ConversationRoleSystem, Text: systemPrompt},
			{Role: common.ConversationRoleUser, Text: userPrompt},
		}

		var finalData string
		err := e.api.AIChatStream(ctx, model, conversations, common.EmptyChatOptions, func(streamResult common.ChatStreamData) {
			if streamResult.Status == common.ChatStreamStatusStreaming || streamResult.Status == common.ChatStreamStatusStreamed || streamResult.Status == common.ChatStreamStatusFinished {
				finalData = streamResult.Data
			}

			if streamResult.Status == common.ChatStreamStatusFinished {
				e.handleAIMatchResult(ctx, query, finalData, existingEmojiSet, aiGeneratingResult.Id)
			}
			if streamResult.Status == common.ChatStreamStatusError {
				e.updateAIPlaceholder(ctx, aiGeneratingResult.Id, "i18n:plugin_emoji_ai_failed", common.NewWoxImageEmoji("⚠️"))
			}
		})
		if err != nil {
			e.updateAIPlaceholder(ctx, aiGeneratingResult.Id, "i18n:plugin_emoji_ai_failed", common.NewWoxImageEmoji("⚠️"))
			return
		}
	})
}

func (e *EmojiPlugin) handleAIMatchResult(ctx context.Context, query plugin.Query, data string, existingEmojiSet map[string]bool, aiGeneratingResultId string) {
	entries := e.parseAIEmojis(data)
	if len(entries) == 0 {
		e.updateAIPlaceholder(ctx, aiGeneratingResultId, "i18n:plugin_emoji_ai_no_result", common.NewWoxImageEmoji("😕"))
		return
	}

	var results []plugin.QueryResult
	for _, entry := range entries {
		if existingEmojiSet[entry.Emoji] {
			continue
		}
		result := e.createEmojiResult(ctx, entry, false)
		result.Group = "i18n:plugin_emoji_ai_group"
		result.GroupScore = 90
		result.Score = 90
		results = append(results, result)
		if len(results) >= 12 {
			break
		}
	}

	if len(results) == 0 {
		e.updateAIPlaceholder(ctx, aiGeneratingResultId, "i18n:plugin_emoji_ai_no_result", common.NewWoxImageEmoji("😕"))
		return
	}
	if len(results) == 1 {
		e.updateAIPlaceholder(ctx, aiGeneratingResultId, "i18n:plugin_emoji_ai_done", results[0].Icon)
	} else {
		e.updateAIPlaceholder(ctx, aiGeneratingResultId, "i18n:plugin_emoji_ai_done", results[0].Icon)
		e.api.PushResults(ctx, query, results[1:])
	}
}

func (e *EmojiPlugin) parseAIEmojis(data string) []EmojiData {
	var results []EmojiData
	seen := make(map[string]bool)

	if gjson.Valid(data) {
		emojiArray := gjson.Get(data, "emojis")
		if emojiArray.IsArray() {
			emojiArray.ForEach(func(_, value gjson.Result) bool {
				emoji := strings.TrimSpace(value.String())
				if emoji == "" || seen[emoji] {
					return true
				}
				entry := e.findEmoji(emoji)
				if entry != nil {
					results = append(results, *entry)
					seen[emoji] = true
				}
				return true
			})
		}
	}

	if len(results) > 0 {
		return results
	}

	for _, entry := range e.emojis {
		if seen[entry.Emoji] {
			continue
		}
		if strings.Contains(data, entry.Emoji) {
			results = append(results, entry)
			seen[entry.Emoji] = true
		}
	}

	return results
}

func (e *EmojiPlugin) createAIPlaceholderResult() plugin.QueryResult {
	return plugin.QueryResult{
		Id:         uuid.New().String(),
		Title:      "i18n:plugin_emoji_ai_matching",
		SubTitle:   "i18n:plugin_emoji_ai_matching_subtitle",
		Icon:       common.AnimatedLoadingIcon,
		Group:      "i18n:plugin_emoji_ai_group",
		GroupScore: 90,
		Score:      90,
	}
}

func (e *EmojiPlugin) updateAIPlaceholder(ctx context.Context, resultId string, subtitle string, icon common.WoxImage) {
	updatable := e.api.GetUpdatableResult(ctx, resultId)
	if updatable == nil {
		return
	}

	updatable.SubTitle = &subtitle
	updatable.Icon = &icon
	e.api.UpdateResult(ctx, *updatable)
}

func (e *EmojiPlugin) getAIModel(ctx context.Context) (common.Model, bool) {
	modelRaw := strings.TrimSpace(e.api.GetSetting(ctx, "aiModel"))
	if modelRaw == "" {
		return common.Model{}, false
	}
	var model common.Model
	if err := json.Unmarshal([]byte(modelRaw), &model); err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse AI model: %v", err))
		return common.Model{}, false
	}
	if model.Name == "" || model.Provider == "" {
		return common.Model{}, false
	}
	return model, true
}

func (e *EmojiPlugin) isAIMatchEnabled(ctx context.Context) bool {
	return strings.EqualFold(strings.TrimSpace(e.api.GetSetting(ctx, "aiMatchEnabled")), "true")
}

func (e *EmojiPlugin) createEmojiResult(ctx context.Context, entry EmojiData, isFrequentlyUsed bool) plugin.QueryResult {
	emoji := entry.Emoji
	title := e.getDisplayName(ctx, entry)
	subTitle := e.getSecondaryName(title, entry)

	result := plugin.QueryResult{
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
			{
				Name:   "i18n:plugin_emoji_copy_large",
				Icon:   common.NewWoxImageEmoji("🖼️"),
				Hotkey: "ctrl+enter",
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					img, err := getNativeEmojiImage(emoji, 200)
					if err != nil {
						e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to build emoji image: %v", err))
						e.api.Notify(ctx, fmt.Sprintf("Failed to build emoji image: %v", err))
						return
					}

					if err := clipboard.Write(&clipboard.ImageData{Image: img}); err != nil {
						e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to copy emoji image: %v", err))
						return
					}

					e.recordUsage(ctx, emoji)
				},
			},
		},
	}

	existingDescriptions := strings.Join(e.customDescriptions[emoji], ", ")

	result.Actions = append(result.Actions, plugin.QueryResultAction{
		Name:                   "i18n:plugin_emoji_add_keyword",
		Icon:                   common.AirdropIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		Form: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTextBox,
				Value: &definition.PluginSettingValueTextBox{
					Key:          "keyword",
					Label:        "i18n:plugin_emoji_add_keyword_label",
					DefaultValue: existingDescriptions,
					Tooltip:      "i18n:plugin_emoji_add_keyword_hint",
					MaxLines:     2,
				},
			},
		},
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			raw := strings.TrimSpace(actionContext.Values["keyword"])
			if raw == "" {
				return
			}
			parts := strings.Split(raw, ",")
			var descriptions []string
			for _, p := range parts {
				if trimmed := strings.TrimSpace(p); trimmed != "" {
					descriptions = append(descriptions, trimmed)
				}
			}
			if len(descriptions) == 0 {
				return
			}
			e.addCustomDescriptions(ctx, emoji, descriptions)
			e.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	})

	if isFrequentlyUsed {
		result.Actions = append(result.Actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_emoji_remove_frequently_used",
			Icon:                   common.TrashIcon,
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				e.removeUsage(ctx, emoji)
				e.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			},
		})
	}

	return result
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

func (e *EmojiPlugin) removeUsage(ctx context.Context, emoji string) {
	usages := e.getFrequentlyUsed(ctx)
	if len(usages) == 0 {
		return
	}

	filtered := usages[:0]
	for _, usage := range usages {
		if usage.Emoji != emoji {
			filtered = append(filtered, usage)
		}
	}

	data, err := json.Marshal(filtered)
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

	// Match custom descriptions
	if extras, ok := e.customDescriptions[entry.Emoji]; ok {
		for _, desc := range extras {
			if strings.Contains(strings.ToLower(desc), search) {
				return true
			}
		}
	}

	return false
}

func (e *EmojiPlugin) loadCustomDescriptions(ctx context.Context) {
	data := e.api.GetSetting(ctx, "customDescriptions")
	if strings.TrimSpace(data) == "" {
		return
	}
	var saved map[string][]string
	if err := json.Unmarshal([]byte(data), &saved); err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to parse custom descriptions: %v", err))
		return
	}
	e.customDescriptions = saved
}

func (e *EmojiPlugin) addCustomDescriptions(ctx context.Context, emoji string, descriptions []string) {
	var cleaned []string
	for _, desc := range descriptions {
		trimmed := strings.TrimSpace(desc)
		if trimmed == "" {
			continue
		}
		cleaned = append(cleaned, trimmed)
	}
	if len(cleaned) == 0 {
		return
	}

	values := e.customDescriptions[emoji]
	for _, v := range cleaned {
		duplicate := false
		for _, exist := range values {
			if strings.EqualFold(strings.TrimSpace(exist), v) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			values = append(values, v)
		}
	}
	e.customDescriptions[emoji] = values

	e.applyCustomDescriptions(ctx)

	data, err := json.Marshal(e.customDescriptions)
	if err != nil {
		e.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to serialize custom descriptions: %v", err))
		return
	}
	e.api.SaveSetting(ctx, "customDescriptions", string(data), false)
}

func (e *EmojiPlugin) applyCustomDescriptions(ctx context.Context) {
	if len(e.customDescriptions) == 0 {
		return
	}

	custom := make(map[string][]string)
	for k, v := range e.customDescriptions {
		for _, item := range v {
			trimmed := strings.ToLower(strings.TrimSpace(item))
			if trimmed == "" {
				continue
			}
			custom[k] = append(custom[k], trimmed)
		}
	}

	for i := range e.emojis {
		terms := e.emojis[i].SearchTerms
		if extra, ok := custom[e.emojis[i].Emoji]; ok {
			terms = append(terms, extra...)
		}
		e.emojis[i].SearchTerms = uniqueLower(terms)
	}
}

func uniqueLower(inputs []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, v := range inputs {
		l := strings.ToLower(strings.TrimSpace(v))
		if l == "" {
			continue
		}
		if seen[l] {
			continue
		}
		seen[l] = true
		out = append(out, l)
	}
	return out
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
