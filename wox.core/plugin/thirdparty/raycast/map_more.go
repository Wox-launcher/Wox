package raycast

import (
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

func mapQuicklinkRows(payload Payload, current woxSnapshot) []mappedQuicklink {
	platforms := map[string]OpenWithPlatform{}
	for _, platform := range payload.Quicklinks.OpenWithPlatforms {
		platforms[platform.ID] = platform
	}
	used := map[string]mappedQuicklink{}
	usedFavorites := map[string]mappedQuicklink{}
	var rows []mappedQuicklink
	for index, link := range payload.Quicklinks.Quicklinks {
		row := mappedQuicklink{id: "quicklink:" + strconv.Itoa(index), title: strings.TrimSpace(link.Name)}
		if row.title == "" {
			continue
		}
		if path, ok := quicklinkLocalPath(quicklinkTarget(link)); ok {
			row.favorite = true
			row.path = path
			row.detail = path
			nameKey := strings.ToLower(row.title)
			if !quicklinkPathExists(path) {
				row.status = statusUnsupported
				row.detailKey = "raycast_path_missing"
			} else if previous, exists := usedFavorites[nameKey]; exists {
				row.selectable = false
				if sameFavoritePath(previous.path, row.path) {
					row.status = statusExists
				} else {
					row.status = statusConflict
					row.detailKey = "raycast_conflict"
				}
			} else if match, exists := favoriteMatch(current.Favorites, row.title); exists {
				row.selectable = false
				if sameFavoritePath(match.Path, row.path) {
					row.status = statusExists
				} else {
					row.status = statusConflict
					row.detailKey = "raycast_conflict"
				}
			} else {
				row.selectable = true
				usedFavorites[nameKey] = row
			}
			rows = append(rows, row)
			continue
		}
		keyword, keywordOK := quicklinkKeyword(row.title)
		rewritten, linkOK := rewriteQuicklinkLink(quicklinkTarget(link))
		row.keyword = keyword
		row.url = rewritten
		openWith := resolveOpenWith(link.OpenWith, platforms)
		if browser, ok := browserForPath(openWith); ok {
			row.browser = browser
		} else if strings.TrimSpace(openWith) != "" {
			row.detailKey = "raycast_browser_fallback"
			row.detail = filepath.Base(openWith)
		}
		row.detail = joinDetail(row.keyword, row.url, row.detail)
		switch {
		case !keywordOK:
			row.status = statusUnsupported
			row.detailKey = "raycast_keyword_rejected"
		case !linkOK:
			row.status = statusUnsupported
			row.detailKey = "raycast_placeholder"
			row.detail = quicklinkTarget(link)
		case keywordTaken(keyword, current):
			row.status = statusUnsupported
			row.detailKey = "raycast_keyword_taken"
		default:
			if previous, exists := used[keyword]; exists && !sameQuicklink(previous, row) {
				row.selectable = false
				row.status = statusConflict
				row.detailKey = "raycast_conflict"
			} else if match, exists := webSearchMatch(current.WebSearches, keyword); exists {
				row.selectable = false
				if sameStoredQuicklink(match, row) {
					row.status = statusExists
				} else {
					row.status = statusConflict
					row.detailKey = "raycast_conflict"
				}
			} else {
				row.selectable = true
				used[keyword] = row
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func favoriteMatch(rows []favoriteRow, name string) (favoriteRow, bool) {
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.Name), strings.TrimSpace(name)) {
			return row, true
		}
	}
	return favoriteRow{}, false
}

func sameFavoritePath(left, right string) bool {
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if runtime.GOOS == "windows" || runtime.GOOS == "darwin" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func resolveOpenWith(value string, platforms map[string]OpenWithPlatform) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if platform, ok := platforms[value]; ok {
		switch runtime.GOOS {
		case "windows":
			if platform.Windows != "" {
				return platform.Windows
			}
		case "darwin":
			if platform.MacOS != "" {
				return platform.MacOS
			}
		case "linux":
			if platform.Linux != "" {
				return platform.Linux
			}
		}
	}
	return value
}

func keywordTaken(keyword string, current woxSnapshot) bool {
	if current.OccupiedKeywords == nil {
		return false
	}
	_, ok := current.OccupiedKeywords[strings.ToLower(keyword)]
	return ok
}

func sameQuicklink(left, right mappedQuicklink) bool {
	return left.url == right.url && left.browser == right.browser && left.title == right.title
}

func sameStoredQuicklink(row webSearchRow, item mappedQuicklink) bool {
	browser := item.browser
	if browser == "" {
		browser = "default"
	}
	stored := row.Browser
	if stored == "" {
		stored = "default"
	}
	return row.URL == item.url && stored == browser && row.Title == item.title
}

func webSearchMatch(rows []webSearchRow, keyword string) (webSearchRow, bool) {
	for _, row := range rows {
		if strings.EqualFold(row.Keyword, keyword) {
			return row, true
		}
	}
	return webSearchRow{}, false
}

func joinDetail(parts ...string) string {
	var kept []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			kept = append(kept, part)
		}
	}
	return strings.Join(kept, "  ")
}

