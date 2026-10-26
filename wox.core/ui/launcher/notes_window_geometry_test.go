package launcher

import (
	"testing"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
	woxui "wox/ui/runtime"
)

func TestNotesCompositionDoesNotAdvanceWindowState(t *testing.T) {
	app := bindTestApp(&App{appearance: appearanceController{palette: defaultPalette()}})
	controller := newNotesWindowController(&app.notes, common.NoteRecord{ID: "note", Document: notesplugin.ParseMarkdown("hello")})
	controller.lastFrame = woxui.Size{Width: 460, Height: 320}
	controller.requestedSize = woxui.Size{Width: 600, Height: 400}
	controller.revealEditorCaret, controller.requestTextFocus = true, true
	controller.searchOpen, controller.searchIndex = true, 7
	for range 2 {
		controller.buildNotes(woxui.FrameInfo{Size: controller.requestedSize})
	}
	if controller.lastFrame.Width != 460 || controller.requestedSize.Width != 600 || controller.manualSize || !controller.revealEditorCaret || !controller.requestTextFocus || controller.searchIndex != 7 {
		t.Fatal("view composition advanced geometry, focus requests, or search selection")
	}
	controller.observeFrame(woxui.Size{Width: 600, Height: 400})
	if controller.manualSize || controller.requestedSize != (woxui.Size{}) || controller.lastFrame.Width != 600 {
		t.Fatal("programmatic resize was not acknowledged by the frame transition")
	}
	controller.observeFrame(woxui.Size{Width: 650, Height: 450})
	if !controller.manualSize {
		t.Fatal("manual resize must stop automatic document sizing")
	}
}
