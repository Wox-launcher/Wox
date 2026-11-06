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
func (d *testWindowDriver) UpdateContent(WindowContentUpdateOption) error {
	return nil
}
func (d *testWindowDriver) ContentCommand(WindowContentCommandOption) error { return nil }
func (d *testWindowDriver) AcceptEvent(e WindowEvent) bool                  { return e.DocumentID == "current" }

// TestWindowTranslationUsesOwner verifies windows with identical keys keep separate plugin dictionaries.
func TestWindowTranslationUsesOwner(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	var translators []func(string) string
	SetWindowFactory(func(_ string, _ CreateWindowOption, emit func(WindowEvent), translate func(string) string) (WindowDriver, error) {
		translators = append(translators, translate)
		return &testWindowDriver{emit: emit}, nil
	})
	ctx, cancel := context.WithCancel(t.Context())
	for _, label := range []string{"First plugin", "Second plugin"} {
		instance := &Instance{Metadata: Metadata{I18n: map[string]map[string]string{"en_US": {"window_label": label}}}}
		api := NewAPI(instance).(*APIImpl)
		t.Cleanup(api.closeWindows)
		if result := api.CreateWindow(ctx, CreateWindowOption{}); !result.Success {
			t.Fatal(result.ErrMsg)
		}
	}
	cancel()
	for index, expected := range []string{"First plugin", "Second plugin"} {
		if actual := translators[index]("i18n:window_label"); actual != expected {
			t.Fatalf("owner %d: got %q, want %q", index, actual, expected)
		}
	}
}

// TestWindowOwnershipAndRelease covers early ready events, stale documents and plugin teardown.
func TestWindowOwnershipAndRelease(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	var driver *testWindowDriver
	SetWindowFactory(func(id string, option CreateWindowOption, emit func(WindowEvent), _ func(string) string) (WindowDriver, error) {
		driver = &testWindowDriver{emit: emit}
		emit(WindowEvent{Type: WindowEventWoxView, DocumentID: "old"})
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
	if result.Window.Content.Command(t.Context(), WindowContentCommandOption{Command: WindowContentCommandFocus, Key: "draft"}).Success {
		t.Fatal("unloaded handle accepted content command")
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
		SetWindowFactory(func(_ string, _ CreateWindowOption, emit func(WindowEvent), _ func(string) string) (WindowDriver, error) {
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

func TestWindowQueueOverflowReportsError(t *testing.T) {
	windowFactory.RLock()
	previous := windowFactory.create
	windowFactory.RUnlock()
	t.Cleanup(func() { SetWindowFactory(previous) })
	var driver *testWindowDriver
	SetWindowFactory(func(_ string, _ CreateWindowOption, emit func(WindowEvent), _ func(string) string) (WindowDriver, error) {
		driver = &testWindowDriver{emit: emit}
		return driver, nil
	})
	started, release, reported := make(chan struct{}), make(chan struct{}), make(chan WindowEvent, 1)
	var once sync.Once
	api := NewAPI(&Instance{}).(*APIImpl)
	r := api.CreateWindow(t.Context(), CreateWindowOption{OnEvent: func(_ context.Context, e WindowEvent) {
		if e.Type == WindowEventError {
			select {
			case reported <- e:
			default:
			}
			return
		}
		once.Do(func() { close(started); <-release })
	}})
	if !r.Success {
		t.Fatal(r.ErrMsg)
	}
	driver.emit(WindowEvent{Type: WindowEventWoxView, DocumentID: "current"})
	<-started
	for i := 0; i < 140; i++ {
		driver.emit(WindowEvent{Type: WindowEventWoxView, DocumentID: "current"})
	}
	close(release)
	select {
	case e := <-reported:
		if e.ErrMsg == "" || e.WindowID != r.WindowID {
			t.Fatalf("overflow: %+v", e)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("overflow not reported")
	}
	_ = r.Window.Close(t.Context(), WindowCloseOption{})
}
