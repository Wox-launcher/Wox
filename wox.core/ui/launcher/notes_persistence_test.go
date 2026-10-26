package launcher

import (
	"testing"
	"time"

	"wox/common"
	notesplugin "wox/plugin/system/notes"
)

// TestNotesAutosaveRejectsQueuedWork simulates a timer whose UI callback has not run yet.
func TestNotesAutosaveRejectsQueuedWork(t *testing.T) {
	queued := make(chan func(), 4)
	saves := 0
	destroyed := false
	autosave := &notesAutosave{
		dispatch: func(_ string, callback func()) error { queued <- callback; return nil },
		stopped:  func() bool { return destroyed },
		save:     func() error { saves++; return nil }, invalidate: func() {},
	}
	t.Cleanup(autosave.cancelScheduledSave)
	awaitCallback := func() func() {
		t.Helper()
		select {
		case callback := <-queued:
			return callback
		case <-time.After(3 * time.Second):
			t.Fatal("autosave did not enqueue a callback")
			return nil
		}
	}
	autosave.scheduleSave()
	old := awaitCallback()
	autosave.scheduleSave()
	old()
	if saves != 0 {
		t.Fatal("superseded callback saved")
	}
	latest := awaitCallback()
	latest()
	if saves != 1 {
		t.Fatal("current callback did not save")
	}
	autosave.scheduleSave()
	canceled := awaitCallback()
	autosave.cancelScheduledSave()
	canceled()
	if saves != 1 {
		t.Fatal("canceled callback saved")
	}
	autosave.scheduleSave()
	shutdown := awaitCallback()
	destroyed = true
	shutdown()
	if saves != 1 {
		t.Fatal("callback saved after shutdown")
	}
}

func TestNotesRecordSwitchInvalidatesQueuedSave(t *testing.T) {
	services := &notesWindowTestServices{saveCall: make(chan struct{}, 1)}
	app := bindTestApp(&App{services: services, appearance: appearanceController{palette: defaultPalette()}})
	controller := newNotesWindowController(&app.notes, common.NoteRecord{ID: "first", Document: notesplugin.ParseMarkdown("first")})
	t.Cleanup(controller.cancelScheduledSave)
	queued := make(chan func(), 1)
	controller.notesAutosave.dispatch = func(_ string, callback func()) error { queued <- callback; return nil }
	controller.onEditorChanged("first edit")
	var callback func()
	select {
	case callback = <-queued:
	case <-time.After(3 * time.Second):
		t.Fatal("autosave did not enqueue a callback")
	}
	controller.applyRecord(common.NoteRecord{ID: "second", Document: notesplugin.ParseMarkdown("second")})
	// Do not schedule another edit: that would invalidate the old callback even
	// if applyRecord forgot to cancel it. Observe entry into save, including no-op flushes.
	saveAttempts := 0
	controller.notesAutosave.save = func() error { saveAttempts++; return nil }
	callback()
	if saveAttempts != 0 {
		t.Fatal("record switch did not invalidate the queued save")
	}
	select {
	case <-services.saveCall:
		t.Fatal("the previous record's callback saved the new record")
	default:
	}
	if controller.record.ID != "second" || controller.dirty {
		t.Fatal("stale callback changed the new edit session")
	}
}
