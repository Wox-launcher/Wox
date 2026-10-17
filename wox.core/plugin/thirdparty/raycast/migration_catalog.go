package raycast

import (
	"strconv"

	"wox/plugin/thirdparty/migrate"
)

func catalogFrom(mapped mappedExport) []migrate.Category {
	var categories []migrate.Category
	if items := hotkeyItems(mapped.hotkeys); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryHotkeys, Items: items})
	}
	if items := settingItems(mapped.general); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryGeneral, Items: items})
	}
	if items := queryItems(mapped.queries); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryQueries, Items: items})
	}
	if items := quicklinkItems(mapped.quicklinks); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryQuicklinks, Items: items})
	}
	if items := shortcutItems(mapped.shortcuts); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryShortcuts, Items: items})
	}
	if items := snippetItems(mapped.snippets); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategorySnippets, Items: items})
	}
	if items := noteItems(mapped.notes); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryNotes, Items: items})
	}
	if items := clipboardItems(mapped.clipboard, mapped.summary); len(items) > 0 {
		categories = append(categories, migrate.Category{ID: migrate.CategoryClipboard, Items: items})
	}
	return categories
}

func hotkeyItems(rows []mappedHotkey) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: migrate.ItemKindSetting, TitleKey: row.titleKey, Title: row.title,
			DetailKey: row.detailKey, Detail: row.detail, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func shortcutItems(rows []mappedShortcut) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: migrate.ItemKindResultBinding, Title: row.title,
			Detail: row.detail, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func queryItems(rows []mappedQuery) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: migrate.ItemKindSetting, TitleKey: "query_alias", Title: row.title,
			DetailKey: row.detailKey, Detail: row.detail, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func settingItems(rows []mappedSetting) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: migrate.ItemKindSetting, TitleKey: row.titleKey,
			DetailKey: row.detailKey, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func quicklinkItems(rows []mappedQuicklink) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		kind := migrate.ItemKindQuicklink
		if row.favorite {
			kind = migrate.ItemKindFavorite
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: kind, Title: row.title,
			DetailKey: row.detailKey, Detail: row.detail, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func snippetItems(rows []mappedSnippet) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		item := migrate.Item{
			ID: row.id, Kind: migrate.ItemKindNote, Title: row.title,
			DetailKey: row.detailKey, Detail: row.detail, Selectable: row.selectable, Status: row.status,
		}
		if row.keyword != "" && row.selectable {
			item.OptionKey = "raycast_keep_keyword"
			item.OptionOn = row.keepKeyword
		}
		items = append(items, item)
	}
	return items
}

func noteItems(rows []mappedNote) []migrate.Item {
	items := make([]migrate.Item, 0, len(rows))
	for _, row := range rows {
		if !row.selectable {
			continue
		}
		items = append(items, migrate.Item{
			ID: row.id, Kind: migrate.ItemKindNote, Title: row.title,
			Detail: row.detail, Selectable: row.selectable, Status: row.status,
		})
	}
	return items
}

func clipboardItems(rows []mappedClipboard, summary clipboardSummary) []migrate.Item {
	if len(rows) == 0 && summary == (clipboardSummary{}) {
		return nil
	}
	return []migrate.Item{{
		ID: migrate.ClipboardSummaryID, Kind: migrate.ItemKindClipboard, TitleKey: "clipboard_summary",
		Detail:    strconv.Itoa(summary.importable) + "|" + strconv.Itoa(summary.duplicate) + "|" + strconv.Itoa(summary.expired) + "|" + strconv.Itoa(summary.capacity) + "|" + strconv.Itoa(summary.missing),
		DetailKey: "raycast_clipboard_summary", Selectable: summary.importable > 0,
	}}
}
