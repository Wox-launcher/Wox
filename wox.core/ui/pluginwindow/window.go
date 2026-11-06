// Package pluginwindow owns plugin window lifetimes and content presentation.
package pluginwindow

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"runtime"
	"wox/i18n"
	"wox/plugin"
	component "wox/ui/launcher/component"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
	"wox/ui/woxview"
	"wox/util/overlay"

	"github.com/google/uuid"
)

var anchors = map[plugin.WindowAnchor]int{
	plugin.WindowAnchorTopLeft: overlay.AnchorTopLeft, plugin.WindowAnchorTopCenter: overlay.AnchorTopCenter, plugin.WindowAnchorTopRight: overlay.AnchorTopRight,
	plugin.WindowAnchorLeftCenter: overlay.AnchorLeftCenter, plugin.WindowAnchorCenter: overlay.AnchorCenter, plugin.WindowAnchorRightCenter: overlay.AnchorRightCenter,
	plugin.WindowAnchorBottomLeft: overlay.AnchorBottomLeft, plugin.WindowAnchorBottomCenter: overlay.AnchorBottomCenter, plugin.WindowAnchorBottomRight: overlay.AnchorBottomRight,
}

// session fields are confined to the UI thread, including native window callbacks.
type session struct {
	id               string
	options          plugin.WindowOptions
	placement        overlay.WindowOptions
	document         string
	sequence         uint64
	view             *woxview.View
	window           *ui.Window
	managed          *ui.ManagedWindow
	host             *widget.Host
	emit             func(plugin.WindowEvent)
	closed           bool
	workArea         func() (ui.Rect, bool)
	translate        func(string) string
	unsubscribeTheme func()
}

func boolValue(p *bool, fallback bool) bool {
	if p != nil {
		return *p
	}
	return fallback
}
func number(p *float64, fallback float64) float64 {
	if p != nil {
		return *p
	}
	return fallback
}

// prepareContent validates the WoxView document before allocating or mutating a window.
func prepareContent(c plugin.WindowContent) (*woxview.Tree, error) {
	if c.Type != plugin.WindowContentWoxView {
		return nil, errors.New("unsupported window content type")
	}
	return woxview.Parse([]byte(c.Data))
}

// validateGeometry rejects NaN/infinite dimensions and unknown anchors.
func validateGeometry(width, height float64, anchor plugin.WindowAnchor, x, y float64) error {
	for _, n := range []float64{width, height, x, y} {
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return errors.New("window geometry must be finite")
		}
	}
	if width <= 0 || height <= 0 || width > 16384 || height > 16384 {
		return errors.New("window size must be between 0 and 16384 logical units")
	}
	if _, ok := anchors[anchor]; !ok {
		return errors.New("unsupported window anchor")
	}
	return nil
}

// Create allocates a hidden registered window and prepares content without showing the launcher.
func Create(id string, o plugin.CreateWindowOption, emit func(plugin.WindowEvent)) (plugin.WindowDriver, error) {
	return CreateWithWorkArea(id, o, emit, nil, nil)
}

// normalizePresentation keeps overlays nonactivating and topmost, while
// ordinary windows participate in native application/window discovery.
func normalizePresentation(options *plugin.WindowOptions) (ui.WindowRole, error) {
	switch options.Presentation {
	case "", plugin.WindowPresentationOverlay:
		return ui.WindowRoleUtility, nil
	case plugin.WindowPresentationWindow:
		if options.Focusable == nil {
			value := true
			options.Focusable = &value
		}
		if options.Topmost == nil {
			value := false
			options.Topmost = &value
		}
		return ui.WindowRoleApplication, nil
	default:
		return 0, errors.New("unsupported window presentation")
	}
}

