package system

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
)

type imagePasteFailureAPI struct {
	plugin.API
	notification string
	logs         []string
}

func (a *imagePasteFailureAPI) GetSetting(context.Context, string) string { return "" }

func (a *imagePasteFailureAPI) GetTranslation(_ context.Context, key string) string {
	return "translated:" + key
}

func (a *imagePasteFailureAPI) Log(_ context.Context, _ plugin.LogLevel, message string) {
	a.logs = append(a.logs, message)
}

func (a *imagePasteFailureAPI) Notify(_ context.Context, message string) {
	a.notification = message
}

// TestImagePasteFailureUsesTranslatedNotification exercises the shared paste action's error display.
func TestImagePasteFailureUsesTranslatedNotification(t *testing.T) {
	api := &imagePasteFailureAPI{}
	c := &ClipboardPlugin{api: api, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	c.imageCache.Store("missing-image", &ImageCacheEntry{})
	result := c.convertImageRecord(context.Background(), ClipboardRecord{ID: "missing-image"}, plugin.Query{
		Env: plugin.QueryEnv{ActiveWindowTitle: "Test window", ActiveWindowPid: 1234},
	})
	for _, action := range result.Actions {
		if !action.IsDefault {
			continue
		}
		action.Action(context.Background(), plugin.ActionContext{})
		if api.notification != "translated:plugin_clipboard_image_restore_failed" {
			t.Fatalf("notification = %q, want translated restore error", api.notification)
		}
		if !strings.Contains(strings.Join(api.logs, "\n"), "file missing: id=missing-image") {
			t.Fatal("detailed restore error missing from logs")
		}
		return
	}
	t.Fatal("default paste action missing")
}

func TestConvertImageRecordSeparatesTitleAndMetadata(t *testing.T) {
	width, height, size := 2443, 1777, int64(1024*1024)
	alias := "My screenshot"
	for _, test := range []struct {
		alias           *string
		width           *int
		size            *int64
		title, subtitle string
	}{
		{nil, &width, &size, "i18n:plugin_clipboard_refinement_type_image", "2443 × 1777 · 1.0 MB"},
		{&alias, &width, &size, alias, "2443 × 1777 · 1.0 MB"},
		{nil, nil, &size, "i18n:plugin_clipboard_refinement_type_image", "1.0 MB"},
		{nil, &width, nil, "i18n:plugin_clipboard_refinement_type_image", "2443 × 1777"},
		{nil, nil, nil, "i18n:plugin_clipboard_refinement_type_image", ""},
	} {
		c := &ClipboardPlugin{api: &imagePasteFailureAPI{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
		c.imageCache.Store("image", &ImageCacheEntry{})
		record := ClipboardRecord{ID: "image", Content: "Image (2443×1777) (1.0 MB)", Alias: test.alias, Width: test.width, Height: &height, FileSize: test.size}
		result := c.convertImageRecord(context.Background(), record, plugin.Query{})
		if result.Title != test.title || result.SubTitle != test.subtitle {
			t.Fatalf("title/subtitle = %q / %q, want %q / %q", result.Title, result.SubTitle, test.title, test.subtitle)
		}
	}
}

func TestConvertImageRecordExposesFavoriteAndEditTitleActions(t *testing.T) {
	api := &imagePasteFailureAPI{}
	c := &ClipboardPlugin{api: api, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	alias := "receipt screenshot"
	result := c.convertImageRecord(context.Background(), ClipboardRecord{
		ID:      "image-fav",
		Type:    string(clipboard.ClipboardTypeImage),
		Content: "Image (10×10) (1 B)",
		Alias:   &alias,
	}, plugin.Query{})

	if result.Title != alias {
		t.Fatalf("title = %q, want alias %q", result.Title, alias)
	}
	if !clipboardResultHasAction(result, "i18n:plugin_clipboard_mark_favorite") {
		t.Fatal("image records must expose add to favorites")
	}
	if clipboardResultHasAction(result, "i18n:plugin_clipboard_cancel_favorite") {
		t.Fatal("non-favorite image records must not expose cancel favorite")
	}
	if !clipboardResultHasAction(result, "i18n:plugin_clipboard_edit_title") {
		t.Fatal("image records must expose edit title")
	}

	favorite := c.convertImageRecord(context.Background(), ClipboardRecord{
		ID:         "image-fav",
		Type:       string(clipboard.ClipboardTypeImage),
		Content:    "Image (10×10) (1 B)",
		IsFavorite: true,
	}, plugin.Query{})
	if !clipboardResultHasAction(favorite, "i18n:plugin_clipboard_cancel_favorite") {
		t.Fatal("favorite image records must expose cancel favorite")
	}
	if clipboardResultHasAction(favorite, "i18n:plugin_clipboard_mark_favorite") {
		t.Fatal("favorite image records must not expose add to favorites")
	}
}

func TestConvertTextRecordPlacesEditTitleNextToEditText(t *testing.T) {
	api := &imagePasteFailureAPI{}
	c := &ClipboardPlugin{api: api, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	result := c.convertTextRecord(context.Background(), ClipboardRecord{
		ID:      "text-edit",
		Type:    string(clipboard.ClipboardTypeText),
		Content: "hello clipboard",
	}, plugin.Query{})

	titleIdx := clipboardResultActionIndex(result, "i18n:plugin_clipboard_edit_title")
	textIdx := clipboardResultActionIndex(result, "i18n:plugin_clipboard_edit_text")
	if titleIdx < 0 || textIdx < 0 {
		t.Fatal("text records must expose edit title and edit text")
	}
	if textIdx != titleIdx+1 {
		t.Fatalf("edit title at %d and edit text at %d must be adjacent with title first", titleIdx, textIdx)
	}
}

func TestConvertRecordsExposeOpenContainingFolderAction(t *testing.T) {
	api := &imagePasteFailureAPI{}
	c := &ClipboardPlugin{api: api, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	query := plugin.Query{}

	imageWithoutPath := c.convertImageRecord(context.Background(), ClipboardRecord{
		ID:      "image-no-path",
		Type:    string(clipboard.ClipboardTypeImage),
		Content: "Image (10×10) (1 B)",
	}, query)
	if clipboardResultHasAction(imageWithoutPath, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("image records without a file path must not expose open containing folder")
	}

	imageWithPath := c.convertImageRecord(context.Background(), ClipboardRecord{
		ID:       "image-path",
		Type:     string(clipboard.ClipboardTypeImage),
		Content:  "Image (10×10) (1 B)",
		FilePath: filepath.Join(t.TempDir(), "clipboard.png"),
	}, query)
	if !clipboardResultHasAction(imageWithPath, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("image records with a file path must expose open containing folder")
	}

	singleFile := c.convertFileRecord(context.Background(), ClipboardRecord{
		ID:        "file-single",
		Type:      string(clipboard.ClipboardTypeFile),
		Content:   "shot.png",
		FilePaths: []string{filepath.Join(t.TempDir(), "shot.png")},
	}, query)
	if !clipboardResultHasAction(singleFile, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("single file records must expose open containing folder")
	}

	multiFile := c.convertFileRecord(context.Background(), ClipboardRecord{
		ID:        "file-multi",
		Type:      string(clipboard.ClipboardTypeFile),
		Content:   "shot.png (+1)",
		FilePaths: []string{filepath.Join(t.TempDir(), "a.png"), filepath.Join(t.TempDir(), "b.png")},
	}, query)
	if !clipboardResultHasAction(multiFile, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("multi-file records must expose open containing folder")
	}

	root := t.TempDir()
	textFile := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(textFile, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	textFileRecord := c.convertTextRecord(context.Background(), ClipboardRecord{
		ID:      "text-file",
		Type:    string(clipboard.ClipboardTypeText),
		Content: textFile,
	}, query)
	if !clipboardResultHasAction(textFileRecord, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("text records that are files must expose open containing folder")
	}

	textDir := c.convertTextRecord(context.Background(), ClipboardRecord{
		ID:      "text-dir",
		Type:    string(clipboard.ClipboardTypeText),
		Content: root,
	}, query)
	if clipboardResultHasAction(textDir, "i18n:plugin_clipboard_open_containing_folder") {
		t.Fatal("text records that are directories must keep open path instead of reveal")
	}
}

func clipboardResultHasAction(result plugin.QueryResult, name string) bool {
	return clipboardResultActionIndex(result, name) >= 0
}

func clipboardResultActionIndex(result plugin.QueryResult, name string) int {
	for index, action := range result.Actions {
		if action.Name == name {
			return index
		}
	}
	return -1
}

func TestApplyCopyPastePrimaryActionHotkeys(t *testing.T) {
	alternate := util.PrimaryHotkey("enter")

	copyPrimaryCopy := plugin.QueryResultAction{Name: "copy"}
	copyPrimaryPaste := plugin.QueryResultAction{Name: "paste"}
	applyCopyPastePrimaryAction(&copyPrimaryCopy, &copyPrimaryPaste, primaryActionValueCopy)
	if !copyPrimaryCopy.IsDefault || copyPrimaryPaste.IsDefault {
		t.Fatalf("copy primary: copy default=%v paste default=%v, want copy=true paste=false", copyPrimaryCopy.IsDefault, copyPrimaryPaste.IsDefault)
	}
	if copyPrimaryCopy.Hotkey != "" {
		t.Fatalf("copy primary: copy hotkey = %q, want empty", copyPrimaryCopy.Hotkey)
	}
	if copyPrimaryPaste.Hotkey != alternate {
		t.Fatalf("copy primary: paste hotkey = %q, want %q", copyPrimaryPaste.Hotkey, alternate)
	}

	pastePrimaryCopy := plugin.QueryResultAction{Name: "copy"}
	pastePrimaryPaste := plugin.QueryResultAction{Name: "paste"}
	applyCopyPastePrimaryAction(&pastePrimaryCopy, &pastePrimaryPaste, primaryActionValuePaste)
	if pastePrimaryCopy.IsDefault || !pastePrimaryPaste.IsDefault {
		t.Fatalf("paste primary: copy default=%v paste default=%v, want copy=false paste=true", pastePrimaryCopy.IsDefault, pastePrimaryPaste.IsDefault)
	}
	if pastePrimaryCopy.Hotkey != alternate {
		t.Fatalf("paste primary: copy hotkey = %q, want %q", pastePrimaryCopy.Hotkey, alternate)
	}
	if pastePrimaryPaste.Hotkey != "" {
		t.Fatalf("paste primary: paste hotkey = %q, want empty", pastePrimaryPaste.Hotkey)
	}

	copyOnly := plugin.QueryResultAction{Name: "copy"}
	applyCopyPastePrimaryAction(&copyOnly, nil, primaryActionValueCopy)
	if !copyOnly.IsDefault {
		t.Fatal("copy-only with copy primary must keep copy as default")
	}
	if copyOnly.Hotkey != "" {
		t.Fatalf("copy-only hotkey = %q, want empty", copyOnly.Hotkey)
	}

	pasteOnlyCopy := plugin.QueryResultAction{Name: "copy"}
	applyCopyPastePrimaryAction(&pasteOnlyCopy, nil, primaryActionValuePaste)
	if pasteOnlyCopy.IsDefault {
		t.Fatal("copy-only with paste primary must leave copy unmarked so the manager can promote it")
	}
}

func TestClipboardIgnoredApplicationsSettingUsesSharedAppPicker(t *testing.T) {
	settings := (&ClipboardPlugin{}).GetMetadata().SettingDefinitions
	var privacyHeadFound bool
	var ignoredAppsFound bool
	for _, item := range settings {
		switch value := item.Value.(type) {
		case *definition.PluginSettingValueHead:
			if value.Content == "i18n:plugin_clipboard_privacy" {
				privacyHeadFound = true
			}
		case *definition.PluginSettingValueTable:
			if value.Key != ignoredApplicationsSettingKey {
				continue
			}
			ignoredAppsFound = true
			if !value.InlineTable || value.DefaultValue != "[]" || len(value.Columns) != 1 {
				t.Fatalf("ignored applications table = %+v", value)
			}
			if value.Columns[0].Type != definition.PluginSettingValueTableColumnTypeApp {
				t.Fatalf("ignored applications column type = %q, want app", value.Columns[0].Type)
			}
			if !item.IsPlatformSpecific {
				t.Fatal("ignored applications must be platform specific")
			}
			if len(item.DisabledInPlatforms) != 1 || item.DisabledInPlatforms[0] != util.PlatformLinux {
				t.Fatalf("disabled platforms = %v, want Linux", item.DisabledInPlatforms)
			}
		}
	}
	if !privacyHeadFound || !ignoredAppsFound {
		t.Fatalf("privacy head found=%v ignored applications found=%v", privacyHeadFound, ignoredAppsFound)
	}
}

func TestIgnoredClipboardApplicationMatching(t *testing.T) {
	rows, err := parseIgnoredClipboardApplications(`[{"App":{"Name":"TextEdit","Identity":"com.apple.TextEdit","Path":"/System/Applications/TextEdit.app"}}]`)
	if err != nil {
		t.Fatalf("parse ignored applications: %v", err)
	}
	if !isIgnoredClipboardApplication(rows, " COM.APPLE.TEXTEDIT ") {
		t.Fatal("expected identity to match case-insensitively")
	}
	if isIgnoredClipboardApplication(rows, "com.apple.Safari") {
		t.Fatal("unexpected identity match")
	}
}

func TestResolveClipboardFilesystemPathAcceptsFileAndDirectory(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	dir, isDir := resolveClipboardFilesystemPath(`"` + root + `"`)
	if !isDir || dir != filepath.Clean(root) {
		t.Fatalf("directory path = %q isDir=%v", dir, isDir)
	}
	file, isDir := resolveClipboardFilesystemPath(filePath)
	if isDir || file != filepath.Clean(filePath) {
		t.Fatalf("file path = %q isDir=%v", file, isDir)
	}
	if path, ok := resolveClipboardFilesystemPath("relative/path"); path != "" || ok {
		t.Fatalf("relative path should be rejected: %q %v", path, ok)
	}
}

func TestParseIgnoredClipboardApplicationsRejectsMalformedJSON(t *testing.T) {
	if _, err := parseIgnoredClipboardApplications("["); err == nil {
		t.Fatal("expected malformed setting to fail")
	}
}

func TestClipboardSearchCandidatesIncludeImageOCRInAllSearch(t *testing.T) {
	ocrText := "invoice total 42"
	alias := "receipt"
	image := clipboardSearchItem{
		Type:    string(clipboard.ClipboardTypeImage),
		Content: `C:\Users\me\AppData\Local\Wox\shot.png`,
		Alias:   &alias,
		OCRText: &ocrText,
	}

	allCandidates := clipboardSearchCandidates(image, clipboardTypeRefinementAll)
	if slices.Contains(allCandidates, image.Content) {
		t.Fatal("All search must not match image cache paths")
	}
	if !slices.Contains(allCandidates, ocrText) || !slices.Contains(allCandidates, alias) {
		t.Fatalf("All search candidates = %v, want OCR text and alias", allCandidates)
	}

	imageCandidates := clipboardSearchCandidates(image, string(clipboard.ClipboardTypeImage))
	if !slices.Contains(imageCandidates, image.Content) || !slices.Contains(imageCandidates, ocrText) {
		t.Fatalf("Image search candidates = %v, want cache path and OCR text", imageCandidates)
	}
}

func TestClipboardFavQueryHintExposesSearchArgument(t *testing.T) {
	commands := (&ClipboardPlugin{}).GetMetadata().Commands
	for _, command := range commands {
		if command.Command != "fav" {
			continue
		}
		if command.QueryHint == nil || command.QueryHint.Argument("search") != "" {
			t.Fatalf("fav QueryHint = %+v, want an empty search argument", command.QueryHint)
		}
		if got := command.QueryHint.Elements[0].Placeholder; got != "i18n:plugin_clipboard_command_fav_search_placeholder" {
			t.Fatalf("fav search placeholder = %q", got)
		}
		return
	}
	t.Fatal("fav command missing")
}

func TestClipboardFavoriteVisibleInFavQuery(t *testing.T) {
	text := FavoriteClipboardItem{Type: string(clipboard.ClipboardTypeText), Content: "pypi token"}
	file := FavoriteClipboardItem{
		Type:      string(clipboard.ClipboardTypeFile),
		Content:   "notes.txt",
		FilePaths: []string{`C:\tmp\notes.txt`},
	}
	ctx := context.Background()

	if !clipboardFavoriteVisibleInFavQuery(ctx, text, "", clipboardTypeRefinementAll) {
		t.Fatal("empty fav search must keep text favorites")
	}
	if !clipboardFavoriteVisibleInFavQuery(ctx, file, "", clipboardTypeRefinementAll) {
		t.Fatal("empty fav search must keep file favorites in All")
	}
	if clipboardFavoriteVisibleInFavQuery(ctx, text, "", string(clipboard.ClipboardTypeImage)) {
		t.Fatal("empty fav search must still honor the Image refinement")
	}
	if clipboardFavoriteVisibleInFavQuery(ctx, file, "notes", clipboardTypeRefinementAll) {
		t.Fatal("fav search in All must not leak file path fragments")
	}
}

func TestClipboardFavQueryFiltersBySearch(t *testing.T) {
	if os.Getenv("WOX_CLIPBOARD_FAV_SEARCH_TEST_CHILD") == "" {
		t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
		t.Setenv(util.TestUserDataDirEnv, t.TempDir())
		t.Setenv("WOX_CLIPBOARD_FAV_SEARCH_TEST_CHILD", "1")
		command := exec.Command(os.Args[0], "-test.run=^TestClipboardFavQueryFiltersBySearch$")
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("fav search check: %v\n%s", err, output)
		}
		return
	}
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	sqlDB, err := database.GetDB().DB()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	favorites, err := json.Marshal([]FavoriteClipboardItem{
		{ID: "wox", Type: string(clipboard.ClipboardTypeText), Content: "Another option is Wox Launcher.", Timestamp: 1},
		{ID: "pypi", Type: string(clipboard.ClipboardTypeText), Content: "pypi token", Timestamp: 2},
	})
	if err != nil {
		t.Fatalf("marshal favorites: %v", err)
	}

	api := &clipboardFavoritesTestAPI{settings: map[string]string{favoritesSettingKey: string(favorites)}}
	c := &ClipboardPlugin{api: api, db: clipboardQueryTestDB{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	ctx := context.Background()

	if got := clipboardResultTitles(c.Query(ctx, plugin.Query{}).Results); len(got) != 0 {
		t.Fatalf("default cb titles = %v, want no favorites", got)
	}
	if got := clipboardResultTitles(c.Query(ctx, plugin.Query{Search: "pypi"}).Results); len(got) != 0 {
		t.Fatalf("default cb pypi titles = %v, want no favorites", got)
	}

	api.settings[showFavoritesByDefaultSettingKey] = "true"
	shown := c.Query(ctx, plugin.Query{}).Results
	if got := clipboardResultTitles(shown); len(got) != 2 || got[0] != "pypi token" || got[1] != "Another option is Wox Launcher." {
		t.Fatalf("shown cb titles = %v, want ranked favorites", got)
	}
	if clipboardHasMoveAction(shown[0], true) || !clipboardHasMoveAction(shown[0], false) || !clipboardHasMoveAction(shown[1], true) || clipboardHasMoveAction(shown[1], false) {
		t.Fatalf("cb favorite moves = up/down %v/%v then %v/%v", clipboardHasMoveAction(shown[0], true), clipboardHasMoveAction(shown[0], false), clipboardHasMoveAction(shown[1], true), clipboardHasMoveAction(shown[1], false))
	}
	searched := c.Query(ctx, plugin.Query{Search: "pypi"}).Results
	if got := clipboardResultTitles(searched); len(got) != 1 || got[0] != "pypi token" {
		t.Fatalf("shown cb pypi titles = %v, want [pypi token]", got)
	}
	if clipboardHasMoveAction(searched[0], true) || clipboardHasMoveAction(searched[0], false) {
		t.Fatal("filtered cb favorites must not expose moves outside the visible list")
	}

	listed := c.Query(context.Background(), plugin.Query{Command: "fav"})
	if got := clipboardResultTitles(listed.Results); len(got) != 2 {
		t.Fatalf("cb fav titles = %v, want both favorites", got)
	}

	filtered := c.Query(context.Background(), plugin.Query{Command: "fav", Search: "pypi"})
	titles := clipboardResultTitles(filtered.Results)
	if len(titles) != 1 || titles[0] != "pypi token" {
		t.Fatalf("cb fav pypi titles = %v, want [pypi token]", titles)
	}
	for _, action := range filtered.Results[0].Actions {
		if action.Name == "i18n:plugin_clipboard_move_favorite_up" || action.Name == "i18n:plugin_clipboard_move_favorite_down" {
			t.Fatal("filtered favorites must not expose moves outside the visible list")
		}
	}

	unmatched := c.Query(context.Background(), plugin.Query{Command: "fav", Search: "sdf"})
	if got := clipboardResultTitles(unmatched.Results); len(got) != 0 {
		t.Fatalf("cb fav sdf titles = %v, want none", got)
	}

	newRecord := ClipboardRecord{ID: "new", Type: string(clipboard.ClipboardTypeText), Content: "new text", Timestamp: 3}
	result := c.convertRecordToResult(ctx, newRecord, plugin.Query{})
	for _, action := range result.Actions {
		if action.Name == "i18n:plugin_clipboard_mark_favorite" {
			action.Action(ctx, plugin.ActionContext{})
			if api.changedQuery.QueryType != plugin.QueryTypeInput || api.changedQuery.QueryText != "cb fav " {
				t.Fatalf("favorite action changed query to %+v", api.changedQuery)
			}
			return
		}
	}
	t.Fatal("mark favorite action missing")
}

func TestClipboardFavoriteUsageAndManualRank(t *testing.T) {
	ctx := context.Background()
	legacyScores := favoriteDisplayScores([]FavoriteClipboardItem{{ID: "old", Timestamp: 1}, {ID: "new", Timestamp: 2}})
	if legacyScores["new"] <= legacyScores["old"] {
		t.Fatalf("legacy favorites lost timestamp order: %v", legacyScores)
	}
	items := []FavoriteClipboardItem{
		{ID: "top", Type: string(clipboard.ClipboardTypeText), Content: "Top", UseCount: 21, Timestamp: 4},
		{ID: "a", Type: string(clipboard.ClipboardTypeText), Content: "A", UseCount: 20, Timestamp: 3},
		{ID: "b", Type: string(clipboard.ClipboardTypeText), Content: "B", UseCount: 18, Timestamp: 2},
		{ID: "c", Type: string(clipboard.ClipboardTypeText), Content: "C", UseCount: 17, Timestamp: 1},
	}
	encoded, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	api := &clipboardFavoritesTestAPI{settings: map[string]string{favoritesSettingKey: string(encoded)}}
	c := &ClipboardPlugin{api: api, db: clipboardQueryTestDB{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}
	if err := c.saveFavoriteRankStats(ctx, items); err != nil {
		t.Fatal(err)
	}

	initialResults := c.Query(ctx, plugin.Query{Command: "fav"}).Results
	for i, result := range initialResults {
		hasUp, hasDown := false, false
		for _, action := range result.Actions {
			hasUp = hasUp || action.Name == "i18n:plugin_clipboard_move_favorite_up"
			hasDown = hasDown || action.Name == "i18n:plugin_clipboard_move_favorite_down"
		}
		if result.Id != result.ScoreKey {
			t.Fatalf("favorite result id = %q, score key = %q", result.Id, result.ScoreKey)
		}
		if hasUp != (i > 0) || hasDown != (i+1 < len(initialResults)) {
			t.Fatalf("favorite %s at %d has up=%v down=%v", result.ScoreKey, i, hasUp, hasDown)
		}
	}
	for _, result := range initialResults {
		if result.ScoreKey != "b" {
			continue
		}
		for _, action := range result.Actions {
			if action.Name == "i18n:plugin_clipboard_move_favorite_up" {
				action.Action(ctx, plugin.ActionContext{})
			}
		}
	}
	if api.refreshCount != 1 || api.lastRefresh.SelectedResultId != "b" || api.lastRefresh.PreserveSelectedIndex {
		t.Fatalf("refresh = %+v count %d, want selected result b", api.lastRefresh, api.refreshCount)
	}
	items, err = c.getFavoriteItems(ctx)
	if err != nil {
		t.Fatal(err)
	}
	sortFavoritesByRank(items)
	if items[0].ID != "top" || items[1].ID != "b" || items[2].ID != "a" || items[3].ID != "c" || items[1].rankScore()-items[2].rankScore() != favoriteMoveScoreGap {
		t.Fatalf("after moving B up: %+v", items)
	}
	response := c.Query(ctx, plugin.Query{Command: "fav"})
	resultScores := map[string]int64{}
	for _, result := range response.Results {
		resultScores[result.ScoreKey] = result.Score
	}
	if !(resultScores["top"] > resultScores["b"] && resultScores["b"] > resultScores["a"] && resultScores["a"] > resultScores["c"]) {
		t.Fatalf("cb fav scores = %v", resultScores)
	}

	for i := 0; i < 10; i++ {
		c.recordUsed(ctx, ClipboardRecord{ID: "a", IsFavorite: true})
	}
	items, _ = c.getFavoriteItems(ctx)
	sortFavoritesByRank(items)
	if items[1].ID != "b" {
		t.Fatalf("A overtook B before its 11th use: %+v", items)
	}
	c.recordUsed(ctx, ClipboardRecord{ID: "a", IsFavorite: true})
	items, _ = c.getFavoriteItems(ctx)
	sortFavoritesByRank(items)
	if items[1].ID != "a" {
		t.Fatalf("A did not overtake B after its 11th use: %+v", items)
	}

	c.moveFavoriteAndRefresh(ctx, "a", false)
	items, _ = c.getFavoriteItems(ctx)
	sortFavoritesByRank(items)
	if api.refreshCount != 2 || api.lastRefresh.SelectedResultId != "a" || items[0].ID != "top" || items[1].ID != "b" || items[2].ID != "a" || items[1].rankScore()-items[2].rankScore() != favoriteMoveScoreGap {
		t.Fatalf("after moving A down: refresh=%+v count=%d items=%+v", api.lastRefresh, api.refreshCount, items)
	}
	if api.settings[favoritesSettingKey] != string(encoded) {
		t.Fatal("usage and reordering must not rewrite favorite content")
	}
	statsBefore := api.settings[favoriteRankStatsSettingKey]
	c.moveFavoriteAndRefresh(ctx, "missing", true)
	c.moveFavoriteAndRefresh(ctx, "top", true)
	if api.refreshCount != 2 || api.settings[favoriteRankStatsSettingKey] != statsBefore {
		t.Fatal("invalid moves must not persist or refresh")
	}
	api.failSetSetting = true
	c.moveFavoriteAndRefresh(ctx, "b", true)
	if api.refreshCount != 2 || api.settings[favoriteRankStatsSettingKey] != statsBefore {
		t.Fatal("failed persistence must not refresh or change the saved rank")
	}
}

func TestMoveFavoriteOneStepEdgeCases(t *testing.T) {
	for _, tc := range []struct {
		name       string
		items      []FavoriteClipboardItem
		id         string
		up         bool
		wantMove   bool
		wantOrder  []string
		wantScores []int64
	}{
		{name: "empty", id: "missing", up: true},
		{name: "single top", items: []FavoriteClipboardItem{{ID: "A"}}, id: "A", up: true, wantOrder: []string{"A"}, wantScores: []int64{0}},
		{name: "single bottom", items: []FavoriteClipboardItem{{ID: "A"}}, id: "A", wantOrder: []string{"A"}, wantScores: []int64{0}},
		{name: "missing", items: []FavoriteClipboardItem{{ID: "A", UseCount: 10}, {ID: "B"}}, id: "missing", up: true, wantOrder: []string{"A", "B"}, wantScores: []int64{10, 0}},
		{name: "top cannot move up", items: []FavoriteClipboardItem{{ID: "A", UseCount: 10}, {ID: "B"}}, id: "A", up: true, wantOrder: []string{"A", "B"}, wantScores: []int64{10, 0}},
		{name: "bottom cannot move down", items: []FavoriteClipboardItem{{ID: "A", UseCount: 10}, {ID: "B"}}, id: "B", wantOrder: []string{"A", "B"}, wantScores: []int64{10, 0}},
		{name: "two items up", items: []FavoriteClipboardItem{{ID: "A", UseCount: 5}, {ID: "B"}}, id: "B", up: true, wantMove: true, wantOrder: []string{"B", "A"}, wantScores: []int64{15, 5}},
		{name: "two items down", items: []FavoriteClipboardItem{{ID: "A", UseCount: 5}, {ID: "B"}}, id: "A", wantMove: true, wantOrder: []string{"B", "A"}, wantScores: []int64{15, 5}},
		{name: "five point gap up", items: []FavoriteClipboardItem{{ID: "A", UseCount: 100}, {ID: "B", UseCount: 95}, {ID: "C", UseCount: 90}}, id: "C", up: true, wantMove: true, wantOrder: []string{"A", "C", "B"}, wantScores: []int64{115, 105, 95}},
		{name: "five point gap down", items: []FavoriteClipboardItem{{ID: "A", UseCount: 100}, {ID: "B", UseCount: 95}, {ID: "C", UseCount: 90}}, id: "B", wantMove: true, wantOrder: []string{"A", "C", "B"}, wantScores: []int64{115, 105, 95}},
		{name: "wide gap leaves top alone", items: []FavoriteClipboardItem{{ID: "A", UseCount: 100}, {ID: "B", UseCount: 20}, {ID: "C"}}, id: "C", up: true, wantMove: true, wantOrder: []string{"A", "C", "B"}, wantScores: []int64{100, 30, 20}},
		{name: "all tied", items: []FavoriteClipboardItem{{ID: "A", Timestamp: 3}, {ID: "B", Timestamp: 2}, {ID: "C", Timestamp: 1}}, id: "C", up: true, wantMove: true, wantOrder: []string{"A", "C", "B"}, wantScores: []int64{20, 10, 0}},
		{name: "crowded chain", items: []FavoriteClipboardItem{{ID: "A", UseCount: 9}, {ID: "B", UseCount: 8}, {ID: "C", UseCount: 7}, {ID: "D", UseCount: 6}}, id: "D", up: true, wantMove: true, wantOrder: []string{"A", "B", "D", "C"}, wantScores: []int64{37, 27, 17, 7}},
		{name: "negative bonus", items: []FavoriteClipboardItem{{ID: "A", RankBonus: -5}, {ID: "B", RankBonus: -10}, {ID: "C", RankBonus: -11}}, id: "C", up: true, wantMove: true, wantOrder: []string{"A", "C", "B"}, wantScores: []int64{10, 0, -10}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			originalUses := make(map[string]int64, len(tc.items))
			for _, item := range tc.items {
				originalUses[item.ID] = item.UseCount
			}
			if moved := moveFavoriteOneStep(tc.items, tc.id, tc.up); moved != tc.wantMove {
				t.Fatalf("moved = %v, want %v", moved, tc.wantMove)
			}
			for i, item := range tc.items {
				if item.ID != tc.wantOrder[i] || item.rankScore() != tc.wantScores[i] || item.UseCount != originalUses[item.ID] {
					t.Fatalf("item %d = %+v, want id=%s score=%d original use count=%d", i, item, tc.wantOrder[i], tc.wantScores[i], originalUses[item.ID])
				}
			}
		})
	}
}

func clipboardHasMoveAction(result plugin.QueryResult, up bool) bool {
	name := "i18n:plugin_clipboard_move_favorite_down"
	if up {
		name = "i18n:plugin_clipboard_move_favorite_up"
	}
	for _, action := range result.Actions {
		if action.Name == name {
			return true
		}
	}
	return false
}

func clipboardResultTitles(results []plugin.QueryResult) []string {
	titles := make([]string, 0, len(results))
	for _, result := range results {
		titles = append(titles, result.Title)
	}
	return titles
}

type clipboardFavoritesTestAPI struct {
	imagePasteFailureAPI
	settings       map[string]string
	refreshCount   int
	lastRefresh    plugin.RefreshQueryParam
	changedQuery   common.PlainQuery
	failSetSetting bool
}

func (a *clipboardFavoritesTestAPI) GetSetting(_ context.Context, key string) string {
	return a.settings[key]
}

func (a *clipboardFavoritesTestAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	if a.failSetSetting {
		return plugin.SetSettingResult{ErrMsg: "test failure"}
	}
	a.settings[option.Key] = option.Value
	return plugin.SetSettingResult{Success: true}
}

func (a *clipboardFavoritesTestAPI) RefreshQuery(_ context.Context, param plugin.RefreshQueryParam) {
	a.refreshCount++
	a.lastRefresh = param
}

func (a *clipboardFavoritesTestAPI) ChangeQuery(_ context.Context, query common.PlainQuery) {
	a.changedQuery = query
}

type clipboardQueryTestDB struct{}

func (clipboardQueryTestDB) Insert(context.Context, ClipboardRecord) error {
	return nil
}
func (clipboardQueryTestDB) Update(context.Context, ClipboardRecord) error {
	return nil
}
func (clipboardQueryTestDB) UpdateTimestamp(context.Context, string, int64) error {
	return nil
}
func (clipboardQueryTestDB) UpdateContent(context.Context, string, string) error {
	return nil
}
func (clipboardQueryTestDB) UpdateAlias(context.Context, string, *string) error {
	return nil
}
func (clipboardQueryTestDB) UpdateOCRText(context.Context, string, *string) error {
	return nil
}
func (clipboardQueryTestDB) Delete(context.Context, string) error { return nil }
func (clipboardQueryTestDB) GetRecent(context.Context, int, int) ([]ClipboardRecord, error) {
	return nil, nil
}
func (clipboardQueryTestDB) GetRecentByType(context.Context, string, int, int) ([]ClipboardRecord, error) {
	return nil, nil
}
func (clipboardQueryTestDB) SearchText(context.Context, string, int) ([]ClipboardRecord, error) {
	return nil, nil
}
func (clipboardQueryTestDB) SearchByType(context.Context, string, string, int) ([]ClipboardRecord, error) {
	return nil, nil
}
func (clipboardQueryTestDB) GetByID(context.Context, string) (*ClipboardRecord, error) {
	return nil, nil
}
func (clipboardQueryTestDB) DeleteExpired(context.Context, int, int) (int64, error) {
	return 0, nil
}
func (clipboardQueryTestDB) EnforceMaxCount(context.Context, int) (int64, error) {
	return 0, nil
}
func (clipboardQueryTestDB) GetStats(context.Context) (map[string]int, error) {
	return map[string]int{}, nil
}
func (clipboardQueryTestDB) Close() error { return nil }

type clipboardRestoreTestDB struct {
	clipboardQueryTestDB
	records map[string]ClipboardRecord
}

func (d clipboardRestoreTestDB) GetByID(_ context.Context, id string) (*ClipboardRecord, error) {
	record, ok := d.records[id]
	if !ok {
		return nil, nil
	}
	copy := record
	return &copy, nil
}

func TestHandleMRURestoreRestoresFavoriteWhenHistoryRowIsGone(t *testing.T) {
	favorites, err := json.Marshal([]FavoriteClipboardItem{
		{ID: "fav-1", Type: string(clipboard.ClipboardTypeText), Content: "Another option is Wox Launcher.", Timestamp: 1},
	})
	if err != nil {
		t.Fatalf("marshal favorites: %v", err)
	}

	api := &clipboardFavoritesTestAPI{settings: map[string]string{favoritesSettingKey: string(favorites)}}
	c := &ClipboardPlugin{api: api, db: clipboardQueryTestDB{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}

	restored, err := c.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{"recordId": "fav-1"},
	})
	if err != nil {
		t.Fatalf("restore favorite: %v", err)
	}
	if restored.Title != "Another option is Wox Launcher." {
		t.Fatalf("title = %q", restored.Title)
	}
	if restored.ScoreKey != "fav-1" {
		t.Fatalf("score key = %q", restored.ScoreKey)
	}
}

func TestHandleMRURestoreRestoresHistoryRecord(t *testing.T) {
	api := &clipboardFavoritesTestAPI{settings: map[string]string{}}
	c := &ClipboardPlugin{
		api:        api,
		db:         clipboardRestoreTestDB{records: map[string]ClipboardRecord{"hist-1": {ID: "hist-1", Type: string(clipboard.ClipboardTypeText), Content: "history text", Timestamp: 1}}},
		imageCache: util.NewHashMap[string, *ImageCacheEntry](),
	}

	restored, err := c.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{"recordId": "hist-1"},
	})
	if err != nil {
		t.Fatalf("restore history: %v", err)
	}
	if restored.Title != "history text" {
		t.Fatalf("title = %q", restored.Title)
	}
}

func TestHandleMRURestoreMissingRecord(t *testing.T) {
	api := &clipboardFavoritesTestAPI{settings: map[string]string{}}
	c := &ClipboardPlugin{api: api, db: clipboardQueryTestDB{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}

	if _, err := c.handleMRURestore(context.Background(), plugin.MRUData{}); err == nil {
		t.Fatal("empty context should fail restore")
	}
	if _, err := c.handleMRURestore(context.Background(), plugin.MRUData{
		ContextData: common.ContextData{"recordId": "missing"},
	}); err == nil {
		t.Fatal("missing record should fail restore")
	}
}

func TestClipboardSearchCandidatesKeepTypedRefinementsScoped(t *testing.T) {
	text := clipboardSearchItem{Type: string(clipboard.ClipboardTypeText), Content: "hello world"}
	if candidates := clipboardSearchCandidates(text, clipboardTypeRefinementAll); !slices.Contains(candidates, text.Content) {
		t.Fatalf("All text candidates = %v, want content", candidates)
	}

	file := clipboardSearchItem{
		Type:      string(clipboard.ClipboardTypeFile),
		Content:   "notes.txt",
		FilePaths: []string{`C:\tmp\notes.txt`},
	}
	if candidates := clipboardSearchCandidates(file, clipboardTypeRefinementAll); len(candidates) != 0 {
		t.Fatalf("All file candidates = %v, want none", candidates)
	}

	fileCandidates := clipboardSearchCandidates(file, string(clipboard.ClipboardTypeFile))
	if !slices.Contains(fileCandidates, file.Content) || !slices.Contains(fileCandidates, file.FilePaths[0]) || !slices.Contains(fileCandidates, "notes.txt") {
		t.Fatalf("File search candidates = %v, want content and path", fileCandidates)
	}

	if clipboardRecordMatchesType(text.Type, text.Content, string(clipboard.ClipboardTypeImage)) {
		t.Fatal("text records must not match the Image refinement")
	}
	if !clipboardRecordMatchesType(string(clipboard.ClipboardTypeImage), "", clipboardTypeRefinementAll) {
		t.Fatal("image records must remain visible to All search")
	}
}

// TestClipboardCompactMetadata covers local midnight and year boundaries.
func TestClipboardCompactMetadata(t *testing.T) {
	now := time.Date(2026, 10, 4, 12, 0, 0, 0, time.FixedZone("test", 8*60*60))
	for _, tc := range []struct {
		age  time.Duration
		want string
	}{
		{0, "12:00"}, {13 * time.Hour, "10-03 23:00"}, {365 * 24 * time.Hour, "2025-10-04 12:00"},
	} {
		copied := now.Add(-tc.age)
		tag := clipboardTimestampTag(copied.UnixMilli(), now)
		if tag.Label != tc.want || tag.Tooltip != copied.Format("2006-01-02 15:04:05") {
			t.Fatalf("unexpected time tag: %+v", tag)
		}
	}
}
