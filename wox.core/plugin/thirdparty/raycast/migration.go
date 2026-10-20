package raycast

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/plugin/thirdparty/migrate"
	"wox/plugin/thirdparty/raycast/brand"
	"wox/plugin/thirdparty/raycast/extension"
	"wox/setting"
	"wox/util"
	"wox/util/shell"
)

func init() {
	plugin.RegisterThirdParty(extension.Layer{})
	migrate.Register(migrationSource{})
}

type migrationSource struct{}

func (migrationSource) ID() string { return "raycast" }

func (migrationSource) Detect(ctx context.Context) (migrate.Installation, error) {
	location, version := detectInstall()
	if location == "" {
		return nil, nil
	}
	return &installation{location: location, version: version}, nil
}

// installation keeps one unlocked export. Reset drops it and invalidates an in-flight Unlock.
type installation struct {
	mu         sync.Mutex
	location   string
	version    string
	generation uint64
	cancel     context.CancelFunc
	data       *mappedExport
	limits     Limits
	// extensionRoot is normally resolved from the user's home; tests supply an isolated installation.
	extensionRoot string
	sessionCtx    context.Context
}

func (i *installation) ID() string   { return "raycast" }
func (i *installation) Name() string { return "Raycast" }
func (i *installation) Version() string {
	return i.version
}
func (i *installation) Location() string      { return i.location }
func (i *installation) Icon() common.WoxImage { return brand.Image() }
func (i *installation) Hotkey() string {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.data == nil {
		return ""
	}
	for _, row := range i.data.hotkeys {
		if row.kind == "main" && row.selectable {
			return row.hotkey
		}
	}
	return ""
}

func (i *installation) Plugins(ctx context.Context) ([]migrate.Plugin, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.data == nil {
		return nil, &ExportError{Code: "export_required"}
	}
	candidates, err := i.localExtensions(ctx, i.data.extensions)
	var plugins []migrate.Plugin
	for _, candidate := range candidates {
		plugins = append(plugins, candidate.Plugin)
	}
	return plugins, err
}

func (i *installation) Generation() uint64 {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.generation
}

func (i *installation) Recognize(path string) (bool, string) {
	return Recognize(path)
}

// Unlock decrypts path and retains the mapped rows until Reset.
func (i *installation) Unlock(ctx context.Context, path, passphrase string) error {
	i.mu.Lock()
	i.generation++
	generation := i.generation
	if i.cancel != nil {
		i.cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	i.cancel = cancel
	i.sessionCtx = ctx
	i.data = nil
	limits := i.limits
	i.mu.Unlock()
	limits = limits.withDefaults()

	started := time.Now()
	raw, err := DecryptFile(ctx, path, passphrase, limits)
	if err != nil {
		return err
	}
	payload, err := ParsePayload(raw, limits)
	wipe(raw)
	if err != nil {
		return err
	}
	mapped := buildMapped(payload, currentSnapshot())
	payload = Payload{}

	i.mu.Lock()
	defer i.mu.Unlock()
	if generation != i.generation {
		return context.Canceled
	}
	i.data = &mapped
	util.GetLogger().Info(ctx, "raycast export unlocked entries="+strconv.Itoa(mappedCount(mapped))+" elapsed="+time.Since(started).String())
	return nil
}

// Reset cancels an in-flight unlock and drops the retained export. It is safe to call more than once.
func (i *installation) Reset() {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.generation++
	if i.cancel != nil {
		i.cancel()
		i.cancel = nil
	}
	i.data = nil
}

// SetKeepKeyword updates one snippet preview. The catalog index is only a session id.
func (i *installation) SetKeepKeyword(itemID string, keep bool) bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.data == nil {
		return false
	}
	for index := range i.data.snippets {
		row := &i.data.snippets[index]
		if row.id != itemID {
			continue
		}
		row.keepKeyword = keep
		applySnippetState(row, currentSnapshot())
		if row.keyword != "" && row.status == "" {
			row.detailKey = "raycast_snippet_keyword"
		}
		return true
	}
	return false
}

