package plugin

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"wox/util"

	"github.com/google/uuid"
)

type WindowContentType string

const WindowContentWoxView WindowContentType = "woxview"

type WindowPresentation string

const (
	WindowPresentationOverlay WindowPresentation = "overlay"
	WindowPresentationWindow  WindowPresentation = "window"
)

type WindowAnchor string

const (
	WindowAnchorTopLeft      WindowAnchor = "top-left"
	WindowAnchorTopCenter    WindowAnchor = "top-center"
	WindowAnchorTopRight     WindowAnchor = "top-right"
	WindowAnchorLeftCenter   WindowAnchor = "left-center"
	WindowAnchorCenter       WindowAnchor = "center"
	WindowAnchorRightCenter  WindowAnchor = "right-center"
	WindowAnchorBottomLeft   WindowAnchor = "bottom-left"
	WindowAnchorBottomCenter WindowAnchor = "bottom-center"
	WindowAnchorBottomRight  WindowAnchor = "bottom-right"
)

type WindowEventType string

const (
	WindowEventReady  WindowEventType = "ready"
	WindowEventError  WindowEventType = "error"
	WindowEventClosed WindowEventType = "closed"
	// WindowEventWoxView carries {"Handler","Args","Key"} for a callback registered in a woxview tree.
	WindowEventWoxView WindowEventType = "woxview_event"
	// WindowEventThemeChanged carries WindowThemeChangedData in Data.
	WindowEventThemeChanged WindowEventType = "theme_changed"
)

// WindowThemeChangedData describes the plugin window's current surface appearance.
type WindowThemeChangedData struct{ Dark bool }

// WindowContent contains a serialized WoxView tree (JSON).
type WindowContent struct {
	Type WindowContentType
	Data string
}

// WindowOptions uses logical units. Nil fields select the documented defaults.
type WindowOptions struct {
	Title                                            string
	Presentation                                     WindowPresentation
	Width, Height                                    *float64
	Anchor                                           WindowAnchor
	OffsetX, OffsetY                                 *float64
	Topmost, Focusable, Resizable, Movable, Closable *bool
	HideChrome                                       *bool
}
type CreateWindowOption struct {
	Content WindowContent
	Window  WindowOptions
	OnEvent func(context.Context, WindowEvent) `json:"-"`
	// Release is host bookkeeping, called after event delivery has stopped.
	Release func() `json:"-"`
}
type WindowEvent struct {
	// CoalesceKey is an internal producer hint, never part of the SDK event payload.
	CoalesceKey string `json:"-"`
	Type        WindowEventType
	WindowID    string
	DocumentID  string
	Data        json.RawMessage
	ErrMsg      string
}
type WindowResult struct {
	Success bool
	ErrMsg  string
}
type CreateWindowResult struct {
	WindowResult
	Window   *PluginWindow `json:"-"`
	WindowID string
}
type WindowShowOption struct{ Activate bool }
type WindowHideOption struct{}
type WindowCloseOption struct{}
type WindowSetOptionsOption struct {
	Width, Height    *float64
	Anchor           WindowAnchor
	OffsetX, OffsetY *float64
	Topmost          *bool
}
type WindowContentSetOption struct{ Content WindowContent }

// WindowContentUpdateOption replaces the woxview tree in place, keeping the document and widget state.
type WindowContentUpdateOption struct {
	Data string
	// DocumentID optionally rejects an update intended for a replaced document.
	DocumentID string
	// Sequence is an optional increasing per-document update number.
	Sequence uint64
}

// WindowContentCommand names an imperative WoxView operation.
type WindowContentCommand string

const (
	WindowContentCommandFocus     WindowContentCommand = "focus"
	WindowContentCommandScrollTo  WindowContentCommand = "scrollTo"
	WindowContentCommandSelectAll WindowContentCommand = "selectAll"
)

