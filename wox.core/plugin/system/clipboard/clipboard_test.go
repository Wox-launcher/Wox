package system

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"wox/setting/definition"
	"wox/util"
	"wox/util/clipboard"
)

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
