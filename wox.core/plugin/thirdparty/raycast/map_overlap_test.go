package raycast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/plugin/thirdparty/migrate"
	"wox/setting"
)

func TestMapOverlapSettingsFromExport(t *testing.T) {
	root := t.TempDir()
	extra := filepath.Join(root, "apps")
	if err := os.Mkdir(extra, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := Payload{}
	payload.Settings.General = map[string]json.RawMessage{
		"appearance":             json.RawMessage(`"dark"`),
		"windowMode":             json.RawMessage(`"compact"`),
		"windowPresentationMode": json.RawMessage(`"mouse-screen"`),
	}
	payload.Settings.InternalExtensions = []InternalExtension{
		{ID: raycastApplicationsID, Meta: json.RawMessage(`{"userSearchScopes":["` + jsonPath(extra) + `","relative"]}`)},
		{ID: raycastDictationExtensionID, Meta: json.RawMessage(`{"sounds":true,"customVocabulary":["Wox"]}`)},
		{ID: raycastEmojiExtensionID, Meta: json.RawMessage(`{"primaryAction":"paste"}`)},
		{ID: raycastScreenshotExtensionID, Meta: json.RawMessage(`{"textRecognitionEnabled":true,"storageDuration":"unlimited"}`)},
	}
	payload.Emoji.Emojis = []EmojiEntry{{Symbol: "😀", CustomKeywords: []string{"grin"}}}
	payload.MCPServers.Servers = []MCPServer{{Title: "ddg", Type: "stdio", Command: "uvx", Arguments: []string{"duckduckgo-mcp-server"}}}
	current := defaultSnapshot()
	current.ThemeID = setting.DefaultLightThemeId
	mapped := buildMapped(payload, current)
	got := map[string]mappedSetting{}
	for _, row := range append(append([]mappedSetting{}, mapped.general...), mapped.clipboardSettings...) {
		if row.selectable {
			got[row.id] = row
		}
	}
	if row := got["setting:theme"]; !row.selectable || row.value != setting.DefaultThemeId || row.detailKey != "theme_dark" {
		t.Fatalf("theme = %+v", row)
	}
	if row := got["setting:ui_density"]; !row.selectable || row.value != string(setting.UiDensityCompact) {
		t.Fatalf("density = %+v", row)
	}
	if _, exists := got["setting:window_position"]; exists {
		t.Fatal("default mouse position was offered again")
	}
	if row := got["setting:app_directories"]; !row.selectable || row.pluginID != common.AppPluginID || !strings.Contains(row.detail, extra) || strings.Contains(row.detail, "relative") {
		t.Fatalf("directories = %+v", row)
	}
	if row := got["setting:mcp_servers"]; !row.selectable || !strings.Contains(row.value, "duckduckgo-mcp-server") || row.detail != "ddg" {
		t.Fatalf("mcp = %+v", row)
	}
	if _, exists := got["setting:dictation_sound"]; exists {
		t.Fatal("default dictation sound was offered again")
	}
	if row := got["setting:dictation_vocabulary"]; !row.selectable || row.pluginID != common.DictationPluginID || !strings.Contains(row.value, "Wox") {
		t.Fatalf("vocabulary = %+v", row)
	}
	if _, exists := got["setting:emoji_action"]; exists {
		t.Fatal("default emoji paste action was offered again")
	}
	if row := got["setting:emoji_history"]; !row.selectable || row.key != "frequentlyUsed" || !strings.Contains(row.value, "😀") {
		t.Fatalf("emoji = %+v", row)
	}
	if row := got["setting:emoji_keywords"]; !row.selectable || !strings.Contains(row.value, "grin") {
		t.Fatalf("keywords = %+v", row)
	}
	if _, exists := got["setting:screenshot_ocr"]; exists {
		t.Fatal("default screenshot OCR was offered again")
	}
	if _, exists := got["setting:screenshot_retention"]; exists {
		t.Fatal("unlimited screenshot retention was imported")
	}
	imported := importSelected(mapped, []string{"setting:theme", "setting:mcp_servers", "setting:dictation_vocabulary"})
	if len(imported.Settings) != 3 {
		t.Fatalf("writes = %+v", imported.Settings)
	}
	for _, category := range catalogFrom(mapped) {
		if category.ID != migrate.CategoryGeneral {
			continue
		}
		for _, item := range category.Items {
			if item.ID == "setting:mcp_servers" && item.TitleKey != "mcp_server" {
				t.Fatalf("catalog = %+v", item)
			}
		}
	}
}

// TestAppearanceImport verifies the written theme and skips an already active target.
func TestAppearanceImport(t *testing.T) {
	for _, tc := range []struct {
		appearance string
		themeID    string
	}{
		{"dark", setting.DefaultThemeId},
		{"light", setting.DefaultLightThemeId},
		{"auto", ""},
		{"system", ""},
		{"unknown", ""},
	} {
		t.Run(tc.appearance, func(t *testing.T) {
			raw, _ := json.Marshal(tc.appearance)
			payload := Payload{}
			payload.Settings.General = map[string]json.RawMessage{"appearance": raw}
			current := defaultSnapshot()
			current.ThemeID = setting.DefaultDarkThemeId
			result := importSelected(buildMapped(payload, current), []string{"setting:theme"})
			if tc.themeID == "" {
				if len(result.Settings) != 0 {
					t.Fatalf("unexpected theme override: %+v", result.Settings)
				}
				return
			}
			if len(result.Settings) != 1 || result.Settings[0].Key != "ThemeId" || result.Settings[0].Value != tc.themeID {
				t.Fatalf("writes = %+v, want ThemeId=%s", result.Settings, tc.themeID)
			}
			current.ThemeID = tc.themeID
			result = importSelected(buildMapped(payload, current), []string{"setting:theme"})
			if len(result.Settings) != 0 {
				t.Fatalf("active theme was imported again: %+v", result.Settings)
			}
		})
	}
}

func TestMapIgnoredClipboardAppMatchesInstalledName(t *testing.T) {
	payload := Payload{}
	payload.Settings.InternalExtensions = []InternalExtension{{
		ID:   raycastClipboardExtensionID,
		Meta: json.RawMessage(`{"disabledApplications":[{"raycastAppId":"/Applications/Notes.app","os":"macos"}]}`),
	}}
	rows := mapIgnoredClipboardApps(payload, []plugin.DesktopApp{{
		Name: "Notes", Path: `C:\Program Files\Notes\Notes.exe`, Executable: `C:\Program Files\Notes\Notes.exe`,
	}}, defaultSnapshot())
	if len(rows) != 1 || rows[0].pluginID != common.ClipboardPluginID || !strings.Contains(rows[0].value, "notes.exe") {
		t.Fatalf("ignored = %+v", rows)
	}
}

func TestMapClipboardFileList(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "notes.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	payload := Payload{}
	payload.ClipboardHistory.Entries = []ClipboardEntry{{
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
		Items: []struct {
			Representations []ClipboardRepresentation `json:"representations"`
		}{{Representations: []ClipboardRepresentation{
			{MimeType: "text/plain", Content: filePath},
			{MimeType: "text/uri-list", Content: "file:///" + filepath.ToSlash(filePath)},
			{MimeType: "application/vnd.raycast.ocr", ContentType: "text", Content: "hello"},
		}}},
	}}
	mapped := buildMapped(payload, defaultSnapshot())
	if len(mapped.clipboard) != 1 || len(mapped.clipboard[0].filePaths) != 1 || mapped.clipboard[0].text != "" || mapped.clipboard[0].ocrText != "hello" {
		t.Fatalf("clipboard = %+v", mapped.clipboard)
	}
	if !mapped.clipboard[0].selectable || mapped.clipboard[0].detailKey != "raycast_clipboard_file" {
		t.Fatalf("row = %+v", mapped.clipboard[0])
	}
	imported := importSelected(mapped, []string{migrate.ClipboardSummaryID})
	if len(imported.Clipboard) != 1 || imported.Clipboard[0].FilePaths[0] != filePath || imported.Clipboard[0].OCRText != "hello" {
		t.Fatalf("import = %+v", imported.Clipboard)
	}
}

func TestMapSkillDirectoriesAndDictationMute(t *testing.T) {
	root := t.TempDir()
	skills := filepath.Join(root, "skills")
	if err := os.Mkdir(skills, 0o755); err != nil {
		t.Fatal(err)
	}
	payload := Payload{}
	payload.Settings.InternalExtensions = []InternalExtension{{
		ID:   raycastAIExtensionID,
		Meta: json.RawMessage(`{"skillDirectories":["` + jsonPath(skills) + `","relative", "` + jsonPath(skills) + `"]}`),
	}, {
		ID:   raycastDictationExtensionID,
		Meta: json.RawMessage(`{"muteWhileRecording":true}`),
	}, {
		ID:   raycastClipboardExtensionID,
		Meta: json.RawMessage(`{"historyDuration":"P1M"}`),
	}}
	current := defaultSnapshot()
	current.Skills = []common.Skill{{Path: skills}}
	mapped := buildMapped(payload, current)
	got := map[string]mappedSetting{}
	for _, row := range append(append([]mappedSetting{}, mapped.general...), mapped.clipboardSettings...) {
		if row.selectable {
			got[row.id] = row
		}
	}
	if _, exists := got["setting:ai_skills"]; exists {
		t.Fatal("an existing skill directory was imported again")
	}
	if row := got["setting:dictation_pause_media"]; !row.selectable || row.key != "duckVolume" || row.value != "true" || row.pluginID != common.DictationPluginID {
		t.Fatalf("mute = %+v", row)
	}
	if row := got["setting:clipboard_text_days"]; !row.selectable || row.value != "30" || row.key != "text_history_days" {
		t.Fatalf("text days = %+v", row)
	}
	if _, exists := got["setting:clipboard_image_days"]; exists {
		t.Fatal("image retention was imported")
	}

	fresh := buildMapped(payload, defaultSnapshot())
	var skill mappedSetting
	for _, row := range fresh.general {
		if row.id == "setting:ai_skills" {
			skill = row
		}
	}
	var stored []common.Skill
	if !skill.selectable || skill.key != "AISkills" || json.Unmarshal([]byte(skill.value), &stored) != nil || len(stored) != 1 || stored[0].Path != skills || stored[0].Source != "local" {
		t.Fatalf("skills = %+v stored=%+v", skill, stored)
	}
}
