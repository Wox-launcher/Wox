package raycast

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path"
	"strconv"
	"strings"

	"wox/common"
	"wox/database"
	"wox/setting"
	"wox/util"

	_ "github.com/mattn/go-sqlite3"
)

var snapshotForTest *woxSnapshot

func currentSnapshot() woxSnapshot {
	if snapshotForTest != nil {
		return *snapshotForTest
	}
	if utilIsTestMode() {
		return defaultSnapshot()
	}
	return readLiveSnapshot()
}

func defaultSnapshot() woxSnapshot {
	return woxSnapshot{
		ClipboardTrigger: "cb", EmojiTrigger: "emoji", TextDays: 90, ImageDays: 3, MaxHistory: 5000,
		ImageTextRecognition: true, ImageTextRecognitionKnown: true,
		ThemeID: setting.DefaultThemeId, ThemeKnown: true,
		ShowPosition: string(setting.PositionTypeMouseScreen), ShowPositionKnown: true,
		UIDensity: string(setting.UiDensityNormal), UIDensityKnown: true,
		DictationSound: true, DictationSoundKnown: true,
		DictationDuck: false, DictationDuckKnown: true,
		EmojiAction: "paste", EmojiActionKnown: true,
		ScreenshotOCR: true, ScreenshotOCRKnown: true,
		ScreenshotDays: 15, ScreenshotDaysKnown: true,
	}
}

func utilIsTestMode() bool {
	return util.IsTestMode()
}

func readLiveSnapshot() woxSnapshot {
	snap := defaultSnapshot()
	applyFileSearchDefaults(&snap)
	woxSetting := setting.GetSettingManager().GetWoxSetting(context.Background())
	if woxSetting == nil || woxSetting.MainHotkey == nil {
		return snap
	}
	snap.MainHotkey = woxSetting.MainHotkey.Get()
	snap.AutostartKnown = true
	snap.Autostart = woxSetting.EnableAutostart.Get()
	snap.ShowTrayKnown = true
	snap.ShowTray = woxSetting.ShowTray.Get()
	if woxSetting.ThemeId != nil {
		snap.ThemeID = woxSetting.ThemeId.Get()
		snap.ThemeKnown = true
	}
	if woxSetting.ShowPosition != nil {
		snap.ShowPosition = string(woxSetting.ShowPosition.Get())
		snap.ShowPositionKnown = true
	}
	if woxSetting.UiDensity != nil {
		snap.UIDensity = string(woxSetting.UiDensity.Get())
		snap.UIDensityKnown = true
	}
	if woxSetting.AIMCPServers != nil {
		snap.MCPServers = append([]common.AIChatMCPServerConfig(nil), woxSetting.AIMCPServers.Get()...)
	}
	if woxSetting.AISkills != nil {
		snap.Skills = append([]common.Skill(nil), woxSetting.AISkills.Get()...)
	}
	for _, row := range woxSetting.QueryHotkeys.Get() {
		snap.QueryHotkeys = append(snap.QueryHotkeys, queryRow{Hotkey: row.Hotkey, Query: row.Query})
	}
	for _, row := range woxSetting.QueryAliases.Get() {
		snap.QueryAliases = append(snap.QueryAliases, queryRow{Alias: row.Alias, Query: row.Query})
	}
	if database.GetDB() == nil {
		return snap
	}
	readWebSearchSnapshot(&snap)
	readFileSearchSnapshot(&snap)
	readFavoriteSnapshot(&snap)
	readNotesSnapshot(&snap)
	readClipboardSnapshot(&snap)
	readOverlapSnapshot(&snap)
	return snap
}

// readFileSearchSnapshot overlays saved file-search settings onto the default baseline.
func readFileSearchSnapshot(snap *woxSnapshot) {
	store := setting.NewPluginSettingStore(database.GetDB(), common.FileSearchPluginID)
	if raw, ok := pluginSettingString(store, "roots"); ok {
		if paths, err := parseImportedPathTable(raw); err == nil {
			snap.FileRoots = paths
			snap.ContentRoots = append([]string(nil), paths...)
		}
	}
	if raw, ok := pluginSettingString(store, "ignorePatterns"); ok {
		if patterns, err := parseImportedPatternTable(raw); err == nil {
			snap.FileIgnorePatterns = withMandatoryIgnore(patterns)
		}
	}
	if raw, ok := pluginSettingString(store, "contentRoots"); ok {
		if paths, err := parseImportedPathTable(raw); err == nil {
			snap.ContentRoots = paths
		}
	}
	if value, ok := pluginSettingBool(store, "skipHiddenFiles"); ok {
		snap.SkipHidden = value
	}
	if value, ok := pluginSettingBool(store, "showPreview"); ok {
		snap.ShowPreview = value
	}
	if value, ok := pluginSettingBool(store, "contentSearchEnabled"); ok {
		snap.ContentSearch = value
	}
}