// WindowContentCommandOption targets a unique node key in the current document.
type WindowContentCommandOption struct {
	DocumentID string
	Command    WindowContentCommand
	Key        string
}
type WindowShowResult struct{ WindowResult }
type WindowHideResult struct{ WindowResult }
type WindowCloseResult struct{ WindowResult }
type WindowSetOptionsResult struct{ WindowResult }
type WindowContentSetResult struct{ WindowResult }
type WindowContentUpdateResult struct{ WindowResult }
type WindowContentCommandResult struct{ WindowResult }

// WindowDriver is the narrow native capability implemented by the UI feature.
type WindowDriver interface {
	Show(WindowShowOption) error
	Hide() error
	Close() error
	SetOptions(WindowSetOptionsOption) error
	SetContent(WindowContent) error
	UpdateContent(WindowContentUpdateOption) error
	ContentCommand(WindowContentCommandOption) error
	AcceptEvent(WindowEvent) bool
}

// WindowFactory receives host capabilities separately from the public creation options.
type WindowFactory func(id string, option CreateWindowOption, emit func(WindowEvent), translate func(string) string) (WindowDriver, error)

var windowFactory struct {
	sync.RWMutex
	create WindowFactory
}

// SetWindowFactory installs the UI capability at startup without importing UI into plugin infrastructure.
func SetWindowFactory(factory WindowFactory) {
	windowFactory.Lock()
	windowFactory.create = factory
	windowFactory.Unlock()
}

// PluginWindow is an instance-bound capability; it never exposes native window handles.
type PluginWindow struct {
	ID      string
	Content *PluginWindowContent
	owner   *APIImpl
	epoch   uint64
	driver  WindowDriver
}
type PluginWindowContent struct{ window *PluginWindow }

func windowResult(err error) WindowResult {
	if err != nil {
		return WindowResult{ErrMsg: err.Error()}
	}
	return WindowResult{Success: true}
}

// CreateWindow creates a hidden instance and installs callbacks before native content loads.
func (a *APIImpl) CreateWindow(ctx context.Context, option CreateWindowOption) CreateWindowResult {
	if a.pluginInstance == nil || a.windowSuspended.Load() {
		return CreateWindowResult{WindowResult: windowResult(errors.New("plugin instance unavailable"))}
	}
	windowFactory.RLock()
	factory := windowFactory.create
	windowFactory.RUnlock()
	if factory == nil {
		return CreateWindowResult{WindowResult: windowResult(errors.New("window UI unavailable"))}
	}
	a.windowOnce.Do(func() { a.windows = util.NewHashMap[string, *PluginWindow]() })
	w := &PluginWindow{ID: uuid.NewString(), owner: a, epoch: a.windowEpoch.Load()}
	w.Content = &PluginWindowContent{window: w}
	// The queue keeps callbacks off the UI thread and coalesces producer-marked changes.
	events := newWindowEventQueue()
	done := make(chan struct{})
	var stop sync.Once
	var overflow atomic.Bool
	emit := func(event WindowEvent) {
		event.WindowID = w.ID
		if event.Type == WindowEventClosed {
			events.close()
			a.windows.Delete(w.ID)
			stop.Do(func() { close(done) })
			return
		}
		if !events.push(event) {
			overflow.Store(true)
			util.GetLogger().Warn(ctx, "plugin window event queue full")
		}
	}
	// Retained windows outlive the query that created them; translation follows the current locale.
	translationContext := context.WithoutCancel(ctx)
	driver, err := factory(w.ID, option, emit, func(text string) string { return a.GetTranslation(translationContext, text) })
	if err != nil {
		if option.Release != nil {
			option.Release()
		}
		return CreateWindowResult{WindowResult: windowResult(err)}
	}
	w.driver = driver
	a.windows.Store(w.ID, w)
	// A native close may race creation before the registry entry exists.
	select {
	case <-done:
		a.windows.Delete(w.ID)
	default:
	}
	deliver := func(event WindowEvent) {
		defer util.GoRecover(ctx, "plugin window callback")
		if option.OnEvent != nil {
			option.OnEvent(context.WithoutCancel(ctx), event)
		}
	}
	util.Go(context.WithoutCancel(ctx), "plugin window events", func() {
		defer func() {
			if option.Release != nil {
				option.Release()
			}
		}()
		for {
			select {
			case <-done:
				if w.valid() && option.OnEvent != nil {
					deliver(WindowEvent{Type: WindowEventClosed, WindowID: w.ID})
				}
				return
			case <-events.ready:
				event, ok := events.pop()
				if !ok {
					continue
				}
				if w.valid() && driver.AcceptEvent(event) && option.OnEvent != nil {
					deliver(event)
				}
				// The UI thread must never block on a slow plugin. Report any dropped events
				// through the callback after it becomes available so the plugin can recover.
				if overflow.Swap(false) && w.valid() && option.OnEvent != nil {
					deliver(WindowEvent{Type: WindowEventError, WindowID: w.ID, ErrMsg: "window event queue full; events were dropped"})
				}
			}
		}
	})
	if !w.valid() {
		_ = driver.Close()
		return CreateWindowResult{WindowResult: windowResult(errors.New("plugin unloaded"))}
	}
	return CreateWindowResult{WindowResult: windowResult(nil), Window: w, WindowID: w.ID}
}
func (w *PluginWindow) valid() bool {
	return !w.owner.windowSuspended.Load() && w.owner.windowEpoch.Load() == w.epoch
}