func (i *installation) Catalog(ctx context.Context) ([]migrate.Category, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.data == nil {
		return nil, &ExportError{Code: "export_required"}
	}
	candidates, err := i.localExtensions(ctx, i.data.extensions)
	if err != nil {
		return nil, err
	}
	categories := catalogFrom(*i.data)
	if len(candidates) > 0 {
		items := make([]migrate.Item, 0, len(candidates))
		for _, candidate := range candidates {
			items = append(items, migrate.Item{ID: "plugin:" + candidate.ID, Kind: migrate.ItemKindPlugin,
				Title: candidate.Name, Icon: candidate.Icon, DetailKey: candidate.Detail,
				Selectable: candidate.Selectable, Status: candidate.Status})
		}
		categories = append(categories, migrate.Category{ID: migrate.CategoryPlugins, Items: items})
	}
	return categories, nil
}

// CloseRunning quits Raycast so the global shortcuts being imported can be registered.
// Raycast.UIAccess.exe is the elevated helper that can own those shortcuts.
func (i *installation) CloseRunning(ctx context.Context) error {
	return shell.CloseNamedProcesses(ctx, "Raycast.exe", "Raycast.UIAccess.exe", "Raycast")
}

func (i *installation) Import(ctx context.Context, itemIDs []string) (migrate.ImportResult, error) {
	i.mu.Lock()
	if i.data == nil {
		i.mu.Unlock()
		return migrate.ImportResult{}, &ExportError{Code: "export_required"}
	}
	result := importSelected(*i.data, itemIDs)
	refs := append([]ExportExtension(nil), i.data.extensions...)
	sessionCtx := i.sessionCtx
	i.mu.Unlock()
	// Disk copies must not hold the session lock: Reset needs to cancel an in-flight import.
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	if sessionCtx != nil {
		stop := context.AfterFunc(sessionCtx, cancel)
		defer stop()
		if sessionCtx.Err() != nil {
			cancel()
		}
	}
	results, err := i.importExtensions(ctx, refs, itemIDs)
	result.Plugins = results
	return result, err
}

func mappedCount(mapped mappedExport) int {
	return len(mapped.hotkeys) + len(mapped.shortcuts) + len(mapped.queries) + len(mapped.general) + len(mapped.fileSearch) + len(mapped.quicklinks) + len(mapped.snippets) + len(mapped.notes) + len(mapped.clipboard) + len(mapped.clipboardSettings) + len(mapped.extensions)
}