// CreateWithWorkArea resolves the launcher's selected display without coupling this feature to UI settings.
func CreateWithWorkArea(id string, o plugin.CreateWindowOption, emit func(plugin.WindowEvent), workArea func() (ui.Rect, bool), translate func(string) string) (plugin.WindowDriver, error) {
	if boolValue(o.Window.PassThroughTransparent, false) {
		if !boolValue(o.Window.Transparent, false) {
			return nil, errors.New("transparent input passthrough requires a transparent window")
		}
		if runtime.GOOS != "windows" {
			return nil, errors.New("transparent input passthrough is currently supported on Windows")
		}
	}
	tree, err := prepareContent(o.Content)
	if err != nil {
		return nil, err
	}
	role, err := normalizePresentation(&o.Window)
	if err != nil {
		return nil, err
	}
	if o.Window.Anchor == "" {
		o.Window.Anchor = plugin.WindowAnchorBottomCenter
	}
	width, height := number(o.Window.Width, 360), number(o.Window.Height, 160)
	x, y := number(o.Window.OffsetX, 0), number(o.Window.OffsetY, -80)
	if err := validateGeometry(width, height, o.Window.Anchor, x, y); err != nil {
		return nil, err
	}
	s := &session{id: id, options: o.Window, emit: emit, workArea: workArea, translate: translate, placement: overlay.WindowOptions{Width: width, Height: height, Anchor: anchors[o.Window.Anchor], OffsetX: x, OffsetY: y}}
	var createErr error
	dispatchErr := ui.Call(func() {
		manager := overlay.WindowManager()
		if manager == nil {
			createErr = errors.New("window manager unavailable")
			return
		}
		s.host = widget.NewHost(s.build)
		opts := ui.WindowOptions{Title: o.Window.Title, Size: ui.Size{Width: float32(width), Height: float32(height)}, Role: role,
			Nonactivating: !boolValue(o.Window.Focusable, false), Transparent: boolValue(o.Window.Transparent, false), PassThroughTransparent: boolValue(o.Window.PassThroughTransparent, false), Topmost: boolValue(o.Window.Topmost, true), Resizable: boolValue(o.Window.Resizable, false),
			OnFrame: s.host.Frame, OnPointer: s.host.Pointer, OnTextInput: func(e ui.TextInputEvent) { s.host.TextInput(e) },
			OnKey: func(e ui.KeyEvent) bool {
				if s.host.Key(e) {
					return true
				}
				if e.Down && e.Key == ui.KeyEscape && boolValue(s.options.Closable, true) {
					_ = s.managed.Close()
					return true
				}
				return false
			},
			OnFocus: func(e ui.FocusEvent) { s.host.SetWindowFocused(e.Active) },
			OnCloseRequested: func() {
				if boolValue(s.options.Closable, true) {
					_ = s.managed.Close()
				}
			},
			OnClosed: func() {
				s.closed = true
				if s.unsubscribeTheme != nil {
					s.unsubscribeTheme()
					s.unsubscribeTheme = nil
				}
				s.host.Dispose()
				s.emit(plugin.WindowEvent{Type: plugin.WindowEventClosed})
			},
		}
		s.managed, _, createErr = manager.Open(ui.WindowID("plugin."+id), opts)
		if createErr != nil {
			s.host.Dispose()
			return
		}
		s.window = s.managed.Window()
		_ = s.window.SetAppearance(!overlay.CurrentThemeChrome().Light)
		s.unsubscribeTheme = overlay.SubscribeThemeChanges(s.themeChanged)
		s.host.Attach(s.window)
		createErr = s.layout(false)
		if createErr == nil {
			createErr = s.setContent(tree)
		}
		if createErr != nil {
			_ = s.managed.Close()
		}
	})
	if dispatchErr != nil {
		return nil, dispatchErr
	}
	if createErr != nil {
		return nil, createErr
	}
	return s, nil
}

// call serializes operations with window teardown and native callbacks.
func (s *session) call(fn func() error) error {
	var err error
	if dispatchErr := ui.Call(func() {
		if s.closed {
			err = errors.New("window closed")
			return
		}
		err = fn()
	}); dispatchErr != nil {
		return dispatchErr
	}
	return err
}

// Show separates passive presentation from explicitly requested activation.
func (s *session) Show(o plugin.WindowShowOption) error {
	return s.call(func() error {
		if o.Activate && !boolValue(s.options.Focusable, false) {
			return errors.New("window is not focusable")
		}
		return s.show(o)
	})
}

func (s *session) show(o plugin.WindowShowOption) error {
	if s.workArea != nil {
		if area, ok := s.workArea(); ok {
			current, err := s.window.Bounds()
			if err == nil && !windowCenterInWorkArea(current, area) {
				if err := s.layout(false); err != nil {
					return err
				}
			}
		}
	}
	if o.Activate {
		_, err := s.managed.Show()
		return err
	}
	_, err := s.managed.ShowInactive()
	return err
}