func pluginSettingString(store *setting.PluginSettingStore, key string) (string, bool) {
	var raw string
	platformKey := key + "@" + util.GetCurrentPlatform()
	if err := store.Get(platformKey, &raw); err == nil && strings.TrimSpace(raw) != "" {
		return raw, true
	}
	raw = ""
	if err := store.Get(key, &raw); err != nil || strings.TrimSpace(raw) == "" {
		return "", false
	}
	return raw, true
}

func pluginSettingBool(store *setting.PluginSettingStore, key string) (bool, bool) {
	raw, ok := pluginSettingString(store, key)
	if !ok {
		return false, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, false
	}
	return value, true
}

func readWebSearchSnapshot(snap *woxSnapshot) {
	store := setting.NewPluginSettingStore(database.GetDB(), common.WebSearchPluginID)
	var raw string
	if err := store.Get("webSearches", &raw); err != nil || strings.TrimSpace(raw) == "" {
		return
	}
	var rows []struct {
		Keyword string
		Title   string
		Urls    []string
		Browser string
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return
	}
	for _, row := range rows {
		url := ""
		if len(row.Urls) > 0 {
			url = row.Urls[0]
		}
		snap.WebSearches = append(snap.WebSearches, webSearchRow{Keyword: row.Keyword, Title: row.Title, URL: url, Browser: row.Browser})
		if snap.OccupiedKeywords == nil {
			snap.OccupiedKeywords = map[string]struct{}{}
		}
		snap.OccupiedKeywords[strings.ToLower(row.Keyword)] = struct{}{}
	}
}

func readFavoriteSnapshot(snap *woxSnapshot) {
	store := setting.NewPluginSettingStore(database.GetDB(), common.FolderPluginID)
	var raw string
	platformKey := "favorites@" + util.GetCurrentPlatform()
	if err := store.Get(platformKey, &raw); err != nil || strings.TrimSpace(raw) == "" {
		raw = ""
		if err := store.Get("favorites", &raw); err != nil {
			return
		}
	}
	var rows []favoriteRow
	if json.Unmarshal([]byte(raw), &rows) != nil {
		return
	}
	snap.Favorites = rows
}

func readNotesSnapshot(snap *woxSnapshot) {
	store := setting.NewPluginSettingStore(database.GetDB(), common.NotesPluginID)
	values, err := store.ListByPrefix("note:")
	if err != nil {
		return
	}
	for _, raw := range values {
		var record common.NoteRecord
		if json.Unmarshal([]byte(raw), &record) != nil {
			continue
		}
		title, body := snapshotNoteText(record)
		snap.Notes = append(snap.Notes, noteRow{
			Title: title, Body: body, Fingerprint: record.ImportFingerprint, Deleted: record.DeletedAt != 0,
		})
	}
}

func snapshotNoteText(record common.NoteRecord) (title, body string) {
	return noteSnapshotText(record.Document)
}

func readClipboardSnapshot(snap *woxSnapshot) {
	store := setting.NewPluginSettingStore(database.GetDB(), common.ClipboardPluginID)
	var days string
	if err := store.Get("text_history_days", &days); err == nil && days != "" {
		if parsed, err := parseDays(days); err == nil {
			snap.TextDays = parsed
		}
	}
	if err := store.Get("image_history_days", &days); err == nil && days != "" {
		if parsed, err := parseDays(days); err == nil {
			snap.ImageDays = parsed
		}
	}
	var recognition string
	if err := store.Get("image_text_recognition_enabled", &recognition); err == nil && recognition != "" {
		if parsed, err := strconv.ParseBool(recognition); err == nil {
			snap.ImageTextRecognition = parsed
		}
	}
	snap.ImageTextRecognitionKnown = true
	if snap.ClipboardTexts == nil {
		snap.ClipboardTexts = map[string]clipboardHit{}
	}
	if snap.ClipboardImages == nil {
		snap.ClipboardImages = map[string]clipboardHit{}
	}
	if snap.ClipboardFiles == nil {
		snap.ClipboardFiles = map[string]clipboardHit{}
	}
	var favorites string
	if err := store.Get("favorites", &favorites); err == nil && favorites != "" {
		var items []struct {
			Type      string
			Content   string
			ImageHash *string
			FilePaths []string `json:"filePaths"`
		}
		if json.Unmarshal([]byte(favorites), &items) == nil {
			for _, item := range items {
				if item.Type == "text" && item.Content != "" {
					snap.ClipboardTexts[item.Content] = clipboardHit{Favorite: true}
				}
				if item.ImageHash != nil && *item.ImageHash != "" {
					snap.ClipboardImages[*item.ImageHash] = clipboardHit{Favorite: true}
				}
				if key := clipboardFileKey(item.FilePaths); key != "" {
					snap.ClipboardFiles[key] = clipboardHit{Favorite: true}
				}
			}
		}
	}
	dbPath := path.Join(util.GetLocation().GetPluginSettingDirectory(), common.ClipboardPluginID+"_clipboard.db")
	if _, err := os.Stat(dbPath); err != nil {
		return
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return
	}
	defer db.Close()
	query := `SELECT type, content, image_hash, file_paths FROM clipboard_history WHERE is_favorite = 0`
	rows, err := db.QueryContext(context.Background(), query)
	withFiles := true
	if err != nil {
		withFiles = false
		rows, err = db.QueryContext(context.Background(), `SELECT type, content, image_hash FROM clipboard_history WHERE is_favorite = 0`)
	}
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var kind, content string
		var hash sql.NullString
		var filePaths sql.NullString
		var scanErr error
		if withFiles {
			scanErr = rows.Scan(&kind, &content, &hash, &filePaths)
		} else {
			scanErr = rows.Scan(&kind, &content, &hash)
		}
		if scanErr != nil {
			continue
		}
		snap.HistoryCount++
		if kind == "text" && content != "" {
			if _, exists := snap.ClipboardTexts[content]; !exists {
				snap.ClipboardTexts[content] = clipboardHit{}
			}
		}
		if hash.Valid && hash.String != "" {
			if _, exists := snap.ClipboardImages[hash.String]; !exists {
				snap.ClipboardImages[hash.String] = clipboardHit{}
			}
		}
		if filePaths.Valid {
			var paths []string
			if json.Unmarshal([]byte(filePaths.String), &paths) == nil {
				if key := clipboardFileKey(paths); key != "" {
					if _, exists := snap.ClipboardFiles[key]; !exists {
						snap.ClipboardFiles[key] = clipboardHit{}
					}
				}
			}
		}
	}
}

