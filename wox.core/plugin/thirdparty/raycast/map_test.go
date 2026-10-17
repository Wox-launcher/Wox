package raycast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
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
	if runtime.GOOS == "windows" {
		extended := json.RawMessage(`{"kind":{"type":"SingleStep","shortcut":{"modifiers":[{"modifier":"Ctrl"},{"modifier":"Shift"}],"key":{"type":"LayoutIndependent","code":57416}}},"locality":"Global"}`)
		value, detailKey, _, ok = mapHotkey(extended)
		if !ok || value != "ctrl+shift+up" || detailKey != "" {
			t.Fatalf("extended = %s %s %v", value, detailKey, ok)
		}
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
	if len(mapped.queries) != 0 || len(mapped.shortcuts) != 1 || !mapped.shortcuts[0].selectable || mapped.shortcuts[0].alias != "nt" || mapped.shortcuts[0].path != app || mapped.shortcuts[0].pluginID != common.AppPluginID {
		t.Fatalf("shortcut = %+v queries=%+v", mapped.shortcuts, mapped.queries)
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

func TestMapNotesKeepsStructureAndSkipsEmpty(t *testing.T) {
	raw := json.RawMessage(`{
		"type":"doc",
		"content":[
			{"type":"heading","attrs":{"level":1},"content":[{"type":"text","text":"Plan"}]},
			{"type":"paragraph","content":[
				{"type":"text","text":"See "},
				{"type":"text","text":"docs","marks":[{"type":"bold"},{"type":"link","attrs":{"href":"https://example.com"}}]}
			]},
			{"type":"list","attrs":{"kind":"bullet"},"content":[{"type":"paragraph","content":[{"type":"text","text":"one"}]}]},
			{"type":"list","attrs":{"kind":"task","checked":true},"content":[{"type":"paragraph","content":[{"type":"text","text":"done"}]}]},
			{"type":"blockquote","content":[{"type":"paragraph","content":[{"type":"text","text":"quoted"}]}]},
			{"type":"horizontalRule"},
			{"type":"heading","attrs":{"level":2},"content":[{"type":"text","text":"Next"}]}
		]
	}`)
	payload := Payload{}
	payload.Notes.Notes = []Note{
		{ID: "empty"},
		{ID: "rich", Title: "Plan", Text: "Plan", RawContent: raw},
		{ID: "preview", Title: "", Text: "only a preview"},
	}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.notes) != 2 {
		t.Fatalf("notes = %+v", mapped.notes)
	}
	rich := mapped.notes[0]
	if rich.id != "note:1" || rich.title != "Plan" || !rich.selectable || rich.sourceID != "rich" {
		t.Fatalf("rich = %+v", rich)
	}
	blocks := rich.document.Blocks
	if len(blocks) != 7 || blocks[0].Type != common.NoteBlockHeading1 || blocks[0].Text != "Plan" {
		t.Fatalf("blocks = %+v", blocks)
	}
	if blocks[1].Text != "See docs" || len(blocks[1].Spans) != 1 || !blocks[1].Spans[0].Bold || blocks[1].Spans[0].Link != "https://example.com" || blocks[1].Spans[0].Start != 4 {
		t.Fatalf("inline = %+v", blocks[1])
	}
	if blocks[2].Type != common.NoteBlockBullet || blocks[3].Type != common.NoteBlockTask || !blocks[3].Checked || blocks[4].Type != common.NoteBlockQuote || blocks[5].Type != common.NoteBlockDivider || blocks[6].Type != common.NoteBlockHeading2 {
		t.Fatalf("structure = %+v", blocks)
	}
	if rich.body != "See docs\n• one\n☑ done\n> quoted\n────────\nNext" {
		t.Fatalf("body = %q", rich.body)
	}
	if mapped.notes[1].title != "only a preview" || mapped.notes[1].document.Blocks[0].Type != common.NoteBlockParagraph {
		t.Fatalf("preview = %+v", mapped.notes[1])
	}
	again := buildMapped(payload, woxSnapshot{Notes: []noteRow{{
		Fingerprint: noteFingerprint(rich.sourceID, rich.title, rich.body), Body: rich.body,
	}}})
	if again.notes[0].selectable || again.notes[0].status != statusExists {
		t.Fatalf("repeat = %+v", again.notes[0])
	}
	imported := importSelected(mapped, []string{rich.id})
	if len(imported.Notes) != 1 || imported.Notes[0].Document.Blocks[2].Text != "one" {
		t.Fatalf("import = %+v", imported.Notes)
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

func TestMapShortcutTargets(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(filePath, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := Payload{}
	payload.Quicklinks.Quicklinks = []Quicklink{
		{Name: "wox", Link: "https://github.com/Wox-launcher/Wox"},
		{Name: "notes", Link: filePath},
	}
	payload.Settings.Commands = []Command{
		{ExtensionID: "e:r:raycast", ID: "settings", Name: "Settings", Alias: "hw"},
		{ExtensionID: "e:r:raycast", ID: "send-focused-window", Name: "Send Focused Window to AI", Alias: "ai"},
		{ExtensionID: "e:r:system", ID: "volume-up", Name: "Turn Volume Up", Alias: "vu"},
		{ExtensionID: "e:r:system", ID: "play-pause", Name: "Play / Pause", Alias: "pp"},
		{ExtensionID: "e:r:windows-settings", ID: "ms-settings:display", Name: "Display", Alias: "fdj"},
		{ExtensionID: "e:r:windows-settings", ID: "ms-settings:easeofaccess-magnifier", Name: "使用放大镜放大大部分屏幕", Alias: "mag"},
		{ExtensionID: "e:r:quicklinks", ID: "wox", Name: "wox", Alias: "www"},
		{ExtensionID: "e:r:quicklinks", ID: "notes", Name: "notes", Alias: "nt"},
		{ExtensionID: "e:r:clipboard-history", ID: "clipboard", Name: "Clipboard History", MacOSHotkey: json.RawMessage(`{"kind":{"shortcut":{"key":{"type":"LayoutIndependent","code":` + spaceKeyCode() + `},"modifiers":[{"modifier":"Alt"}]}}}`)},
	}
	mapped := buildMapped(payload, defaultSnapshot())
	byTitle := map[string]mappedShortcut{}
	for _, row := range mapped.shortcuts {
		if !row.selectable {
			t.Fatalf("hidden shortcut listed = %+v", row)
		}
		byTitle[row.title] = row
	}
	settings := byTitle["Settings"]
	if settings.pluginID != common.SysPluginID || settings.command != "open_wox_settings" || settings.alias != "hw" {
		t.Fatalf("settings = %+v", settings)
	}
	volume := byTitle["Turn Volume Up"]
	if volume.command != "volume-up" || volume.alias != "vu" {
		t.Fatalf("volume = %+v", volume)
	}
	display := byTitle["Display"]
	if display.pluginID != common.AppPluginID || display.path != "ms-settings:display" {
		t.Fatalf("display = %+v", display)
	}
	link := byTitle["wox"]
	if link.pluginID != common.WebSearchPluginID || link.trigger != "wox" || link.alias != "www" || link.prerequisiteID == "" {
		t.Fatalf("link = %+v", link)
	}
	file := byTitle["notes"]
	if file.pluginID != common.FolderPluginID || file.path != filePath || !file.prerequisiteFavorite {
		t.Fatalf("file = %+v", file)
	}
	pause := byTitle["Play / Pause"]
	if pause.pluginID != common.MediaPlayerPluginID || pause.search != "playpause" || pause.identityKey != common.MediaPlayerToggleIdentity || pause.alias != "pp" {
		t.Fatalf("play pause = %+v", pause)
	}
	for _, title := range []string{"Send Focused Window to AI", "使用放大镜放大大部分屏幕", "Clipboard History"} {
		if _, ok := byTitle[title]; ok {
			t.Fatalf("%s was mapped", title)
		}
	}
	for _, row := range mapped.queries {
		if row.selectable {
			t.Fatalf("query alias = %+v", row)
		}
	}
	var clipboardHotkey bool
	for _, row := range mapped.hotkeys {
		if row.kind == "query" && row.selectable && row.title == "Clipboard History" {
			clipboardHotkey = true
		}
	}
	if !clipboardHotkey {
		t.Fatalf("clipboard hotkey = %+v", mapped.hotkeys)
	}
	imported := importSelected(mapped, []string{link.id, file.id})
	if len(imported.Quicklinks) != 1 || imported.Quicklinks[0].Keyword != "wox" || len(imported.Favorites) != 1 || imported.Favorites[0].Path != filePath {
		t.Fatalf("prerequisites = %+v %+v", imported.Quicklinks, imported.Favorites)
	}
	if len(imported.ResultBindings) != 2 || imported.ResultBindings[0].Alias != "www" || imported.ResultBindings[1].Alias != "nt" {
		t.Fatalf("bindings = %+v", imported.ResultBindings)
	}
	both := importSelected(mapped, []string{link.prerequisiteID, link.id})
	if len(both.Quicklinks) != 1 || len(both.ResultBindings) != 1 {
		t.Fatalf("dedup = %+v %+v", both.Quicklinks, both.ResultBindings)
	}
}

func TestMapExportedCommandIDs(t *testing.T) {
	const magnifier = `::{26EE0668-A00A-44D7-9371-BEB064C98683}\0\::{ED7BA470-8E54-465E-825C-99712043E01C}\{3D8638E3-EC96-4CE8-8279-36B0065B5F2C}`
	settingsAppsForTest = func() []plugin.DesktopApp {
		return []plugin.DesktopApp{{Name: "使用放大镜放大部分屏幕", Path: magnifier}}
	}
	t.Cleanup(func() { settingsAppsForTest = nil })

	payload := Payload{}
	payload.Quicklinks.Quicklinks = []Quicklink{{
		ID: "01M1VBS91J2HW3SRPW8Y46WGFN", Name: "wox", Link: "https://github.com/Wox-launcher/Wox",
	}}
	payload.Settings.Commands = []Command{
		{ExtensionID: "e:r:applications", ID: "c:r:applications::*::application::=::/Applications/94f60fda-9dce-5346-8267-287b6affb1c0", Alias: "wp"},
		{ExtensionID: "e:r:raycast-core", ID: "c:r:raycast-core::-::openSettings", Alias: "hw"},
		{ExtensionID: "e:r:quicklinks", ID: "c:r:quicklinks::*::quicklink::=::01M1VBS91J2HW3SRPW8Y46WGFN", Alias: "www", WindowsHotkey: json.RawMessage(`{"kind":{"type":"SingleStep","shortcut":{"modifiers":[{"modifier":"Ctrl"},{"modifier":"Shift"}],"key":{"type":"LayoutIndependent","code":24}}},"locality":"Global"}`)},
		{ExtensionID: "e:r:system", ID: "c:r:system::-::turnVolumeUp", WindowsHotkey: json.RawMessage(`{"kind":{"type":"SingleStep","shortcut":{"modifiers":[{"modifier":"Ctrl"},{"modifier":"Shift"}],"key":{"type":"LayoutIndependent","code":57416}}},"locality":"Global"}`)},
		{ExtensionID: "e:r:system", ID: "c:r:system::-::playPause", Alias: "pp"},
		{ExtensionID: "e:r:preferences", ID: `c:r:preferences::*::preference::=::/ControlPanel/::{26EE0668-A00A-44D7-9371-BEB064C98683}\0\::{ED7BA470-8E54-465E-825C-99712043E01C}\{3D8638E3-EC96-4CE8-8279-36B0065B5F2C}`, Alias: "fdj", WindowsHotkey: json.RawMessage(`{"kind":{"type":"SingleStep","shortcut":{"modifiers":[{"modifier":"Ctrl"},{"modifier":"Shift"}],"key":{"type":"LayoutIndependent","code":23}}},"locality":"Global"}`)},
		{ExtensionID: "e:r:preferences", ID: `c:r:preferences::*::preference::=::/ControlPanel/{00000000-0000-0000-0000-000000000001}`, Alias: "missing"},
		{ExtensionID: "e:r:screen-awareness", ID: "c:r:screen-awareness::-::sendFocusedWindow", WindowsHotkey: json.RawMessage(`{"kind":{"type":"DoubleTap","shortcut":{"modifier":{"modifier":"Ctrl","directionality":"right"}}},"locality":"Global"}`)},
	}
	mapped := buildMapped(payload, defaultSnapshot())
	byTitle := map[string]mappedShortcut{}
	for _, row := range mapped.shortcuts {
		byTitle[row.title] = row
	}
	settings := byTitle["Settings"]
	if settings.command != "open_wox_settings" || settings.alias != "hw" || settings.pluginID != common.SysPluginID {
		t.Fatalf("settings = %+v", settings)
	}
	link := byTitle["wox"]
	if link.alias != "www" || link.trigger != "wox" || link.pluginID != common.WebSearchPluginID {
		t.Fatalf("link = %+v", link)
	}
	if runtime.GOOS == "windows" && link.hotkey != "ctrl+shift+o" {
		t.Fatalf("link hotkey = %s", link.hotkey)
	}
	pause := byTitle["Play / Pause"]
	if pause.alias != "pp" || pause.pluginID != common.MediaPlayerPluginID || pause.identityKey != common.MediaPlayerToggleIdentity || pause.search != "playpause" {
		t.Fatalf("play pause = %+v", pause)
	}
	magnifierRow := byTitle["使用放大镜放大部分屏幕"]
	if magnifierRow.alias != "fdj" || magnifierRow.pluginID != common.AppPluginID || magnifierRow.path != magnifier || magnifierRow.identityKey != magnifier || magnifierRow.search != "使用放大镜放大部分屏幕" {
		t.Fatalf("control panel = %+v", magnifierRow)
	}
	want := 4
	if runtime.GOOS == "windows" {
		want = 5
		volume := byTitle["Turn Volume Up"]
		if volume.command != "volume-up" || volume.hotkey != "ctrl+shift+up" {
			t.Fatalf("volume = %+v", volume)
		}
		if magnifierRow.hotkey != "ctrl+shift+i" {
			t.Fatalf("control panel hotkey = %s", magnifierRow.hotkey)
		}
	}
	for _, title := range []string{"wp", "e:r:preferences"} {
		if _, ok := byTitle[title]; ok {
			t.Fatalf("%s was mapped", title)
		}
	}
	if len(byTitle) != want {
		t.Fatalf("shortcuts = %+v", mapped.shortcuts)
	}
}

func TestRaycastShortcutIDMatchesIndexedExecutable(t *testing.T) {
	const executable = `C:\Users\qianl\AppData\Roaming\baidu\BaiduNetdisk\BaiduNetdisk.exe`
	if got := raycastShortcutID(executable, ""); got != "94f60fda-9dce-5346-8267-287b6affb1c0" {
		t.Fatalf("id = %s", got)
	}
	if raycastShortcutID(executable, "-extra") == raycastShortcutID(executable, "") {
		t.Fatal("arguments left the id unchanged")
	}
	id, ok := raycastAppUUID("/Applications/94f60fda-9dce-5346-8267-287b6affb1c0")
	if !ok || id != "94f60fda-9dce-5346-8267-287b6affb1c0" {
		t.Fatalf("uuid = %s %v", id, ok)
	}
	if _, ok = raycastAppUUID(`/Applications/Notes.app`); ok {
		t.Fatal("bundle path parsed as a uuid")
	}
}

func TestMapApplicationUUID(t *testing.T) {
	const executable = `C:\Users\qianl\AppData\Roaming\baidu\BaiduNetdisk\BaiduNetdisk.exe`
	lnk := `C:\Users\qianl\AppData\Roaming\Microsoft\Windows\Start Menu\Programs\百度网盘\百度网盘.lnk`
	desktopAppsForTest = func() []plugin.DesktopApp {
		return []plugin.DesktopApp{{
			Name: "百度网盘", Path: lnk, Executable: executable,
		}, {
			Name: "Other", Path: `C:\Other\Other.lnk`, Executable: `C:\Other\Other.exe`, Arguments: "-x",
		}}
	}
	t.Cleanup(func() { desktopAppsForTest = nil })

	payload := Payload{}
	payload.Settings.Commands = []Command{{
		ExtensionID: "e:r:applications",
		ID:          "c:r:applications::*::application::=::/Applications/94f60fda-9dce-5346-8267-287b6affb1c0",
		Alias:       "wp",
	}, {
		ExtensionID: "e:r:applications",
		ID:          "c:r:applications::*::application::=::/Applications/00000000-0000-4000-8000-000000000001",
		Alias:       "missing",
	}}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.shortcuts) != 1 {
		t.Fatalf("shortcuts = %+v", mapped.shortcuts)
	}
	row := mapped.shortcuts[0]
	if !row.selectable || row.title != "百度网盘" || row.alias != "wp" || row.path != lnk || row.pluginID != common.AppPluginID || row.search != "百度网盘" {
		t.Fatalf("row = %+v", row)
	}
	if len(mapped.queries) != 1 || mapped.queries[0].selectable || mapped.queries[0].alias != "missing" {
		t.Fatalf("queries = %+v", mapped.queries)
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