// windowCenterInWorkArea preserves dragged positions when the selected display has not changed.
func windowCenterInWorkArea(bounds, area ui.Rect) bool {
	x, y := bounds.X+bounds.Width/2, bounds.Y+bounds.Height/2
	return x >= area.X && x < area.X+area.Width && y >= area.Y && y < area.Y+area.Height
}

func (s *session) Hide() error {
	return s.call(func() error {
		return s.managed.Hide()
	})
}

// Close serializes idempotent teardown with the native event loop.
func (s *session) Close() error {
	var err error
	dispatchErr := ui.Call(func() {
		if !s.closed {
			err = s.managed.Close()
		}
	})
	if dispatchErr != nil {
		return dispatchErr
	}
	return err
}

// layout delegates monitor selection, logical coordinates, and clamping to the shared overlay geometry.
func (s *session) layout(preserve bool) error {
	current, _ := s.window.Bounds()
	o := s.placement
	o.PreservePosition = preserve
	area := overlay.WorkArea(o, current)
	if !preserve && s.workArea != nil {
		if selected, ok := s.workArea(); ok {
			area = selected
		}
	}
	size := ui.Size{Width: min(float32(o.Width), area.Width), Height: min(float32(o.Height), area.Height)}
	return s.window.SetBounds(overlay.Bounds(o, current, area, size))
}

// SetOptions applies a patch without recreating the native window or its document.
func (s *session) SetOptions(o plugin.WindowSetOptionsOption) error {
	return s.call(func() error {
		next := s.placement
		anchor := s.options.Anchor
		if o.Width != nil {
			next.Width = *o.Width
		}
		if o.Height != nil {
			next.Height = *o.Height
		}
		if o.Anchor != "" {
			anchor = o.Anchor
			next.Anchor = anchors[anchor]
		}
		if o.OffsetX != nil {
			next.OffsetX = *o.OffsetX
		}
		if o.OffsetY != nil {
			next.OffsetY = *o.OffsetY
		}
		if err := validateGeometry(next.Width, next.Height, anchor, next.OffsetX, next.OffsetY); err != nil {
			return err
		}
		if o.Topmost != nil {
			if err := s.window.SetTopmost(*o.Topmost); err != nil {
				return err
			}
		}
		s.placement = next
		s.options.Anchor = anchor
		if err := s.layout(o.Anchor == "" && o.OffsetX == nil && o.OffsetY == nil); err != nil {
			return err
		}
		return s.window.Invalidate()
	})
}

// SetContent validates input before replacing the current document.
func (s *session) SetContent(c plugin.WindowContent) error {
	tree, err := prepareContent(c)
	if err != nil {
		return err
	}
	return s.call(func() error { return s.setContent(tree) })
}

// UpdateContent swaps the woxview tree in place. Unlike SetContent it keeps the document, so
// keyed widget state (scroll offsets, text being edited) survives and no ready event is sent.
func (s *session) UpdateContent(o plugin.WindowContentUpdateOption) error {
	tree, err := woxview.Parse([]byte(o.Data))
	if err != nil {
		return err
	}
	return s.call(func() error {
		if err := s.validateUpdate(o); err != nil {
			return err
		}
		if o.Sequence != 0 {
			s.sequence = o.Sequence
		}
		s.view.SetTree(tree)
		return s.window.Invalidate()
	})
}

// validateUpdate prevents delayed work from overwriting a newer document or ordered update.
func (s *session) validateUpdate(o plugin.WindowContentUpdateOption) error {
	if o.DocumentID != "" && o.DocumentID != s.document {
		return errors.New("document replaced")
	}
	if (o.Sequence != 0 && o.Sequence <= s.sequence) || (o.Sequence == 0 && s.sequence != 0) {
		return errors.New("stale window content update sequence")
	}
	return nil
}

// setContent replaces the document identity and drops retained state and pending callbacks.
func (s *session) setContent(tree *woxview.Tree) error {
	if s.view != nil {
		s.view.CancelPending()
	}
	s.document = uuid.NewString()
	s.sequence = 0
	s.view = woxview.NewView()
	s.view.SetTree(tree)
	s.emit(plugin.WindowEvent{Type: plugin.WindowEventReady, DocumentID: s.document})
	return s.window.Invalidate()
}

func (s *session) chromeHeight() float32 {
	if boolValue(s.options.HideChrome, false) {
		return 0
	}
	if boolValue(s.options.Closable, true) || boolValue(s.options.Movable, false) {
		return 30
	}
	return 0
}

