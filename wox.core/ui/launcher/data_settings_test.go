package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestOpenDataLogLevelPickerUsesSharedChoiceState(t *testing.T) {
	controller := newGeneralControllerForTest()
	controller.ApplyData(settingsData{LogLevel: "DEBUG"})
	app := &App{
		generalSettings: controller,
		translations: map[string]string{
			"ui_data_log_level_title": "Log Level",
			"ui_data_log_level_info":  "INFO",
			"ui_data_log_level_debug": "DEBUG",
		},
	}
	anchor := woxui.Rect{X: 10, Y: 20, Width: 280, Height: 34}

	app.openDataLogLevelPicker(anchor)

	picker := controller.ChoicePicker()
	if picker == nil {
		t.Fatal("log level dropdown did not open the shared choice picker")
	}
	if picker.anchor != anchor || picker.item.key != "LogLevel" || picker.item.value != "DEBUG" {
		t.Fatalf("unexpected picker state: %#v", picker)
	}
	if len(picker.item.choices) != 2 || picker.item.choices[0].value != "INFO" || picker.item.choices[1].value != "DEBUG" {
		t.Fatalf("unexpected log level choices: %#v", picker.item.choices)
	}
}