func mapSnippetRows(payload Payload, current woxSnapshot) []mappedSnippet {
	var rows []mappedSnippet
	for index, snippet := range payload.Snippets.Snippets {
		title := strings.TrimSpace(snippet.Title)
		if title == "" {
			continue
		}
		row := mappedSnippet{
			id: "snippet:" + strconv.Itoa(index), sourceID: strings.TrimSpace(snippet.ID),
			title: title, text: snippet.Text, keyword: strings.TrimSpace(snippet.Keyword), keepKeyword: true,
			selectable: true,
		}
		if snippetHasDynamic(snippet.Text) {
			row.detailKey = "raycast_snippet_literal"
		} else if row.keyword != "" {
			row.detailKey = "raycast_snippet_keyword"
		}
		row.detail = snippetBody(row.text, row.keyword, true)
		applySnippetState(&row, current)
		rows = append(rows, row)
	}
	return rows
}

func applySnippetState(row *mappedSnippet, current woxSnapshot) {
	body := snippetBody(row.text, row.keyword, row.keepKeyword)
	row.detail = body
	fingerprint := snippetFingerprint(row.sourceID, row.title, row.text, row.keyword, row.keepKeyword)
	for _, note := range current.Notes {
		if note.Deleted {
			continue
		}
		if note.Fingerprint == fingerprint || (row.sourceID != "" && note.SourceID == row.sourceID) {
			row.selectable = false
			// A title suffix from an earlier import still identifies the same snippet.
			if note.Body == body {
				row.status = statusExists
			} else {
				row.status = statusConflict
				row.detailKey = "raycast_conflict"
			}
			return
		}
	}
}

func mapClipboardRows(payload Payload, current woxSnapshot) ([]mappedClipboard, clipboardSummary) {
	var rows []mappedClipboard
	summary := clipboardSummary{}
	remaining := current.MaxHistory - current.HistoryCount
	if current.MaxHistory <= 0 {
		remaining = 5000
	}
	if remaining < 0 {
		remaining = 0
	}
	var fresh []mappedClipboard
	for index, entry := range payload.ClipboardHistory.Entries {
		text, imagePath, missing := clipboardTextAndImage(entry)
		if text == "" && imagePath == "" && !missing {
			continue
		}
		created, _ := parseRaycastTime(entry.CreatedAt)
		pinned := clipboardPinned(entry.Pinned)
		row := mappedClipboard{
			id: "clipboard:" + strconv.Itoa(index), text: text, imagePath: imagePath,
			createdAt: created, pinned: pinned,
		}
		if text == "" && imagePath != "" && !missing {
			row.imageHash = localImageHash(imagePath)
		}
		if missing {
			row.status = statusUnsupported
			row.detailKey = "raycast_image_missing"
			row.detail = imagePath
			summary.missing++
			rows = append(rows, row)
			continue
		}
		if text != "" && len(text) > 0 && currentTextTooLarge(text) {
			row.status = statusUnsupported
			row.detailKey = "raycast_limit_item"
			rows = append(rows, row)
			continue
		}
		if !pinned && expiredClipboard(row, current) {
			row.status = statusUnsupported
			row.detailKey = "raycast_clipboard_expired"
			summary.expired++
			rows = append(rows, row)
			continue
		}
		if hit, exists := clipboardExisting(row, current); exists {
			if row.pinned && !hit.Favorite {
				row.selectable = true
				row.detailKey = "raycast_clipboard_promote"
				summary.importable++
			} else {
				row.status = statusExists
				summary.duplicate++
			}
			rows = append(rows, row)
			continue
		}
		fresh = append(fresh, row)
	}
	sortClipboardNewest(fresh)
	kept := remaining
	for _, row := range fresh {
		if !row.pinned && kept <= 0 {
			row.status = statusUnsupported
			row.detailKey = "raycast_clipboard_capacity"
			summary.capacity++
			rows = append(rows, row)
			continue
		}
		row.selectable = true
		if row.text != "" {
			row.detail = trimPreview(row.text)
		} else {
			row.detail = filepath.Base(row.imagePath)
			row.detailKey = "raycast_clipboard_image"
		}
		if !row.pinned {
			kept--
		}
		summary.importable++
		rows = append(rows, row)
	}
	return rows, summary
}

func currentTextTooLarge(text string) bool {
	return len(text) > DefaultLimits().MaxTextBytes
}

func expiredClipboard(row mappedClipboard, current woxSnapshot) bool {
	if row.createdAt.IsZero() {
		return false
	}
	days := current.TextDays
	if row.text == "" {
		days = current.ImageDays
	}
	if days <= 0 {
		days = 90
		if row.text == "" {
			days = 3
		}
	}
	return current.now().Sub(row.createdAt) > time.Duration(days)*24*time.Hour
}

func clipboardExisting(row mappedClipboard, current woxSnapshot) (clipboardHit, bool) {
	if row.text != "" {
		hit, ok := current.ClipboardTexts[row.text]
		return hit, ok
	}
	if row.imageHash != "" {
		if hit, ok := current.ClipboardImages[row.imageHash]; ok {
			return hit, true
		}
	}
	hit, ok := current.ClipboardImages[row.imagePath]
	return hit, ok
}

func sortClipboardNewest(rows []mappedClipboard) {
	for i := 1; i < len(rows); i++ {
		item := rows[i]
		j := i
		for j > 0 && rows[j-1].createdAt.Before(item.createdAt) {
			rows[j] = rows[j-1]
			j--
		}
		rows[j] = item
	}
}

func trimPreview(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > 80 {
		return value[:80]
	}
	return value
}
