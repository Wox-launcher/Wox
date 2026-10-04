package system

import (
	"context"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
	"wox/common/icons"
	"wox/plugin"
	"wox/setting"
	"wox/setting/definition"
	"wox/util/screenshotedit"
)

type screenshotSceneTestAPI struct {
	plugin.API
	refreshed chan bool
	notified  chan string
}

func (api *screenshotSceneTestAPI) RefreshQuery(_ context.Context, options plugin.RefreshQueryParam) {
	api.refreshed <- options.PreserveSelectedIndex
}

func (api *screenshotSceneTestAPI) Notify(_ context.Context, message string) {
	api.notified <- message
}

func TestScreenshotSceneSaveRunsInBackgroundAndFinishesBeforeUnload(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	p := &ScreenshotPlugin{backgroundCtx: ctx, backgroundCancel: cancel}
	started, release, stopped := make(chan struct{}), make(chan struct{}), make(chan struct{})
	defer cancel()
	returned := make(chan struct{})
	go func() {
		p.saveScreenshotSceneInBackground("capture.jpg", func() error { close(started); <-release; return nil })
		close(returned)
	}()
	select {
	case <-returned:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("capture completion waited for scene encoding")
	}
	<-started
	go func() { p.stopScreenshotBackgroundTasks(); close(stopped) }()
	<-ctx.Done()
	select {
	case <-stopped:
		t.Fatal("unload returned while scene data was still being written")
	default:
	}
	close(release)
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("unload did not finish after scene commit")
	}
}

func TestScreenshotSceneBackgroundCompletionRefreshesOrReportsFailure(t *testing.T) {
	for _, fail := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		api := &screenshotSceneTestAPI{refreshed: make(chan bool, 1), notified: make(chan string, 1)}
		p := &ScreenshotPlugin{api: api, backgroundCtx: ctx, backgroundCancel: cancel}
		p.saveScreenshotSceneInBackground("capture.jpg", func() error {
			if fail {
				return errors.New("disk full")
			}
			return nil
		})
		p.backgroundWG.Wait()
		if fail {
			select {
			case message := <-api.notified:
				if message != "i18n:plugin_screenshot_edit_save_failed" {
					t.Fatal(message)
				}
			default:
				t.Fatal("background scene failure was silent")
			}
		} else {
			select {
			case preserve := <-api.refreshed:
				if !preserve {
					t.Fatal("scene completion reset selection")
				}
			default:
				t.Fatal("ready scene did not refresh edit actions")
			}
		}
		p.stopScreenshotBackgroundTasks()
	}
}

func TestScreenshotHistoryEditActionRequiresScene(t *testing.T) {
	path := filepath.Join(t.TempDir(), "capture.jpg")
	if err := os.WriteFile(path, []byte("image"), 0600); err != nil {
		t.Fatal(err)
	}
	p := &ScreenshotPlugin{}
	item := screenshotHistoryItem{path: path, fileName: "capture.jpg", size: 5}
	assertEditable := func(want bool) {
		t.Helper()
		result := p.screenshotHistoryResult(item)
		found := false
		for _, action := range result.Actions {
			if action.Name == "i18n:plugin_screenshot_history_edit" {
				found = true
				if action.IsDefault {
					t.Fatal("edit replaced the copy default")
				}
			}
		}
		if found != want {
			t.Fatalf("edit action = %v, want %v", found, want)
		}
	}
	assertEditable(false)
	if err := os.WriteFile(path+screenshotedit.Suffix, []byte("scene"), 0600); err != nil {
		t.Fatal(err)
	}
	assertEditable(true)
	if err := screenshotedit.Remove(path); err != nil {
		t.Fatal(err)
	}
	assertEditable(false)
}

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
