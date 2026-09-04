package emoji

import (
	"sort"
	"strings"
)

const (
	browsePerCategory      = 20
	skipBrowseCategoryEn   = "Component"
	frequentlyUsedLimit    = 16
	frequentlyUsedStoreCap = 50
)

type browseItem struct {
	Entry    EmojiData
	Category string
}

type indexedBrowseEntry struct {
	entry    EmojiData
	category string
	index    int
	useCount int
}

// selectBrowseEmojis returns a usage-ranked sample from each catalog category
// for an empty emoji query. Frequently used items in excluded are omitted so
// they are not duplicated, and Component (skin-tone modifiers) is skipped.
func selectBrowseEmojis(emojis []EmojiData, usageCounts map[string]int, excluded map[string]bool, categoryOf func(EmojiData) string, perCategory int) []browseItem {
	if perCategory <= 0 {
		return nil
	}

	order := make([]string, 0)
	grouped := make(map[string][]indexedBrowseEntry)
	for index, entry := range emojis {
		if excluded[entry.Emoji] {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(entry.Categories["en"]), skipBrowseCategoryEn) {
			continue
		}
		category := strings.TrimSpace(categoryOf(entry))
		if category == "" {
			continue
		}
		if _, exists := grouped[category]; !exists {
			order = append(order, category)
		}
		grouped[category] = append(grouped[category], indexedBrowseEntry{
			entry:    entry,
			category: category,
			index:    index,
			useCount: usageCounts[entry.Emoji],
		})
	}

	results := make([]browseItem, 0, len(order)*perCategory)
	for _, category := range order {
		items := grouped[category]
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].useCount != items[j].useCount {
				return items[i].useCount > items[j].useCount
			}
			return items[i].index < items[j].index
		})
		if len(items) > perCategory {
			items = items[:perCategory]
		}
		for _, item := range items {
			results = append(results, browseItem{Entry: item.entry, Category: category})
		}
	}
	return results
}

func usageCountMap(usages []emojiUsage) map[string]int {
	counts := make(map[string]int, len(usages))
	for _, usage := range usages {
		counts[usage.Emoji] = usage.UseCount
	}
	return counts
}
