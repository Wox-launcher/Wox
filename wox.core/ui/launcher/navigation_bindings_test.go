package launcher

import (
	"errors"
	"runtime"
	"testing"

	"wox/setting"
	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

func TestNavigationDirections(t *testing.T) {
	for _, platform := range []string{"windows", "linux", "darwin"} {
		modifier := woxui.KeyModifierAlt
		if platform == "darwin" {
			modifier = woxui.KeyModifierControl
		}
		for _, tc := range []struct {
			mode setting.NavigationBindingMode
			key  woxui.Key
			want woxui.Key
		}{
			{setting.NavigationBindingModeEmacs, "p", woxui.KeyArrowUp}, {setting.NavigationBindingModeEmacs, "n", woxui.KeyArrowDown},
			{setting.NavigationBindingModeEmacs, "b", woxui.KeyArrowLeft}, {setting.NavigationBindingModeEmacs, "f", woxui.KeyArrowRight},
			{setting.NavigationBindingModeVim, "k", woxui.KeyArrowUp}, {setting.NavigationBindingModeVim, "j", woxui.KeyArrowDown},
			{setting.NavigationBindingModeVim, "h", woxui.KeyArrowLeft}, {setting.NavigationBindingModeVim, "l", woxui.KeyArrowRight},
		} {
			event := woxui.KeyEvent{Key: tc.key, Modifiers: modifier, Down: true, Repeat: true}
			if got := navigationDirectionForPlatform(tc.mode, event, platform); got != tc.want {
				t.Fatalf("%s %s %s = %s, want %s", platform, tc.mode, tc.key, got, tc.want)
			}
			for _, excluded := range []woxui.KeyEvent{
				{Key: tc.key, Modifiers: modifier, Down: true, Composing: true},
				{Key: tc.key, Modifiers: modifier | woxui.KeyModifierShift, Down: true},
				{Key: tc.key, Modifiers: modifier | woxui.KeyModifierMeta, Down: true},
				{Key: tc.key, Modifiers: woxui.KeyModifierControl | woxui.KeyModifierAlt, Down: true},
				{Key: tc.key, Modifiers: modifier, Down: false},
			} {
				if got := navigationDirectionForPlatform(tc.mode, excluded, platform); got != "" {
					t.Fatalf("excluded event %+v mapped to %s", excluded, got)
				}
			}
			if got := navigationDirectionForPlatform(setting.NavigationBindingModeOff, event, platform); got != "" {
				t.Fatalf("disabled navigation mapped to %s", got)
			}
		}
	}
}

func TestNavigationSelectsResultsWithoutEditingQuery(t *testing.T) {
	app := bindTestApp(&App{
		querySession: queryController{editor: woxui.NewTextEditor("query"), canRecallHistory: true},
		resultList:   resultController{results: []queryResult{{ID: "one"}, {ID: "two"}, {ID: "three"}}},
		uiCall:       func(func()) error { return errors.New("no native window") },
	})
	app.generalSettings.ApplyData(settingsData{NavigationBindings: setting.NavigationBindingModeVim})
	modifier := woxui.KeyModifierAlt
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierControl
	}
	if !app.onKey(woxui.KeyEvent{Key: "j", Modifiers: modifier, Down: true}) || app.resultList.selected != 1 {
		t.Fatal("Vim down should select the next result")
	}
	if !app.onKey(woxui.KeyEvent{Key: "k", Modifiers: modifier, Down: true}) || app.resultList.selected != 0 {
		t.Fatal("Vim up should select the previous result")
	}
	if app.querySession.editor.State().Text != "query" || app.querySession.canRecallHistory {
		t.Fatal("result navigation should not edit the query or recall history")
	}
	before := app.querySession.editor.State()
	app.onKey(woxui.KeyEvent{Key: "h", Modifiers: modifier, Down: true})
	if after := app.querySession.editor.State(); after.Selection != before.Selection || after.Text != before.Text || app.resultList.selected != 0 {
		t.Fatal("horizontal bindings should not navigate a list or move the query caret")
	}
	if app.onKey(woxui.KeyEvent{Key: "j", Modifiers: modifier, Down: true, Composing: true}) || app.resultList.selected != 0 {
		t.Fatal("composition must retain the key without selecting a result")
	}
	app.resultList.layout = queryLayout{GridLayout: &gridLayout{Columns: 2}}
	if !app.onKey(woxui.KeyEvent{Key: "l", Modifiers: modifier, Down: true}) || app.resultList.selected != 1 {
		t.Fatal("Vim right should select the next grid result")
	}
	if app.querySession.editor.State().Text != "query" {
		t.Fatal("grid navigation must not edit the query")
	}
	app.actions.actionPanel = true
	app.actions.actionFilter = woxui.NewTextEditor("")
	app.onKey(woxui.KeyEvent{Key: "h", Modifiers: modifier, Down: true})
	if app.resultList.selected != 1 {
		t.Fatal("horizontal navigation in the action panel must not move the underlying grid")
	}
}

func TestNavigationSettingsOnHotkeyPage(t *testing.T) {
	data, err := settingsDataFromContract(contract.GeneralSettings{NavigationBindings: setting.NavigationBindingModeVim})
	if err != nil || data.NavigationBindings != setting.NavigationBindingModeVim {
		t.Fatalf("loaded navigation settings = %+v, %v", data, err)
	}
	for _, item := range settingItems("hotkey", data) {
		if item.key == "NavigationBindings" {
			if item.value != string(setting.NavigationBindingModeVim) || len(item.choices) != 3 || item.choices[0].value != string(setting.NavigationBindingModeOff) || item.choices[1].value != string(setting.NavigationBindingModeEmacs) || item.choices[2].value != string(setting.NavigationBindingModeVim) {
				t.Fatalf("navigation choices = %+v", item)
			}
			modifier := "Alt"
			if runtime.GOOS == "darwin" {
				modifier = "Ctrl"
			}
			if settingValueLabel(item) != "Vim" || item.choices[1].label != "Emacs" || item.choiceDescriptions[string(setting.NavigationBindingModeEmacs)] != modifier+" + B/F/P/N" || item.choiceDescriptions[string(setting.NavigationBindingModeVim)] != modifier+" + H/J/K/L" {
				t.Fatalf("navigation labels or descriptions = %+v", item)
			}
			return
		}
	}
	t.Fatal("navigation setting is missing from the Hotkey page")
}

func TestNavigationInActionPanel(t *testing.T) {
	app := bindTestApp(&App{
		actions:    actionPanelController{actionPanel: true, actionFilter: woxui.NewTextEditor("")},
		resultList: resultController{results: []queryResult{{Actions: []resultAction{{ID: "open", Name: "Open"}, {ID: "copy", Name: "Copy"}}}}},
	})
	app.generalSettings.ApplyData(settingsData{NavigationBindings: setting.NavigationBindingModeVim})
	modifier := woxui.KeyModifierAlt
	if runtime.GOOS == "darwin" {
		modifier = woxui.KeyModifierControl
		app.resultList.results[0].Actions[0].Hotkey = "ctrl+j"
	} else {
		app.resultList.results[0].Actions[0].Hotkey = "alt+j"
	}
	if !app.actions.onActionKey(woxui.KeyEvent{Key: "j", Modifiers: modifier, Down: true}) || app.actions.actionSelected != 1 {
		t.Fatal("Vim down should select the next action")
	}
	if app.actions.onActionKey(woxui.KeyEvent{Key: "h", Modifiers: modifier, Down: true}) {
		t.Fatal("horizontal navigation must not move the action filter caret")
	}
}