// build renders the WoxView document with the window chrome.
func (s *session) build(frame ui.FrameInfo) widget.Widget {
	chrome := overlay.CurrentThemeChrome()
	h := s.chromeHeight()
	width, height := frame.Size.Width, max(float32(1), frame.Size.Height-h)
	// Replaced documents must never inherit retained widget state.
	document := s.document
	body := s.view.Build(woxview.BuildProps{
		Translate: s.translate, Width: width, Height: height, Theme: controlTheme(chrome), Window: s.window, IDPrefix: "plugin-window/" + document,
		OnNodeEvent: func(event woxview.Event) {
			data, _ := json.Marshal(event)
			coalesceKey := ""
			if event.Coalescible {
				// Length-prefix the handler so arbitrary node keys cannot collide.
				coalesceKey = fmt.Sprintf("%d:%s%s", len(event.Handler), event.Handler, event.Key)
			}
			s.emit(plugin.WindowEvent{Type: plugin.WindowEventWoxView, DocumentID: document, Data: data, CoalesceKey: coalesceKey})
		},
		OnOpenLink: func(url string) { _ = s.window.OpenExternalURL(url) },
	})
	body = contentDrag(s.options, body, func() { _ = s.window.StartDragging() })
	children := []widget.StackChild{{Top: h, Child: body}}
	if h > 0 {
		if boolValue(s.options.Movable, false) {
			children = append(children, widget.StackChild{Child: widget.Gesture{ID: "plugin-window-drag", OnDragStart: func() { _ = s.window.StartDragging() }, Child: widget.Container{Width: width - 30, Height: h}}})
		}
		if boolValue(s.options.Closable, true) {
			children = append(children, widget.StackChild{Right: 0, AnchorRight: true, Child: component.WoxIconButton(component.IconButtonProps{ID: "plugin-window-close", Label: i18n.GetI18nManager().TranslateWox(context.Background(), "ui_close"), Icon: component.CloseGlyph(12, chrome.Foreground), Width: 30, Height: 30, OnTap: func() { _ = s.managed.Close() }})})
		}
	}
	background := chrome.Background
	if boolValue(s.options.Transparent, false) {
		background = ui.Color{}
	}
	return widget.Container{Width: width, Height: frame.Size.Height, Color: background, Child: widget.Stack{Width: width, Height: frame.Size.Height, Children: children}}
}

// contentDrag lets interactive descendants retain their own gestures and selection.
func contentDrag(options plugin.WindowOptions, body widget.Widget, start func()) widget.Widget {
	if !boolValue(options.Movable, false) || !boolValue(options.DragContent, false) {
		return body
	}
	return widget.Gesture{ID: "plugin-window-content-drag", OnDragStart: start, Child: body}
}

// controlTheme maps the overlay chrome onto component colors. Text uses the overlay foreground
// rather than launcher colors because plugin windows paint the overlay wash, not the launcher.
func controlTheme(chrome overlay.ThemeChrome) component.ControlTheme {
	colors := overlay.CurrentThemeControlColors(chrome)
	return component.ControlTheme{
		Dark: !chrome.Light, Hover: colors.Hover, Disabled: colors.Disabled, Link: colors.Link, Overlay: chrome.Background,
		Text: chrome.Foreground, BodyText: chrome.Foreground, ControlText: chrome.Foreground, InputText: chrome.Foreground, ChromeText: chrome.Foreground,
		TextSecondary: colors.Secondary, Background: chrome.Background, Surface: chrome.Background, Border: colors.Border, Focus: colors.Focus,
		Accent: colors.Accent, AccentText: colors.AccentText, SelectionBackground: colors.Selection, SelectionText: colors.SelectedText,
		TextSelectionBackground: colors.Selection, TextSelectionText: chrome.Foreground,
		Success: colors.Success, Warning: colors.Warning, WarningText: colors.WarningText, Error: colors.Error,
	}
}

// AcceptEvent drops already-queued events after a document replacement or close.
func (s *session) AcceptEvent(e plugin.WindowEvent) bool {
	accepted := false
	_ = s.call(func() error {
		accepted = e.DocumentID == s.document
		if accepted && e.Type == plugin.WindowEventWoxView {
			var payload struct{ Handler woxview.HandlerID }
			accepted = s.view != nil && json.Unmarshal(e.Data, &payload) == nil && s.view.HasHandler(payload.Handler)
		}
		return nil
	})
	return accepted
}