// call rejects stale capabilities before crossing into the UI thread. Driver methods serialize native state.
func (w *PluginWindow) call(fn func() error) WindowResult {
	if !w.valid() {
		return windowResult(errors.New("plugin unloaded"))
	}
	return windowResult(fn())
}
func (w *PluginWindow) Show(ctx context.Context, o WindowShowOption) WindowShowResult {
	return WindowShowResult{w.call(func() error { return w.driver.Show(o) })}
}
func (w *PluginWindow) Hide(ctx context.Context, o WindowHideOption) WindowHideResult {
	return WindowHideResult{w.call(w.driver.Hide)}
}
func (w *PluginWindow) Close(ctx context.Context, o WindowCloseOption) WindowCloseResult {
	return WindowCloseResult{windowResult(w.driver.Close())}
}
func (w *PluginWindow) SetOptions(ctx context.Context, o WindowSetOptionsOption) WindowSetOptionsResult {
	return WindowSetOptionsResult{w.call(func() error { return w.driver.SetOptions(o) })}
}
func (c *PluginWindowContent) Set(ctx context.Context, o WindowContentSetOption) WindowContentSetResult {
	return WindowContentSetResult{c.window.call(func() error { return c.window.driver.SetContent(o.Content) })}
}

// Update applies a new woxview tree without replacing the document, so scroll offsets and input state survive.
func (c *PluginWindowContent) Update(ctx context.Context, o WindowContentUpdateOption) WindowContentUpdateResult {
	return WindowContentUpdateResult{c.window.call(func() error { return c.window.driver.UpdateContent(o) })}
}

// Command accepts a local operation for execution after the next content layout.
func (c *PluginWindowContent) Command(ctx context.Context, o WindowContentCommandOption) WindowContentCommandResult {
	return WindowContentCommandResult{c.window.call(func() error { return c.window.driver.ContentCommand(o) })}
}

// PluginWindowByID only resolves windows owned by this live API instance, for the host RPC adapter.
func (a *APIImpl) PluginWindowByID(id string) (*PluginWindow, bool) {
	a.windowOnce.Do(func() { a.windows = util.NewHashMap[string, *PluginWindow]() })
	return a.windows.Load(id)
}

// closeWindows is called for unload, host loss, offline suspension, and failed initialization.
func (a *APIImpl) closeWindows() {
	a.windowSuspended.Store(true)
	a.windowEpoch.Add(1)
	a.windowOnce.Do(func() { a.windows = util.NewHashMap[string, *PluginWindow]() })
	for _, id := range a.windows.Keys() {
		if w, ok := a.windows.Load(id); ok {
			_ = w.driver.Close()
		}
	}
}
