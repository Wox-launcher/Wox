package system

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
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
