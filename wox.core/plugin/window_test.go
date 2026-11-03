package plugin

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type testWindowDriver struct {
	emit func(WindowEvent)
	stop sync.Once
}

func (d *testWindowDriver) Show(WindowShowOption) error { return nil }
func (d *testWindowDriver) Hide() error                 { return nil }
func (d *testWindowDriver) Close() error {
	d.stop.Do(func() { d.emit(WindowEvent{Type: WindowEventClosed}) })
	return nil
}
func (d *testWindowDriver) SetOptions(WindowSetOptionsOption) error { return nil }
func (d *testWindowDriver) SetContent(WindowContent) error          { return nil }
func (d *testWindowDriver) PostMessage(WindowMessageOption) error   { return nil }
func (d *testWindowDriver) AcceptEvent(e WindowEvent) bool          { return e.DocumentID == "current" }

// TestWindowOwnershipAndRelease covers early ready events, stale documents and plugin teardown.
func TestWindowOwnershipAndRelease(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	var driver *testWindowDriver
	SetWindowFactory(func(id string, option CreateWindowOption, emit func(WindowEvent)) (WindowDriver, error) {
		driver = &testWindowDriver{emit: emit}
		emit(WindowEvent{Type: WindowEventMessage, DocumentID: "old"})
		emit(WindowEvent{Type: WindowEventReady, DocumentID: "current"})
		return driver, nil
	})
	api := NewAPI(&Instance{}).(*APIImpl)
	other := NewAPI(&Instance{}).(*APIImpl)
	events := make(chan WindowEvent, 8)
	released := make(chan struct{}, 1)
	result := api.CreateWindow(t.Context(), CreateWindowOption{
		OnEvent: func(_ context.Context, event WindowEvent) { events <- event },
		Release: func() { released <- struct{}{} },
	})
	if !result.Success || result.Window == nil {
		t.Fatalf("create: %+v", result)
	}
	if _, ok := other.PluginWindowByID(result.WindowID); ok {
		t.Fatal("cross-plugin window lookup")
	}
	select {
	case event := <-events:
		if event.Type != WindowEventReady || event.WindowID != result.WindowID {
			t.Fatalf("event: %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("missing early ready event")
	}
	api.closeWindows()
	if api.CreateWindow(t.Context(), CreateWindowOption{}).Success {
		t.Fatal("suspended plugin created a window")
	}
	if result.Window.Show(t.Context(), WindowShowOption{}).Success {
		t.Fatal("unloaded handle remained usable")
	}
	if _, ok := api.PluginWindowByID(result.WindowID); ok {
		t.Fatal("closed window retained")
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("callback not released")
	}
	select {
	case event := <-events:
		t.Fatalf("unload delivered stale event: %+v", event)
	default:
	}
	if !result.Window.Close(t.Context(), WindowCloseOption{}).Success {
		t.Fatal("close is not idempotent")
	}
}

// TestWindowCreationFailureAndEarlyClose verifies cleanup even before the registry is populated.
func TestWindowCreationFailureAndEarlyClose(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	for _, fail := range []bool{false, true} {
		released := make(chan struct{}, 1)
		SetWindowFactory(func(_ string, _ CreateWindowOption, emit func(WindowEvent)) (WindowDriver, error) {
			if fail {
				return nil, errors.New("native failure")
			}
			d := &testWindowDriver{emit: emit}
			_ = d.Close()
			return d, nil
		})
		api := NewAPI(&Instance{}).(*APIImpl)
		r := api.CreateWindow(t.Context(), CreateWindowOption{Release: func() { released <- struct{}{} }})
		if r.Success == fail {
			t.Fatalf("unexpected result: %+v", r)
		}
		select {
		case <-released:
		case <-time.After(time.Second):
			t.Fatal("release not called")
		}
		if len(api.windows.Keys()) != 0 {
			t.Fatal("creation leaked registry entry")
		}
	}
}