func readOverlapSnapshot(snap *woxSnapshot) {
	if raw, ok := pluginSettingString(setting.NewPluginSettingStore(database.GetDB(), common.AppPluginID), "AppDirectories"); ok {
		if paths, err := parseImportedPathTable(raw); err == nil {
			snap.AppDirectories = paths
		}
	}
	dictation := setting.NewPluginSettingStore(database.GetDB(), common.DictationPluginID)
	if value, ok := pluginSettingBool(dictation, "playSound"); ok {
		snap.DictationSound = value
		snap.DictationSoundKnown = true
	}
	if value, ok := pluginSettingBool(dictation, "duckVolume"); ok {
		snap.DictationDuck = value
		snap.DictationDuckKnown = true
	}
	if raw, ok := pluginSettingString(dictation, "dictionary"); ok {
		var entries []struct {
			Phrase string `json:"phrase"`
		}
		if json.Unmarshal([]byte(raw), &entries) == nil {
			for _, entry := range entries {
				if phrase := strings.TrimSpace(entry.Phrase); phrase != "" {
					snap.DictationPhrases = append(snap.DictationPhrases, phrase)
				}
			}
		}
	}
	emoji := setting.NewPluginSettingStore(database.GetDB(), common.EmojiPluginID)
	if raw, ok := pluginSettingString(emoji, "primary_action"); ok {
		snap.EmojiAction = strings.ToLower(strings.TrimSpace(raw))
		snap.EmojiActionKnown = snap.EmojiAction == "copy" || snap.EmojiAction == "paste"
	}
	if raw, ok := pluginSettingString(emoji, "frequentlyUsed"); ok {
		var usages []struct {
			Emoji string `json:"emoji"`
		}
		if json.Unmarshal([]byte(raw), &usages) == nil {
			for _, usage := range usages {
				if emoji := strings.TrimSpace(usage.Emoji); emoji != "" {
					snap.EmojiUsed = append(snap.EmojiUsed, emoji)
				}
			}
		}
	}
	if raw, ok := pluginSettingString(emoji, "customDescriptions"); ok {
		var descriptions map[string][]string
		if json.Unmarshal([]byte(raw), &descriptions) == nil {
			snap.EmojiKeywords = descriptions
		}
	}
	screenshots := setting.NewPluginSettingStore(database.GetDB(), common.ScreenshotPluginID)
	if value, ok := pluginSettingBool(screenshots, "ocr_enabled"); ok {
		snap.ScreenshotOCR = value
		snap.ScreenshotOCRKnown = true
	}
	if raw, ok := pluginSettingString(screenshots, "retention_days"); ok {
		if days, err := parseDays(raw); err == nil && days > 0 {
			snap.ScreenshotDays = days
			snap.ScreenshotDaysKnown = true
		}
	}
	if raw, ok := pluginSettingString(setting.NewPluginSettingStore(database.GetDB(), common.ClipboardPluginID), "ignored_applications"); ok {
		var rows []struct {
			App ignoredClipboardApp `json:"App"`
		}
		if json.Unmarshal([]byte(raw), &rows) == nil {
			for _, row := range rows {
				if strings.TrimSpace(row.App.Identity) != "" {
					snap.IgnoredClipboardApps = append(snap.IgnoredClipboardApps, row.App)
				}
			}
		}
	}
}

func parseDays(value string) (int, error) {
	return strconv.Atoi(strings.TrimSpace(value))
}
