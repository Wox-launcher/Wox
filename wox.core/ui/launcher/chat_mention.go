package launcher

import (
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"wox/common"
	"wox/util"
)

const (
	// chatMentionPanel is the grouped @ overlay. Kinds are rows, not separate panels.
	chatMentionPanel      = "mentions"
	chatMentionKindPlugin = string(common.AIMentionKindPlugin)
)

var chatMentionTagPattern = regexp.MustCompile(`\{([a-z][a-z0-9_]*):([^}]*)\}`)

type chatMention struct {
	Kind   string
	ID     string
	Name   string
	NameEn string
	Icon   woxImage
}

type chatMentionRef struct {
	Kind string `json:"Kind"`
	ID   string `json:"Id"`
	Name string `json:"Name"`
}

type chatMentionTagRange struct {
	start int
	end   int
	kind  string
	name  string
}

func chatMentionTag(kind, payload string) string {
	return "{" + kind + ":" + payload + "}"
}

func chatMentionChipLabel(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "@"
	}
	return "@" + name
}

func chatMentionsFromPlugins(plugins []chatPluginMention) []chatMention {
	mentions := make([]chatMention, 0, len(plugins))
	for _, plugin := range plugins {
		mentions = append(mentions, chatMention{
			Kind: chatMentionKindPlugin, ID: plugin.ID, Name: plugin.Name, NameEn: plugin.NameEn, Icon: plugin.Icon,
		})
	}
	return mentions
}

func (a *App) chatMentionCatalog() []chatMention {
	if a == nil || a.aiSettings == nil {
		return nil
	}
	// Additional @ kinds append here. The overlay stays one grouped catalog.
	return chatMentionsFromPlugins(a.aiSettings.PluginMentions())
}

func (a *App) chatMentionUsePinYin() bool {
	return a != nil && a.generalSettings != nil && a.usePinYin()
}

func (a *App) filteredChatMentionItems(mentions []chatMention, query string) []chatCommandPaletteItem {
	return chatMentionPaletteItems(mentions, query, a.chatMentionUsePinYin())
}

func chatMentionPaletteItems(mentions []chatMention, query string, usePinYin bool) []chatCommandPaletteItem {
	query = strings.TrimSpace(query)
	type rankedMention struct {
		item  chatCommandPaletteItem
		score int64
		index int
	}
	ranked := make([]rankedMention, 0, len(mentions))
	for index, mention := range mentions {
		score, matched := chatMentionMatchScore(mention, query, usePinYin)
		if !matched {
			continue
		}
		item := chatCommandPaletteItem{group: mention.Kind, sourceIndex: index, title: mention.Name}
		item.searchText = mention.Name + " " + mention.NameEn
		ranked = append(ranked, rankedMention{item: item, score: score, index: index})
	}
	if query != "" {
		sort.SliceStable(ranked, func(i, j int) bool {
			if ranked[i].score != ranked[j].score {
				return ranked[i].score > ranked[j].score
			}
			return ranked[i].index < ranked[j].index
		})
	}
	items := make([]chatCommandPaletteItem, 0, len(ranked))
	for _, entry := range ranked {
		items = append(items, entry.item)
	}
	return items
}

func chatMentionMatchScore(mention chatMention, query string, usePinYin bool) (int64, bool) {
	if query == "" {
		return 0, true
	}
	best := int64(0)
	matched := false
	consider := func(text string) {
		if strings.TrimSpace(text) == "" {
			return
		}
		ok, score := util.IsStringMatchScore(text, query, usePinYin)
		if ok && (!matched || score > best) {
			matched = true
			best = score
		}
	}
	consider(mention.Name)
	consider(mention.NameEn)
	consider(mention.ID)
	return best, matched
}

func chatMentionTagRanges(text string) []chatMentionTagRange {
	matches := chatMentionTagPattern.FindAllStringSubmatchIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}
	ranges := make([]chatMentionTagRange, 0, len(matches))
	for _, match := range matches {
		if len(match) < 6 || match[0] < 0 || match[1] < 0 || match[2] < 0 || match[3] < 0 || match[4] < 0 || match[5] < 0 {
			continue
		}
		kind := text[match[2]:match[3]]
		if !common.IsAIMentionKind(kind) {
			continue
		}
		ranges = append(ranges, chatMentionTagRange{
			start: utf8.RuneCountInString(text[:match[0]]),
			end:   utf8.RuneCountInString(text[:match[1]]),
			kind:  kind,
			name:  text[match[4]:match[5]],
		})
	}
	return ranges
}

func lookupChatMention(kind, name string, mentions []chatMention) (chatMention, bool) {
	kind = strings.TrimSpace(kind)
	name = strings.TrimSpace(name)
	if kind == "" || name == "" {
		return chatMention{}, false
	}
	for _, mention := range mentions {
		if mention.Kind != kind {
			continue
		}
		if mention.Name == name || mention.NameEn == name || mention.ID == name {
			return mention, true
		}
	}
	return chatMention{}, false
}

func chatMentionRefsFromText(text string, catalog []chatMention) []chatMentionRef {
	tags := chatMentionTagRanges(text)
	if len(tags) == 0 {
		return nil
	}
	refs := make([]chatMentionRef, 0, len(tags))
	seen := map[string]bool{}
	for _, tag := range tags {
		mention, ok := lookupChatMention(tag.kind, tag.name, catalog)
		key := mention.Kind + "/" + mention.ID
		if !ok || mention.ID == "" || seen[key] {
			continue
		}
		refs = append(refs, chatMentionRef{Kind: mention.Kind, ID: mention.ID, Name: mention.Name})
		seen[key] = true
	}
	return refs
}

func unresolvedChatMentionTag(text string, catalog []chatMention) string {
	for _, tag := range chatMentionTagRanges(text) {
		if _, ok := lookupChatMention(tag.kind, tag.name, catalog); !ok {
			return chatMentionChipLabel(tag.name)
		}
	}
	return ""
}

func chatMentionTagsPresent(text string) bool {
	return len(chatMentionTagRanges(text)) > 0
}
