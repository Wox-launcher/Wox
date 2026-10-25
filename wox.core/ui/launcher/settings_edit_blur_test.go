package launcher

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestBuiltInTextBlurDropsAnUnchangedEdit(t *testing.T) {
	app := bindTestApp(&App{generalSettings: newGeneralControllerForTest()})
	if !app.generalSettings.StartEdit("HttpProxyUrl", "http://localhost:7890", -1) {
		t.Fatal("StartEdit should claim the proxy URL")
	}

	app.generalSettings.blurBuiltInSettingEdit(settingItem{key: "HttpProxyUrl", value: "http://localhost:7890", text: true})

	if app.generalSettings.EditKey() != "" || app.settingsWindow.pendingBuiltInText != nil || app.settingsWindow.saving {
		t.Fatalf("edit = %q pending = %+v saving = %v, want an unchanged blur to only leave the session", app.generalSettings.EditKey(), app.settingsWindow.pendingBuiltInText, app.settingsWindow.saving)
	}
}

func TestBuiltInTextBlurDuringClickKeepsTheSwitchTap(t *testing.T) {
	app := bindTestApp(&App{generalSettings: newGeneralControllerForTest()})
	if !app.generalSettings.StartEdit("HttpProxyUrl", "sdfdf", -1) {
		t.Fatal("StartEdit should claim the proxy URL")
	}
	item := settingItem{key: "HttpProxyUrl", value: "", text: true}
	sawTap := false
	host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget {
		return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Focusable{Key: "url", OnFocusChange: func(focused bool) {
				if !focused {
					app.generalSettings.blurBuiltInSettingEdit(item)
				}
			}, Child: woxwidget.Container{Width: 40, Height: 20}},
			woxwidget.Focusable{Key: "proxy", Child: woxwidget.Gesture{ID: "proxy", OnTap: func() {
				sawTap = true
				if app.generalSettings.EditKey() != "" {
					t.Errorf("EditKey = %q, want the text session ended before the switch click", app.generalSettings.EditKey())
				}
				pending := app.settingsWindow.pendingBuiltInText
				if pending == nil || pending.key != "HttpProxyUrl" || pending.value != "sdfdf" {
					t.Errorf("pending = %+v, want deferred HttpProxyUrl sdfdf", pending)
				}
			}, Child: woxwidget.Container{Width: 40, Height: 20}}},
		}}
	})
	host.AttachServices(formTableHostServices{})
	defer host.Dispose()
	app.settingsWindow.host = host

	frame := woxui.FrameInfo{Size: woxui.Size{Width: 80, Height: 20}, PixelSize: woxui.PixelSize{Width: 80, Height: 20}, Scale: 1}
	host.Frame(&woxui.DisplayList{}, frame)
	if !host.RequestFocus("url") {
		t.Fatal("RequestFocus should focus the URL field")
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerDown, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 50, Y: 10}})
	if app.generalSettings.EditKey() != "" {
		t.Fatal("pointer down should leave the text edit before the click is released")
	}
	host.Pointer(woxui.PointerEvent{Kind: woxui.PointerUp, Button: woxui.PointerButtonPrimary, Position: woxui.Point{X: 50, Y: 10}})
	if !sawTap {
		t.Fatal("switch tap was not delivered")
	}
	if app.settingsWindow.pendingBuiltInText == nil {
		t.Fatal("releasing the switch should leave the typed URL pending until that click saves")
	}
}
