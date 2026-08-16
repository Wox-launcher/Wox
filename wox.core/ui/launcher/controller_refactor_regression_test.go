package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestUsePinYinReadsGeneralController(t *testing.T) {
	controller := newGeneralControllerForTest()
	controller.ApplyData(settingsData{UsePinYin: true})
	app := &App{generalSettings: controller}

	if !app.usePinYin() {
		t.Fatal("usePinYin() = false, want true")
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
