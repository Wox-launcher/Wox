package launcher

import (
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

func TestUsePinYinDoesNotReenterAppLock(t *testing.T) {
	controller := newGeneralControllerForTest()
	controller.ApplyData(settingsData{UsePinYin: true})
	app := &App{generalSettings: controller}

	app.mu.Lock()
	done := make(chan bool, 1)
	go func() {
		done <- app.usePinYin()
	}()

	select {
	case got := <-done:
		app.mu.Unlock()
		if !got {
			t.Fatal("usePinYin() = false, want true")
		}
	case <-time.After(time.Second):
		app.mu.Unlock()
		t.Fatal("usePinYin blocked while the caller held App.mu")
	}
}

func TestOpenSettingChoicePickerEndsEditBeforeOpeningPicker(t *testing.T) {
	controller := newGeneralControllerForTest()
	if !controller.StartEdit("CustomPythonPath", "/usr/bin/python3", -1) {
		t.Fatal("StartEdit should claim the shared editor")
	}
	app := &App{generalSettings: controller}
	item := settingItem{
		key:        "LangCode",
		title:      "Language",
		filterable: true,
		choices:    []settingChoice{{value: "en", label: "English"}},
	}

	app.openSettingChoicePickerAt(item, woxui.Rect{X: 10, Y: 20, Width: 100, Height: 24})

	if got := controller.EditKey(); got != "" {
		t.Fatalf("EditKey() = %q, want empty after opening choice picker", got)
	}
	picker := controller.ChoicePicker()
	if picker == nil {
		t.Fatal("choice picker was cleared while ending the previous text edit")
	}
	if picker.item.key != "LangCode" {
		t.Fatalf("choice picker item = %q, want LangCode", picker.item.key)
	}
}

func TestThemeEditorPreviewSnapshotWaitsForAppLock(t *testing.T) {
	result := queryResult{QueryID: "query", ID: "result"}
	preview := queryPreview{PreviewData: `{"ThemeName":"Test","AppBackgroundColor":"#000000"}`}
	raw, key, err := themeEditorPreviewDataAndKey(result, preview)
	if err != nil {
		t.Fatalf("prepare theme editor preview: %v", err)
	}
	controller := newThemeSettingsController(CommonDeps{})
	controller.SetThemeEditor(newThemeEditorState(key, raw))
	app := &App{themeSettings: controller}

	started := make(chan struct{})
	done := make(chan error, 1)
	app.mu.Lock()
	go func() {
		close(started)
		_, snapshotErr := app.themeEditorPreviewSnapshotFor(result, preview)
		done <- snapshotErr
	}()
	<-started

	select {
	case snapshotErr := <-done:
		app.mu.Unlock()
		t.Fatalf("snapshot read mutable theme state without App.mu: %v", snapshotErr)
	case <-time.After(50 * time.Millisecond):
	}
	app.mu.Unlock()

	select {
	case snapshotErr := <-done:
		if snapshotErr != nil {
			t.Fatalf("snapshot theme editor preview: %v", snapshotErr)
		}
	case <-time.After(time.Second):
		t.Fatal("theme editor snapshot did not resume after App.mu was released")
	}
}
