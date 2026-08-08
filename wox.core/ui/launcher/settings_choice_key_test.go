package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
)

func TestFilterableSettingChoiceLeavesPrintableKeysForTextInput(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(value string) string { return value }}
	shared := newSharedEditState()
	app := &App{
		generalSettings: newGeneralSettingsController(deps, shared),
		aiSettings:      newAISettingsController(deps),
		cloudSettings:   newCloudSettingsController(deps),
	}
	app.generalSettings.SetChoicePicker(&settingChoicePickerState{item: settingItem{filterable: true}})

	if app.onSettingsKey(woxui.KeyEvent{Key: woxui.Key("p"), Down: true}) {
		t.Fatal("filterable choice consumed a printable key before native text input")
	}
	if app.generalSettings.ChoicePicker() == nil {
		t.Fatal("printable key unexpectedly closed the choice picker")
	}
}

func TestPlainSettingChoiceStillTrapsUnhandledKeys(t *testing.T) {
	deps := CommonDeps{Invalidate: func() {}, Translate: func(value string) string { return value }}
	shared := newSharedEditState()
	app := &App{
		generalSettings: newGeneralSettingsController(deps, shared),
		aiSettings:      newAISettingsController(deps),
		cloudSettings:   newCloudSettingsController(deps),
	}
	app.generalSettings.SetChoicePicker(&settingChoicePickerState{item: settingItem{filterable: false}})

	if !app.onSettingsKey(woxui.KeyEvent{Key: woxui.Key("p"), Down: true}) {
		t.Fatal("plain choice should keep unhandled keys trapped inside the modal menu")
	}
}
