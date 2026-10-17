package raycast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"wox/plugin/thirdparty/migrate"
)

func TestMapGlobalHotkey(t *testing.T) {
	raw := json.RawMessage(`{"kind":{"shortcut":{"key":{"type":"LayoutIndependent","code":` + spaceKeyCode() + `},"modifiers":[{"modifier":"Alt"}]}}}`)
	value, detailKey, _, ok := mapHotkey(raw)
	if !ok || value != "alt+space" || detailKey != "" {
		t.Fatalf("hotkey = %s %s %v", value, detailKey, ok)
	}
	unknown := json.RawMessage(`{"kind":{"shortcut":{"key":{"type":"LayoutIndependent","code":49},"modifiers":[{"modifier":"Function"}]}}}`)
	if _, detailKey, _, ok = mapHotkey(unknown); ok || detailKey != "raycast_unknown_modifier" {
		t.Fatalf("unknown modifier ok=%v key=%s", ok, detailKey)
	}
}

func TestMainHotkeyStaysListedWhenTheCurrentShortcutMatches(t *testing.T) {
	payload := Payload{}
	payload.Settings.General = map[string]json.RawMessage{
		"globalHotkey": json.RawMessage(`{"kind":{"shortcut":{"key":{"type":"LayoutIndependent","code":` + spaceKeyCode() + `},"modifiers":[{"modifier":"Ctrl"},{"modifier":"Alt"}]}}}`),
	}
	current := defaultSnapshot()
	current.MainHotkey = "ctrl+alt+space"
	mapped := buildMapped(payload, current)
	if len(mapped.hotkeys) != 1 || !mapped.hotkeys[0].selectable || mapped.hotkeys[0].hotkey != "ctrl+alt+space" || mapped.hotkeys[0].detail != "Ctrl + Alt + Space" {
		t.Fatalf("main hotkey = %+v", mapped.hotkeys)
	}
	catalog := catalogFrom(mapped)
	if len(catalog) != 1 || catalog[0].ID != migrate.CategoryHotkeys || catalog[0].Items[0].ID != "setting:main_hotkey" {
		t.Fatalf("catalog = %+v", catalog)
	}
}

func spaceKeyCode() string {
	if runtime.GOOS == "windows" {
		return "57"
	}
	return "49"
}

func TestMapApplicationAlias(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "Notes.exe")
	if err := os.WriteFile(app, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := Payload{}
	payload.Settings.Commands = []Command{{
		ExtensionID: "e:r:applications",
		ID:          "open::=::" + app,
		Name:        "Notes",
		Alias:       "nt extra",
		MacOSHotkey: json.RawMessage(`{"kind":{"shortcut":{"key":{"type":"LayoutIndependent","code":49},"modifiers":[{"modifier":"Alt"}]}}}`),
	}}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.queries) != 1 || !mapped.queries[0].selectable || mapped.queries[0].alias != "nt" || mapped.queries[0].query != "app Notes" {
		t.Fatalf("alias = %+v", mapped.queries)
	}
	missing := payload
	missing.Settings.Commands[0].ID = "open::=::" + filepath.Join(dir, "gone.exe")
	mapped = buildMapped(missing, defaultSnapshot())
	if mapped.queries[0].selectable || mapped.queries[0].detailKey != "raycast_app_missing" {
		t.Fatalf("missing = %+v", mapped.queries[0])
	}
}

func TestMapQuicklinkPlaceholders(t *testing.T) {
	link, ok := rewriteQuicklinkLink("https://example.com/search?q={Query}")
	if !ok || link != "https://example.com/search?q={wox:parameter?name=query}" {
		t.Fatalf("query = %s %v", link, ok)
	}
	link, ok = rewriteQuicklinkLink(`https://example.com/?q={argument name="q"}`)
	if !ok || link != "https://example.com/?q={wox:parameter?name=q}" {
		t.Fatalf("named = %s %v", link, ok)
	}
	if _, ok = rewriteQuicklinkLink("https://example.com/?q={clipboard}"); ok {
		t.Fatal("accepted an unknown placeholder")
	}
	keyword, ok := quicklinkKeyword("Wox Smoke")
	if !ok || keyword != "woxsmoke" {
		t.Fatalf("keyword = %s %v", keyword, ok)
	}
}

func TestMapSnippetBodies(t *testing.T) {
	if snippetBody("hello", "", true) != "hello" {
		t.Fatal("empty keyword changed the body")
	}
	kept := snippetBody("hello {date}", "addr", true)
	if kept != "Keyword: addr\n\nhello {date}" || !snippetHasDynamic("hello {date}") {
		t.Fatalf("kept = %q", kept)
	}
	if snippetBody("hello", "addr", false) != "hello" {
		t.Fatal("disabled keyword leaked into the body")
	}
	payload := Payload{}
	payload.Snippets.Snippets = []Snippet{{Title: "", Text: "skip"}, {Title: "Address", Text: "hello", Keyword: "addr"}}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.snippets) != 1 || mapped.snippets[0].title != "Address" {
		t.Fatalf("snippets = %+v", mapped.snippets)
	}
	first := snippetFingerprint(mapped.snippets[0].sourceID, "Address", "hello", "addr", true)
	again := buildMapped(payload, woxSnapshot{Notes: []noteRow{{
		Title: "Address (2)", Body: snippetBody("hello", "addr", true), Fingerprint: first,
	}}})
	if again.snippets[0].selectable || again.snippets[0].status != statusExists {
		t.Fatalf("repeat = %+v", again.snippets[0])
	}
	edited := buildMapped(payload, woxSnapshot{Notes: []noteRow{{
		Title: "Address", Body: "user edited", Fingerprint: first,
	}}})
	if edited.snippets[0].status != statusConflict {
		t.Fatalf("edited = %+v", edited.snippets[0])
	}
	deleted := buildMapped(payload, woxSnapshot{Notes: []noteRow{{
		Title: "Address", Body: snippetBody("hello", "addr", true), Fingerprint: first, Deleted: true,
	}}})
	if !deleted.snippets[0].selectable {
		t.Fatal("deleted target did not allow a new note")
	}
}

