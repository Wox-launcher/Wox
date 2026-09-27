package launcher

import (
	"errors"
	"reflect"
	"testing"

	"wox/setting"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/screen"
)

func TestShowDisplayButtonLabel(t *testing.T) {
	translate := func(key string) string {
		switch key {
		case "i18n:ui_show_position_choose_screen":
			return "Choose screen"
		case "i18n:ui_show_position_screen":
			return "Screen"
		case "i18n:ui_show_position_screen_primary":
			return "Primary · {width}×{height}"
		case "i18n:ui_show_position_screen_numbered":
			return "Screen {index} · {width}×{height}"
		case "i18n:ui_show_position_screen_disconnected":
			return "Screen unavailable"
		default:
			return key
		}
	}
	displays := []screen.Display{
		{ID: "main", Primary: true, Bounds: screen.Rect{Width: 1920, Height: 1080}, WorkArea: screen.Rect{X: 0, Y: 40, Width: 1920, Height: 1040}},
		{ID: "side", Bounds: screen.Rect{X: 1920, Width: 2560, Height: 1440}, WorkArea: screen.Rect{X: 1920, Y: 0, Width: 2560, Height: 1400}},
	}
	if got := showDisplayButtonLabel(setting.ShowDisplayTarget{}, displays, true, true, translate); got != "Choose screen" {
		t.Fatalf("empty choice = %q", got)
	}
	saved := setting.ShowDisplayTarget{ID: "side", WorkX: 1920, WorkY: 0, WorkWidth: 2560, WorkHeight: 1400}
	if got := showDisplayButtonLabel(saved, displays, true, true, translate); got != "Screen 2 · 2560×1440" {
		t.Fatalf("side screen = %q", got)
	}
	if got := showDisplayButtonLabel(saved, nil, false, true, translate); got != "Screen · 2560×1400" {
		t.Fatalf("unloaded screen = %q", got)
	}
	missing := setting.ShowDisplayTarget{ID: "gone", WorkX: 4000, WorkY: 0, WorkWidth: 800, WorkHeight: 600}
	if got := showDisplayButtonLabel(missing, displays, true, true, translate); got != "Screen unavailable" {
		t.Fatalf("missing screen = %q", got)
	}
	scaled := []screen.Display{{
		ID: "side", Primary: false,
		Bounds:      screen.Rect{X: 2194, Width: 1097, Height: 685},
		PixelBounds: screen.Rect{X: 3840, Width: 1920, Height: 1200},
		WorkArea:    screen.Rect{X: 2194, Width: 1097, Height: 685},
	}}
	if got := showDisplayButtonLabel(setting.ShowDisplayTarget{ID: "side", WorkX: 2194, WorkWidth: 1097, WorkHeight: 685}, scaled, true, true, translate); got != "Screen 1 · 1920×1200" {
		t.Fatalf("physical resolution = %q", got)
	}
}

func TestShowDisplayMapRectUsesPhysicalDesktop(t *testing.T) {
	primary := screen.Display{
		Bounds:      screen.Rect{Width: 1707, Height: 960},
		PixelBounds: screen.Rect{Width: 3840, Height: 2160},
	}
	side := screen.Display{
		Bounds:      screen.Rect{X: 2194, Width: 1097, Height: 685},
		PixelBounds: screen.Rect{X: 3840, Width: 1920, Height: 1200},
	}
	if showDisplayMapRect(primary, true).Right() != showDisplayMapRect(side, true).X {
		t.Fatalf("map rects = %+v %+v, want the physical edges to meet", showDisplayMapRect(primary, true), showDisplayMapRect(side, true))
	}
}

func TestShowDisplayMapRectUsesLinuxLogicalDesktop(t *testing.T) {
	primary := screen.Display{Bounds: screen.Rect{Width: 1920, Height: 1080}, PixelBounds: screen.Rect{Width: 3840, Height: 2160}}
	side := screen.Display{Bounds: screen.Rect{X: 1920, Width: 1920, Height: 1080}, PixelBounds: screen.Rect{X: 1920, Width: 1920, Height: 1080}}
	if showDisplayMapRect(primary, false).Right() != showDisplayMapRect(side, false).X {
		t.Fatalf("logical map rects = %+v %+v, want adjacent tiles", showDisplayMapRect(primary, false), showDisplayMapRect(side, false))
	}
}

func TestFailedShowDisplayLoadWaitsForExplicitRetry(t *testing.T) {
	app := newApp(false, nil, woxui.NewWindowManager(), newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.generalSettings.ApplyShowDisplays(nil, errors.New("enumeration failed"))
	app.ensureShowDisplays()
	if app.generalSettings.ShowDisplaysLoading() {
		t.Fatal("a failed display read should not retry during page construction")
	}
}

func TestAppearanceSpecificScreenAddsChooser(t *testing.T) {
	windows := woxui.NewWindowManager()
	app := newApp(false, nil, windows, newAppInstanceRegistry(), nil, true, "", launcherWindowID)
	defer app.cancel()
	app.translations = map[string]string{
		"ui_show_position_screen":        "Screen",
		"ui_show_position_screen_tips":   "Choose the display Wox opens on",
		"ui_show_position_choose_screen": "Choose screen",
	}
	data := settingsData{ShowPosition: "specific_screen"}
	app.generalSettings.ApplyShowDisplays(nil, nil)
	page := app.buildSettingsPage(settingsSnapshot{
		tab: "appearance", palette: settingsPalette(),
		general: generalSettingsSnapshot{Data: data, ShowDisplaysLoaded: true},
	}, settingItems("appearance", data), 800, 600, 1)
	button, ok := findSemantics(page, "setting-button-ShowDisplay")
	if !ok || button.Value != "Choose screen" {
		t.Fatalf("chooser = %#v found %v", button, ok)
	}

	plain := app.buildSettingsPage(settingsSnapshot{
		tab: "appearance", palette: settingsPalette(),
		general: generalSettingsSnapshot{Data: settingsData{ShowPosition: "mouse_screen"}, ShowDisplaysLoaded: true},
	}, settingItems("appearance", settingsData{ShowPosition: "mouse_screen"}), 800, 600, 1)
	if _, ok := findSemantics(plain, "setting-button-ShowDisplay"); ok {
		t.Fatal("chooser should appear only for a specific screen")
	}
}

func findSemantics(root woxwidget.Widget, id string) (woxwidget.Semantics, bool) {
	var found woxwidget.Semantics
	var ok bool
	var walk func(reflect.Value)
	walk = func(value reflect.Value) {
		if ok || !value.IsValid() {
			return
		}
		switch value.Kind() {
		case reflect.Interface, reflect.Pointer:
			if value.IsNil() {
				return
			}
			walk(value.Elem())
		case reflect.Slice, reflect.Array:
			for index := 0; index < value.Len(); index++ {
				walk(value.Index(index))
			}
		case reflect.Struct:
			if value.CanInterface() {
				if semantics, isSemantics := value.Interface().(woxwidget.Semantics); isSemantics && semantics.AutomationID == id {
					found = semantics
					ok = true
					return
				}
			}
			for index := 0; index < value.NumField(); index++ {
				field := value.Field(index)
				if field.CanInterface() {
					walk(field)
				}
			}
		}
	}
	walk(reflect.ValueOf(root))
	return found, ok
}
