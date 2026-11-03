// Package pluginwindow owns plugin window lifetimes and content presentation.
package pluginwindow

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"time"
	"wox/i18n"
	"wox/internal/assetfs"
	"wox/plugin"
	component "wox/ui/launcher/component"
	preview "wox/ui/launcher/view/preview"
	"wox/ui/previewcontent"
	ui "wox/ui/runtime"
	widget "wox/ui/widget"
	"wox/util/overlay"

	"github.com/google/uuid"
)

const maxMessageBytes = 256 * 1024

var anchors = map[plugin.WindowAnchor]int{
	plugin.WindowAnchorTopLeft: overlay.AnchorTopLeft, plugin.WindowAnchorTopCenter: overlay.AnchorTopCenter, plugin.WindowAnchorTopRight: overlay.AnchorTopRight,
	plugin.WindowAnchorLeftCenter: overlay.AnchorLeftCenter, plugin.WindowAnchorCenter: overlay.AnchorCenter, plugin.WindowAnchorRightCenter: overlay.AnchorRightCenter,
	plugin.WindowAnchorBottomLeft: overlay.AnchorBottomLeft, plugin.WindowAnchorBottomCenter: overlay.AnchorBottomCenter, plugin.WindowAnchorBottomRight: overlay.AnchorBottomRight,
}