func TestMapClipboardTextWinsAndPinnedFlag(t *testing.T) {
	entry := ClipboardEntry{
		CreatedAt: "2024-01-02T03:04:05.5Z",
		Pinned:    json.RawMessage(`"yes"`),
		Items: []struct {
			Representations []ClipboardRepresentation `json:"representations"`
		}{{Representations: []ClipboardRepresentation{
			{MimeType: "image/png", ContentType: "url", Content: `C:\missing.png`},
			{MimeType: "text/plain", Content: "hello"},
		}}},
	}
	text, image, missing := clipboardTextAndImage(entry)
	if text != "hello" || image != "" || missing {
		t.Fatalf("text preferred = %q %q %v", text, image, missing)
	}
	if clipboardPinned(entry.Pinned) {
		t.Fatal("non-boolean pin was treated as pinned")
	}
	created, ok := parseRaycastTime(entry.CreatedAt)
	if !ok || created.UTC().Format(time.RFC3339Nano) == "" {
		t.Fatal(created, ok)
	}
	payload := Payload{}
	payload.ClipboardHistory.Entries = []ClipboardEntry{{
		Items: []struct {
			Representations []ClipboardRepresentation `json:"representations"`
		}{{Representations: []ClipboardRepresentation{
			{MimeType: "image/png", ContentType: "url", Content: filepath.Join(t.TempDir(), "gone.png")},
		}}},
	}}
	mapped := buildMapped(payload, defaultSnapshot())
	if mapped.summary.missing != 1 || mapped.clipboard[0].selectable {
		t.Fatalf("missing = %+v %+v", mapped.summary, mapped.clipboard)
	}
}

func TestMapQuicklinkFilesAndFoldersBecomeFavorites(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "macchiato.toml")
	if err := os.WriteFile(filePath, []byte("name = \"macchiato\"\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	folderPath := filepath.Join(root, "vicinae")
	if err := os.Mkdir(folderPath, 0o755); err != nil {
		t.Fatalf("create folder: %v", err)
	}
	fileURL := "file:///" + filepath.ToSlash(filePath)
	payload := Payload{}
	payload.Quicklinks.Quicklinks = []Quicklink{
		{Name: "wox", Link: `https://github.com/Wox-launch?{argument name="test1"}`},
		{Name: "macchiato", Link: fileURL},
		{Name: "vicinaeqianlifeng", Link: folderPath},
		{Name: "gone", Link: filepath.Join(root, "missing.toml")},
	}
	mapped := buildMapped(payload, defaultSnapshot())
	var quicklinks []migrate.Item
	for _, category := range catalogFrom(mapped) {
		if category.ID == migrate.CategoryQuicklinks {
			quicklinks = category.Items
		}
	}
	if len(quicklinks) != 3 || quicklinks[0].Kind != migrate.ItemKindQuicklink || quicklinks[0].Title != "wox" {
		t.Fatalf("catalog = %+v", quicklinks)
	}
	if quicklinks[1].Kind != migrate.ItemKindFavorite || quicklinks[1].Title != "macchiato" || quicklinks[1].Detail != filePath {
		t.Fatalf("file = %+v want %s", quicklinks[1], filePath)
	}
	if quicklinks[2].Kind != migrate.ItemKindFavorite || quicklinks[2].Title != "vicinaeqianlifeng" || quicklinks[2].Detail != folderPath {
		t.Fatalf("folder = %+v", quicklinks[2])
	}
	imported := importSelected(mapped, []string{quicklinks[0].ID, quicklinks[1].ID, quicklinks[2].ID})
	if len(imported.Quicklinks) != 1 || imported.Quicklinks[0].Keyword != "wox" {
		t.Fatalf("quicklinks = %+v", imported.Quicklinks)
	}
	if len(imported.Favorites) != 2 || imported.Favorites[0].Name != "macchiato" || imported.Favorites[0].Path != filePath || imported.Favorites[1].Path != folderPath {
		t.Fatalf("favorites = %+v", imported.Favorites)
	}
	repeat := buildMapped(payload, woxSnapshot{Favorites: []favoriteRow{{Name: "macchiato", Path: filePath}}})
	for _, row := range repeat.quicklinks {
		if row.title == "macchiato" && (row.selectable || row.status != statusExists) {
			t.Fatalf("repeat file = %+v", row)
		}
		if row.title == "vicinaeqianlifeng" && !row.selectable {
			t.Fatalf("unrelated folder hidden = %+v", row)
		}
	}
}

func TestMapGeneralSettings(t *testing.T) {
	payload := Payload{}
	payload.Settings.General = map[string]json.RawMessage{
		"openAtLogin":   json.RawMessage(`true`),
		"showInMenuBar": json.RawMessage(`false`),
	}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.general) != 2 || mapped.general[0].key != "EnableAutostart" || mapped.general[0].value != "true" {
		t.Fatalf("general = %+v", mapped.general)
	}
	if mapped.general[1].key != "ShowTray" || mapped.general[1].value != "false" {
		t.Fatalf("tray = %+v", mapped.general[1])
	}
}
