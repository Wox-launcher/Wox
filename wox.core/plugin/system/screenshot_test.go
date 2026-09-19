package system

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"wox/common/icons"
	"wox/setting"
	"wox/setting/definition"
)

func TestScreenshotHistoryThumbnailHasWidth(t *testing.T) {
	path := filepath.Join(t.TempDir(), "preview.png")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := png.Encode(file, image.NewRGBA(image.Rect(0, 0, 400, 200))); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	if !screenshotHistoryThumbnailHasWidth(path, 400) {
		t.Fatal("matching thumbnail width must remain valid")
	}
	if screenshotHistoryThumbnailHasWidth(path, 1024) {
		t.Fatal("stale thumbnail width must be invalidated")
	}
}

func TestScreenshotHistoryImageExtensions(t *testing.T) {
	for _, path := range []string{"capture.png", "capture.jpg", "capture.JPEG"} {
		if !isScreenshotHistoryImage(path) {
			t.Fatalf("screenshot history rejected %s", path)
		}
	}
	if isScreenshotHistoryImage("capture.webp") {
		t.Fatal("screenshot history accepted an unsupported image")
	}
}

func TestScreenshotHistoryResultIncludesNotesAction(t *testing.T) {
	result := (&ScreenshotPlugin{}).screenshotHistoryResult(screenshotHistoryItem{
		path:      "/tmp/capture.png",
		fileName:  "capture.png",
		size:      128,
		timestamp: 1,
		ocrText:   "hello",
	})
	found := false
	for _, action := range result.Actions {
		if action.Name == "i18n:plugin_notes_action_save" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("screenshot history actions = %#v", result.Actions)
	}
}

func TestScreenshotAIToolbarSettingIsLastInAIGroup(t *testing.T) {
	settings := (&ScreenshotPlugin{}).GetMetadata().SettingDefinitions
	if len(settings) < 2 {
		t.Fatal("screenshot settings are missing the AI group")
	}
	head, ok := settings[len(settings)-2].Value.(*definition.PluginSettingValueHead)
	if !ok || head.Content != "i18n:plugin_screenshot_ai_head" {
		t.Fatalf("AI group head = %#v", settings[len(settings)-2].Value)
	}
	checkbox, ok := settings[len(settings)-1].Value.(*definition.PluginSettingValueCheckBox)
	if !ok || checkbox.Key != screenshotAIToolbarEnabledSettingKey {
		t.Fatalf("last setting = %#v", settings[len(settings)-1].Value)
	}
	if checkbox.Label != "i18n:plugin_screenshot_ai_toolbar" {
		t.Fatalf("AI toolbar label = %q", checkbox.Label)
	}
}

func TestScreenshotAIToolbarActionsRequiresSettingAndProvider(t *testing.T) {
	actions := screenshotAIToolbarActions(true, true, "Send to AI Chat")
	if len(actions) != 1 || actions[0].ID != screenshotAIExtraActionID || actions[0].Icon != icons.ControlSparkles {
		t.Fatalf("enabled actions = %#v", actions)
	}
	if screenshotAIToolbarActions(false, true, "Send to AI Chat") != nil {
		t.Fatal("disabled setting should hide the AI button")
	}
	if screenshotAIToolbarActions(true, false, "Send to AI Chat") != nil {
		t.Fatal("missing AI provider should hide the AI button")
	}
	if hasConfiguredAIProvider(nil) || hasConfiguredAIProvider([]setting.AIProvider{}) {
		t.Fatal("empty provider list should count as unconfigured")
	}
	if !hasConfiguredAIProvider([]setting.AIProvider{{Name: "openai"}}) {
		t.Fatal("a configured provider should count as available")
	}
}

func TestNewScreenshotActionAllowsLauncherHide(t *testing.T) {
	result := (&ScreenshotPlugin{}).newScreenshotResult()
	if len(result.Actions) != 1 {
		t.Fatalf("screenshot action count = %d", len(result.Actions))
	}
	if result.Actions[0].PreventHideAfterAction {
		t.Fatal("new screenshot action must allow the launcher to hide")
	}
}