// session fields are confined to the UI thread, including native message callbacks.
type session struct {
	id            string
	options       plugin.WindowOptions
	placement     overlay.WindowOptions
	content       plugin.WindowContent
	document      string
	html          string
	markdown      component.MarkdownDocument
	window        *ui.Window
	managed       *ui.ManagedWindow
	host          *widget.Host
	emit          func(plugin.WindowEvent)
	ready, closed bool
	contentDrag   contentDrag
	pending       []json.RawMessage
	loadTimer     *time.Timer
	loadFailed    bool
	pendingShow   *plugin.WindowShowOption
	workArea      func() (ui.Rect, bool)
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

// validateContent bounds allocations before content enters a native engine.
func validateContent(c plugin.WindowContent) error {
	if len(c.Data) > 4*1024*1024 {
		return errors.New("window content exceeds 4 MiB")
	}
	switch c.Type {
	case plugin.WindowContentText, plugin.WindowContentMarkdown, plugin.WindowContentHTML:
		return nil
	}
	return errors.New("unsupported window content type")
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
	return CreateWithWorkArea(id, o, emit, nil)
}

// CreateWithWorkArea resolves the launcher's selected display without coupling this feature to UI settings.
func CreateWithWorkArea(id string, o plugin.CreateWindowOption, emit func(plugin.WindowEvent), workArea func() (ui.Rect, bool)) (plugin.WindowDriver, error) {
	if err := validateContent(o.Content); err != nil {
		return nil, err
	}
	if o.Window.Presentation != "" && o.Window.Presentation != plugin.WindowPresentationOverlay {
		return nil, errors.New("unsupported window presentation")
	}
	if o.Window.Anchor == "" {
		o.Window.Anchor = plugin.WindowAnchorBottomCenter
	}
	width, height := number(o.Window.Width, 360), number(o.Window.Height, 160)
	x, y := number(o.Window.OffsetX, 0), number(o.Window.OffsetY, -80)
	if err := validateGeometry(width, height, o.Window.Anchor, x, y); err != nil {
		return nil, err
	}
	s := &session{id: id, options: o.Window, emit: emit, workArea: workArea, placement: overlay.WindowOptions{Width: width, Height: height, Anchor: anchors[o.Window.Anchor], OffsetX: x, OffsetY: y}}
	var createErr error
	dispatchErr := ui.Call(func() {
		manager := overlay.WindowManager()
		if manager == nil {
			createErr = errors.New("window manager unavailable")
			return
		}
		s.host = widget.NewHost(s.build)
		opts := ui.WindowOptions{Title: o.Window.Title, Size: ui.Size{Width: float32(width), Height: float32(height)}, Role: ui.WindowRoleUtility,
			Nonactivating: !boolValue(o.Window.Focusable, false), Topmost: boolValue(o.Window.Topmost, true), Resizable: boolValue(o.Window.Resizable, false),
			OnFrame: s.host.Frame, OnPointer: s.host.Pointer, OnTextInput: func(e ui.TextInputEvent) { s.host.TextInput(e) },
			OnKey: func(e ui.KeyEvent) bool {
				if e.Down && e.Key == ui.KeyEscape && boolValue(s.options.Closable, true) {
					_ = s.managed.Close()
					return true
				}
				return s.host.Key(e)
			},
			OnFocus: func(e ui.FocusEvent) { s.host.SetWindowFocused(e.Active) }, OnWebViewMessage: s.message,
			OnCloseRequested: func() {
				if boolValue(s.options.Closable, true) {
					_ = s.managed.Close()
				}
			},
			OnClosed: func() {
				s.closed = true
				if s.loadTimer != nil {
					s.loadTimer.Stop()
				}
				s.pending = nil
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
		s.host.Attach(s.window)
		createErr = s.layout(false)
		if createErr == nil {
			createErr = s.setContent(o.Content)
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
		if s.content.Type == plugin.WindowContentHTML && !s.ready {
			if s.loadFailed {
				return errors.New("HTML document failed to become ready")
			}
			s.pendingShow = &o
			return nil
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
		s.pendingShow = nil
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
	if err := validateContent(c); err != nil {
		return err
	}
	return s.call(func() error { return s.setContent(c) })
}

// setContent replaces the document identity before resetting native content, invalidating late messages.
func (s *session) setContent(c plugin.WindowContent) error {
	s.contentDrag = contentDrag{}
	if s.loadTimer != nil {
		s.loadTimer.Stop()
	}
	s.document = uuid.NewString()
	s.ready = false
	s.loadFailed = false
	s.pending = nil
	s.content = c
	s.html = ""
	if err := s.window.ResetWebView(); err != nil {
		return err
	}
	switch c.Type {
	case plugin.WindowContentHTML:
		s.html = bridgeHTML(c.Data, s.document)
		document := s.document
		s.loadTimer = time.AfterFunc(15*time.Second, func() {
			_ = ui.Post(func() {
				if !s.closed && !s.ready && s.document == document {
					s.loadFailed = true
					s.pendingShow = nil
					s.pending = nil
					s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: document, ErrMsg: "HTML document did not become ready within 15 seconds"})
				}
			})
		})
		bounds, _ := s.window.Bounds()
		if err := s.showHTML(ui.Rect{Width: bounds.Width, Height: max(float32(1), bounds.Height-s.chromeHeight())}); err != nil {
			return err
		}
	case plugin.WindowContentMarkdown:
		s.markdown = component.ParseMarkdown(c.Data)
		s.ready = true
	default:
		s.ready = true
	}
	if s.ready {
		s.emit(plugin.WindowEvent{Type: plugin.WindowEventReady, DocumentID: s.document})
		s.showPending()
	}
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

func (s *session) showHTML(bounds ui.Rect) error {
	return s.window.ShowWebView(ui.WebViewContent{DocumentKey: s.document, HTML: s.html}, bounds)
}

// contentDrag delays primary presses until they resolve as clicks or native window drags.
type contentDrag struct{ press *ui.PointerEvent }

func (d *contentDrag) pointer(event ui.PointerEvent, forward func(ui.PointerEvent) bool, drag func()) bool {
	switch event.Kind {
	case ui.PointerDown:
		if event.Button == ui.PointerButtonPrimary {
			press := event
			d.press = &press
			return true
		}
	case ui.PointerMove:
		if d.press != nil {
			dx, dy := event.Position.X-d.press.Position.X, event.Position.Y-d.press.Position.Y
			if dx*dx+dy*dy >= 9 {
				d.press = nil
				drag()
				return true
			}
		}
	case ui.PointerUp:
		if event.Button == ui.PointerButtonPrimary && d.press != nil {
			press := *d.press
			d.press = nil
			forward(press)
			return forward(event)
		}
	}
	return forward(event)
}

func (s *session) contentPointer(event ui.PointerEvent) bool {
	return s.contentDrag.pointer(event, s.window.ForwardEmbeddedSurfacePointer, func() { _ = s.window.StartDragging() })
}

// build reuses native Preview text/Markdown renderers and the shared WebView surface.
func (s *session) build(frame ui.FrameInfo) widget.Widget {
	chrome := overlay.CurrentThemeChrome()
	h := s.chromeHeight()
	width, height := frame.Size.Width, max(float32(1), frame.Size.Height-h)
	controls := component.ControlTheme{Text: chrome.Foreground, BodyText: chrome.Foreground, ControlText: chrome.Foreground, InputText: chrome.Foreground, TextSecondary: chrome.Foreground, Background: chrome.Background, Surface: chrome.Background}
	theme := component.Theme{Controls: controls, PreviewText: chrome.Foreground, QueryBackground: chrome.Background}
	var body widget.Widget
	switch s.content.Type {
	case plugin.WindowContentHTML:
		// The enclosing container already paints the theme wash; a second layer
		// would darken a transparent WebView and hide the native material.
		props := previewcontent.WebViewSurfaceProps{Key: "plugin-window-webview", Width: width, Height: height, OnPointer: s.window.ForwardEmbeddedSurfacePointer, OnBounds: func(bounds ui.Rect) {
			if err := s.showHTML(bounds); err != nil {
				s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: s.document, ErrMsg: err.Error()})
			}
		}}
		if boolValue(s.options.Movable, false) && boolValue(s.options.DragContent, false) {
			props.OnPointer = s.contentPointer
		}
		body = previewcontent.WebViewSurface(props)
	case plugin.WindowContentMarkdown:
		body = preview.MarkdownPreviewView(preview.MarkdownPreviewProps{ID: s.document, Document: s.markdown, Width: width, Height: height, Window: s.window, Theme: theme, OnOpenLink: func(url string) { _ = s.window.OpenExternalURL(url) }})
	default:
		body = preview.ScrollablePreviewText(preview.ScrollablePreviewTextProps{ID: s.document, Value: s.content.Data, Width: width, Height: height, FontSize: 14, LineHeight: 21, Color: chrome.Foreground, Window: s.window, Theme: theme})
	}
	children := []widget.StackChild{{Top: h, Child: body}}
	if h > 0 {
		if boolValue(s.options.Movable, false) {
			children = append(children, widget.StackChild{Child: widget.Gesture{ID: "plugin-window-drag", OnDragStart: func() { _ = s.window.StartDragging() }, Child: widget.Container{Width: width - 30, Height: h}}})
		}
		if boolValue(s.options.Closable, true) {
			children = append(children, widget.StackChild{Right: 0, AnchorRight: true, Child: component.WoxIconButton(component.IconButtonProps{ID: "plugin-window-close", Label: i18n.GetI18nManager().TranslateWox(context.Background(), "ui_close"), Icon: component.CloseGlyph(12, chrome.Foreground), Width: 30, Height: 30, OnTap: func() { _ = s.managed.Close() }})})
		}
	}
	return widget.Container{Width: width, Height: frame.Size.Height, Color: chrome.Background, Child: widget.Stack{Width: width, Height: frame.Size.Height, Children: children}}
}

// PostMessage sends JSON only. Hidden windows keep their document; replacement drops pending data.
func (s *session) PostMessage(o plugin.WindowMessageOption) error {
	data := o.Data
	if len(data) > maxMessageBytes || !json.Valid(data) {
		return errors.New("message must be valid JSON of at most 256 KiB")
	}
	copyData := append(json.RawMessage(nil), data...)
	return s.call(func() error {
		if o.DocumentID != "" && o.DocumentID != s.document {
			return errors.New("document replaced")
		}
		if s.loadFailed {
			return errors.New("HTML document failed to become ready")
		}
		if s.content.Type != plugin.WindowContentHTML {
			return errors.New("messages require HTML content")
		}
		if !s.ready {
			if len(s.pending) >= 64 {
				return errors.New("window message queue full")
			}
			s.pending = append(s.pending, copyData)
			return nil
		}
		return s.send(copyData)
	})
}

// send targets the current inline document through the shared WebView queue.
func (s *session) send(data json.RawMessage) error {
	token, _ := json.Marshal(s.document)
	// Parse JSON as data: object literals would give __proto__ special semantics.
	payload, _ := json.Marshal(string(data))
	script := "window.__woxWindowReceive(" + string(token) + ",JSON.parse(" + string(payload) + "))"
	if !s.window.UpdateWebViewScript(s.document, script) {
		return errors.New("document unavailable")
	}
	return nil
}

// message authenticates each document generation before delivering native callbacks.
func (s *session) message(raw string) {
	if s.closed || len(raw) > maxMessageBytes+256 {
		return
	}
	var m struct {
		Token string
		Type  string
		Data  json.RawMessage
	}
	if json.Unmarshal([]byte(raw), &m) != nil || m.Token != s.document || s.content.Type != plugin.WindowContentHTML {
		return
	}
	switch m.Type {
	case "ready":
		if s.ready {
			return
		}
		s.ready = true
		s.loadFailed = false
		if s.loadTimer != nil {
			s.loadTimer.Stop()
		}
		s.emit(plugin.WindowEvent{Type: plugin.WindowEventReady, DocumentID: s.document})
		pending := s.pending
		s.pending = nil
		for _, data := range pending {
			if err := s.send(data); err != nil {
				s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: s.document, ErrMsg: err.Error()})
			}
		}
		s.showPending()
	case "message":
		if json.Valid(m.Data) {
			s.emit(plugin.WindowEvent{Type: plugin.WindowEventMessage, DocumentID: s.document, Data: m.Data})
		}
	case "error":
		var detail string
		if json.Unmarshal(m.Data, &detail) == nil {
			s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: s.document, ErrMsg: detail})
		}
	case "link":
		var url string
		if json.Unmarshal(m.Data, &url) == nil {
			_ = s.window.OpenExternalURL(url)
		}
	}
}

// showPending presents the latest requested document only after its ready event.
func (s *session) showPending() {
	if s.pendingShow == nil {
		return
	}
	o := *s.pendingShow
	s.pendingShow = nil
	if err := s.show(o); err != nil {
		s.emit(plugin.WindowEvent{Type: plugin.WindowEventError, DocumentID: s.document, ErrMsg: err.Error()})
	}
}

//go:embed bridge.js
var rawBridgeFS embed.FS

var bridgeSource = string(assetfs.New(rawBridgeFS).MustReadFile("bridge.js"))

// bridgeHTML installs the bridge before plugin scripts, preserving standards mode.
func bridgeHTML(html, token string) string {
	encoded, _ := json.Marshal(token)
	script := "<script>" + strings.Replace(bridgeSource, "__WOX_DOCUMENT_TOKEN__", string(encoded), 1) + "</script>"
	end := 0
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(html)), "<!doctype") {
		end = strings.Index(html, ">") + 1
	}
	return html[:end] + script + html[end:]
}

// AcceptEvent drops already-queued messages after a document replacement or close.
func (s *session) AcceptEvent(e plugin.WindowEvent) bool {
	accepted := false
	_ = s.call(func() error { accepted = e.DocumentID == s.document; return nil })
	return accepted
}