func importSelected(mapped mappedExport, itemIDs []string) migrate.ImportResult {
	selected := map[string]struct{}{}
	for _, id := range itemIDs {
		selected[strings.TrimSpace(id)] = struct{}{}
	}
	var result migrate.ImportResult
	var hotkeyRows []setting.QueryHotkey
	var aliasRows []setting.QueryAlias
	var hotkeyIDs []string
	var aliasIDs []string
	for _, row := range mapped.hotkeys {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		if row.kind == "main" {
			result.Settings = append(result.Settings, migrate.SettingWrite{ItemIDs: []string{row.id}, Key: "MainHotkey", Value: row.hotkey})
			continue
		}
		hotkeyIDs = append(hotkeyIDs, row.id)
		hotkeyRows = append(hotkeyRows, setting.QueryHotkey{Hotkey: row.hotkey, Query: row.query})
	}
	if len(hotkeyRows) > 0 {
		raw, err := json.Marshal(hotkeyRows)
		if err == nil {
			result.Settings = append(result.Settings, migrate.SettingWrite{ItemIDs: hotkeyIDs, Key: "QueryHotkeys", Value: string(raw)})
		}
	}
	for _, row := range mapped.queries {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		aliasIDs = append(aliasIDs, row.id)
		aliasRows = append(aliasRows, setting.QueryAlias{Alias: row.alias, Query: row.query})
	}
	if len(aliasRows) > 0 {
		raw, err := json.Marshal(aliasRows)
		if err == nil {
			result.Settings = append(result.Settings, migrate.SettingWrite{ItemIDs: aliasIDs, Key: "QueryAliases", Value: string(raw)})
		}
	}
	for _, row := range mapped.general {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		result.Settings = append(result.Settings, migrate.SettingWrite{ItemIDs: []string{row.id}, Key: row.key, Value: row.value, PluginID: row.pluginID})
	}
	result.Settings = append(result.Settings, fileSearchSettingWrites(mapped, selected)...)
	seenFavorites := map[string]struct{}{}
	seenQuicklinks := map[string]struct{}{}
	for _, row := range mapped.quicklinks {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		if row.favorite {
			seenFavorites[row.id] = struct{}{}
			result.Favorites = append(result.Favorites, migrate.FavoriteWrite{ItemID: row.id, Name: row.title, Path: row.path})
			continue
		}
		seenQuicklinks[row.id] = struct{}{}
		result.Quicklinks = append(result.Quicklinks, migrate.QuicklinkWrite{
			ItemID: row.id, Title: row.title, Keyword: row.keyword, URL: row.url, Browser: row.browser,
		})
	}
	for _, row := range mapped.shortcuts {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		if row.prerequisiteID != "" && row.prerequisiteFavorite {
			if _, ok := seenFavorites[row.prerequisiteID]; !ok {
				seenFavorites[row.prerequisiteID] = struct{}{}
				result.Favorites = append(result.Favorites, migrate.FavoriteWrite{ItemID: row.prerequisiteID, Name: row.prerequisiteName, Path: row.prerequisitePath})
			}
		}
		if row.prerequisiteID != "" && !row.prerequisiteFavorite {
			if _, ok := seenQuicklinks[row.prerequisiteID]; !ok {
				seenQuicklinks[row.prerequisiteID] = struct{}{}
				result.Quicklinks = append(result.Quicklinks, migrate.QuicklinkWrite{
					ItemID: row.prerequisiteID, Title: row.prerequisiteName, Keyword: row.prerequisiteKeyword, URL: row.prerequisiteURL, Browser: row.prerequisiteBrowser,
				})
			}
		}
		result.ResultBindings = append(result.ResultBindings, migrate.ResultBindingWrite{
			ItemID: row.id, PluginID: row.pluginID, Search: row.search, Command: row.command, TriggerKeyword: row.trigger,
			IdentityKey: row.identityKey, Path: row.path, Title: row.matchTitle, Keyword: row.keyword,
			RequireSettingsPath: row.requireSettings, Alias: row.alias, Hotkey: row.hotkey,
		})
	}
	for _, row := range mapped.snippets {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		result.Notes = append(result.Notes, migrate.NoteWrite{
			ItemID: row.id, SourceID: row.sourceID,
			Fingerprint: snippetFingerprint(row.sourceID, row.title, row.text, row.keyword, row.keepKeyword),
			Title:       row.title, Body: snippetBody(row.text, row.keyword, row.keepKeyword),
		})
	}
	for _, row := range mapped.notes {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		result.Notes = append(result.Notes, migrate.NoteWrite{
			ItemID: row.id, SourceID: row.sourceID,
			Fingerprint: noteFingerprint(row.sourceID, row.title, row.body),
			Title:       row.title, Body: row.body, Document: row.document,
		})
	}
	for _, row := range mapped.clipboardSettings {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		result.Settings = append(result.Settings, migrate.SettingWrite{
			ItemIDs: []string{row.id}, Key: row.key, Value: row.value, PluginID: row.pluginID,
		})
	}
	_, summarySelected := selected[migrate.ClipboardSummaryID]
	for _, row := range mapped.clipboard {
		if !row.selectable {
			continue
		}
		if _, ok := selected[row.id]; !ok && !summarySelected {
			continue
		}
		result.Clipboard = append(result.Clipboard, migrate.ClipboardWrite{
			ItemID: row.id, Text: row.text, ImagePath: row.imagePath, FilePaths: append([]string(nil), row.filePaths...),
			OCRText: row.ocrText, CreatedAt: row.createdAt, Pinned: row.pinned,
		})
	}
	return result
}
