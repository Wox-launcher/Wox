package overlay

import (
	"math"
	"runtime"
	"sync"
	"time"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/screen"
	"wox/util/window"
)

const (
	AnchorTopLeft      = 0
	AnchorTopCenter    = 1
	AnchorTopRight     = 2
	AnchorLeftCenter   = 3
	AnchorCenter       = 4
	AnchorRightCenter  = 5
	AnchorBottomLeft   = 6
	AnchorBottomCenter = 7
	AnchorBottomRight  = 8
	AnchorBelowCenter  = 9
)

// WindowOptions defines the window-level behavior shared by runtime overlays.
type WindowOptions struct {
	ID            string
	CloseOnEscape bool
	TakeFocus     bool
	// Topmost keeps the overlay above the launcher. Preview windows need this
	// because they take focus and would otherwise share Wox's floating level.
	Topmost          bool
	AbsolutePosition bool
	PreservePosition bool
	StickyWindowPid  int
	StickyWindowId   string
	Anchor           int
	OffsetX          float64
	OffsetY          float64
	// WorkArea overrides display discovery when a platform tracker already resolved
	// the target window and visible frame in one coordinate space.
	WorkArea  *woxui.Rect
	Movable   bool
	Resizable bool
	// LightAppearance requests the light window appearance instead of the
	// default dark one, letting themed overlays match the active theme.
	LightAppearance bool
	// FollowsThemeAppearance updates native window materials when the Wox
	// theme switches between light and dark, instead of freezing the
	// creation-time appearance.
	FollowsThemeAppearance bool
	CornerRadius           float64
	AspectRatio            float64
	Width                  float64
	MinWidth               float64
	MaxWidth               float64
	Height                 float64
	MaxHeight              float64
	OnClose                func()
}

// View supplies one overlay's portable measurement, widget tree, and interaction state.
type View struct {
	Kind      string
	Measure   func(window *woxui.Window, workArea woxui.Rect) woxui.Size
	Build     func(window *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget
	OnPointer func(event woxui.PointerEvent)
	OnKey     func(event woxui.KeyEvent) bool
	OnFocus   func(event woxui.FocusEvent)
	OnDispose func()
}

type runtimeOverlay struct {
	id      string
	options WindowOptions
	view    View
	managed *woxui.ManagedWindow
	window  *woxui.Window
	host    *woxwidget.Host

	stickyStop   chan struct{}
	stickyDetach func()
}

var runtimeOverlays = struct {
	sync.Mutex
	manager *woxui.WindowManager
	byID    map[string]*runtimeOverlay
}{byID: map[string]*runtimeOverlay{}}

// SetWindowManager attaches overlays to the process-local Go UI window registry.
func SetWindowManager(manager *woxui.WindowManager) {
	runtimeOverlays.Lock()
	runtimeOverlays.manager = manager
	runtimeOverlays.Unlock()
}

// ShowWindow creates or updates one runtime-rendered overlay.
func ShowWindow(options WindowOptions, view View) bool {
	if options.ID == "" || view.Build == nil {
		return false
	}
	shown := false
	if err := woxui.Call(func() { shown = showWindowOnUI(options, view) }); err != nil {
		return false
	}
	if shown {
		RegisterCloseCallback(options.ID, options.OnClose)
	}
	return shown
}

func showWindowOnUI(options WindowOptions, view View) bool {
	runtimeOverlays.Lock()
	manager := runtimeOverlays.manager
	instance := runtimeOverlays.byID[options.ID]
	runtimeOverlays.Unlock()
	if manager == nil {
		return false
	}
	if instance != nil && instance.view.Kind != view.Kind && windowCreationOptionsChanged(instance.options, options) {
		_ = instance.managed.Close()
		instance = nil
	}

	created := instance == nil
	if created {
		instance = &runtimeOverlay{id: options.ID}
		instance.host = woxwidget.NewHost(func(frame woxui.FrameInfo) woxwidget.Widget {
			content := instance.view.Build(instance.window, frame)
			if !instance.options.Movable {
				return content
			}
			return woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: []woxwidget.StackChild{
				{Child: woxwidget.Gesture{ID: "overlay-drag", OnDragStart: func() { _ = instance.window.StartDragging() }, Child: woxwidget.Container{Width: frame.Size.Width, Height: frame.Size.Height}}},
				{Child: content},
			}}
		})
		initialWidth := float32(options.Width)
		if initialWidth <= 0 {
			initialWidth = 100
		}
		initialHeight := float32(options.Height)
		if initialHeight <= 0 {
			initialHeight = 80
		}
		nativeOptions := overlayNativeWindowOptions(options, woxui.Size{Width: initialWidth, Height: initialHeight})
		nativeOptions.OnStickyWindowChanged = func(uintptr) {
			if instance.options.StickyWindowPid > 0 {
				instance.applyLayout(false)
			}
		}
		nativeOptions.OnFrame = instance.host.Frame
		nativeOptions.OnPointer = func(event woxui.PointerEvent) {
			if instance.view.OnPointer != nil {
				instance.view.OnPointer(event)
			}
			instance.host.Pointer(event)
		}
		nativeOptions.OnKey = func(event woxui.KeyEvent) bool {
			if event.Down && event.Key == woxui.KeyEscape && instance.options.CloseOnEscape {
				RequestClose(instance.id)
				return true
			}
			if instance.view.OnKey != nil && instance.view.OnKey(event) {
				return true
			}
			return instance.host.Key(event)
		}
		nativeOptions.OnTextInput = func(event woxui.TextInputEvent) { instance.host.TextInput(event) }
		nativeOptions.OnFocus = func(event woxui.FocusEvent) {
			instance.host.SetWindowFocused(event.Active)
			if instance.view.OnFocus != nil {
				instance.view.OnFocus(event)
			}
		}
		nativeOptions.OnCloseRequested = func() { RequestClose(instance.id) }
		nativeOptions.OnClosed = instance.dispose
		managed, _, err := manager.Open(woxui.WindowID("overlay."+options.ID), nativeOptions)
		if err != nil {
			instance.host.Dispose()
			return false
		}
		instance.managed = managed
		instance.window = managed.Window()
		instance.host.Attach(instance.window)
		_ = instance.window.SetAppearance(!options.LightAppearance)
		runtimeOverlays.Lock()
		runtimeOverlays.byID[instance.id] = instance
		runtimeOverlays.Unlock()
	}

	if !created && instance.view.Kind != view.Kind && instance.view.OnDispose != nil {
		instance.view.OnDispose()
	}
	stickyChanged := instance.options.StickyWindowPid != options.StickyWindowPid || instance.options.StickyWindowId != options.StickyWindowId
	appearanceChanged := !created && instance.options.LightAppearance != options.LightAppearance
	instance.options = options
	instance.view = view
	if appearanceChanged {
		_ = instance.window.SetAppearance(!options.LightAppearance)
	}
	instance.applyLayout(false)
	if created {
		if _, err := instance.managed.Show(); err != nil {
			_ = instance.managed.Close()
			return false
		}
	}
	if stickyChanged || created {
		instance.restartStickyTracking()
	}
	return true
}

// Relayout remeasures an existing overlay while preserving its current origin.
func Relayout(id string) {
	_ = woxui.Call(func() {
		if instance := runtimeOverlayByID(id); instance != nil {
			instance.applyLayout(true)
		}
	})
}

// ScaleWindow resizes an overlay around its center while preserving its configured aspect ratio.
func ScaleWindow(id string, factor, minWidth, minHeight float32) {
	if factor <= 0 {
		return
	}
	_ = woxui.Call(func() {
		instance := runtimeOverlayByID(id)
		if instance == nil {
			return
		}
		current, err := instance.window.Bounds()
		if err != nil {
			return
		}
		target := scaledBounds(current, WorkArea(instance.options, current), factor, float32(instance.options.AspectRatio), woxui.Size{Width: minWidth, Height: minHeight})
		instance.options.Width = float64(target.Width)
		instance.options.Height = float64(target.Height)
		_ = instance.window.SetBounds(target)
		_ = instance.window.Invalidate()
	})
}

func scaledBounds(current, workArea woxui.Rect, factor, ratio float32, minimum woxui.Size) woxui.Rect {
	width := max(minimum.Width, current.Width*factor)
	height := max(minimum.Height, current.Height*factor)
	if ratio > 0 {
		height = width / ratio
		if height < minimum.Height {
			height = minimum.Height
			width = height * ratio
		}
	}
	if width > workArea.Width {
		width = workArea.Width
		if ratio > 0 {
			height = width / ratio
		}
	}
	if height > workArea.Height {
		height = workArea.Height
		if ratio > 0 {
			width = height * ratio
		}
	}
	target := woxui.Rect{X: current.X + (current.Width-width)/2, Y: current.Y + (current.Height-height)/2, Width: width, Height: height}
	return clampBounds(target, workArea)
}

// Invalidate requests a new frame for an existing overlay.
func Invalidate(id string) {
	_ = woxui.Call(func() {
		if instance := runtimeOverlayByID(id); instance != nil {
			_ = instance.window.Invalidate()
		}
	})
}

// NotifyThemeChanged updates native appearance and requests a new frame for
// every open overlay so themed chrome (for example image overlay title bars)
// matches the active light or dark theme. Invalidate alone cannot retint the
// platform title-bar material, which is owned by SetAppearance.
func NotifyThemeChanged(isDark bool) {
	_ = woxui.Call(func() {
		runtimeOverlays.Lock()
		instances := make([]*runtimeOverlay, 0, len(runtimeOverlays.byID))
		for _, instance := range runtimeOverlays.byID {
			instances = append(instances, instance)
		}
		runtimeOverlays.Unlock()
		for _, instance := range instances {
			if syncOverlayAppearance(&instance.options, isDark) {
				_ = instance.window.SetAppearance(isDark)
			}
			_ = instance.window.Invalidate()
		}
	})
}

// syncOverlayAppearance writes the live light/dark native appearance onto
// overlays that opted into theme following. It returns whether SetAppearance
// must run; unthemed overlays keep the appearance chosen at creation.
func syncOverlayAppearance(options *WindowOptions, isDark bool) bool {
	if options == nil || !options.FollowsThemeAppearance {
		return false
	}
	options.LightAppearance = !isDark
	return true
}

// Close removes one overlay without firing its user-close callback.
func Close(id string) {
	RegisterCloseCallback(id, nil)
	_ = woxui.Call(func() {
		if instance := runtimeOverlayByID(id); instance != nil {
			_ = instance.managed.Close()
		}
	})
}

func runtimeOverlayByID(id string) *runtimeOverlay {
	runtimeOverlays.Lock()
	instance := runtimeOverlays.byID[id]
	runtimeOverlays.Unlock()
	return instance
}

func (instance *runtimeOverlay) dispose() {
	instance.stopStickyTracking()
	if instance.view.OnDispose != nil {
		instance.view.OnDispose()
	}
	instance.host.Dispose()
	RegisterCloseCallback(instance.id, nil)
	runtimeOverlays.Lock()
	if runtimeOverlays.byID[instance.id] == instance {
		delete(runtimeOverlays.byID, instance.id)
	}
	runtimeOverlays.Unlock()
}

// overlayNativeWindowOptions maps overlay chrome onto a utility window. Topmost
// preview surfaces take focus but still float above the launcher. Tooltips stay
// nonactivating so they do not steal focus. Windows and macOS still take their
// material from the native window; Linux paints PanelFill because it has no
// acrylic or vibrancy.
func overlayNativeWindowOptions(options WindowOptions, size woxui.Size) woxui.WindowOptions {
	return woxui.WindowOptions{
		Title:            "Wox Overlay",
		Size:             size,
		Role:             woxui.WindowRoleUtility,
		Resizable:        options.Resizable,
		AspectRatio:      float32(options.AspectRatio),
		Nonactivating:    !(options.TakeFocus || options.CloseOnEscape),
		TransientOverlay: true,
		Topmost:          options.Topmost,
	}
}

// PanelFill is the painted overlay surface. Windows and macOS leave this empty
// so native acrylic or vibrancy shows through. Linux has neither material, so
// the panel is an opaque fill matching the requested window appearance.
func PanelFill(goos string, lightAppearance bool) woxui.Color {
	if goos != "linux" {
		return woxui.Color{}
	}
	if lightAppearance {
		return woxui.Color{R: 245, G: 245, B: 247, A: 255}
	}
	return woxui.Color{R: 24, G: 24, B: 26, A: 255}
}

// HUDSurface is the rounded overlay panel used by compact HUD windows. Linux
// paints PanelFill; other platforms keep the native window material.
func HUDSurface(width, height, radius float32, lightAppearance bool, child woxwidget.Widget) woxwidget.Container {
	return woxwidget.Container{
		Width: width, Height: height, Radius: radius,
		Color: PanelFill(runtime.GOOS, lightAppearance),
		Child: child,
	}
}

// windowCreationOptionsChanged identifies native flags that cannot be updated in place.
func windowCreationOptionsChanged(current, next WindowOptions) bool {
	return current.Resizable != next.Resizable ||
		current.AspectRatio != next.AspectRatio ||
		(current.TakeFocus || current.CloseOnEscape) != (next.TakeFocus || next.CloseOnEscape)
}

// applyLayout resolves content size and clamps the native window to its logical work area.
func (instance *runtimeOverlay) applyLayout(preserveOrigin bool) {
	current, _ := instance.window.Bounds()
	workArea := WorkArea(instance.options, current)
	size := woxui.Size{Width: float32(instance.options.Width), Height: float32(instance.options.Height)}
	if instance.view.Measure != nil {
		size = instance.view.Measure(instance.window, workArea)
	}
	if instance.options.Width > 0 {
		size.Width = float32(instance.options.Width)
	}
	if instance.options.Height > 0 {
		size.Height = float32(instance.options.Height)
	}
	if size.Width <= 0 {
		size.Width = max(float32(1), current.Width)
	}
	if size.Height <= 0 {
		size.Height = max(float32(1), current.Height)
	}
	if instance.options.MinWidth > 0 {
		size.Width = max(size.Width, float32(instance.options.MinWidth))
	}
	if instance.options.MaxWidth > 0 {
		size.Width = min(size.Width, float32(instance.options.MaxWidth))
	}
	if instance.options.MaxHeight > 0 {
		size.Height = min(size.Height, float32(instance.options.MaxHeight))
	}
	size.Width = min(size.Width, workArea.Width)
	size.Height = min(size.Height, workArea.Height)
	options := instance.options
	options.PreservePosition = options.PreservePosition || preserveOrigin
	target := Bounds(options, current, workArea, size)
	if !sameBounds(current, target) {
		_ = instance.window.SetBounds(target)
	}
	_ = instance.window.Invalidate()
}

func (instance *runtimeOverlay) restartStickyTracking() {
	instance.stopStickyTracking()
	if instance.options.StickyWindowPid <= 0 {
		return
	}
	if instance.startNativeStickyTracking() {
		return
	}
	stop := make(chan struct{})
	instance.stickyStop = stop
	// Polling is the cross-platform fallback when the native Windows hook is unavailable.
	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				_ = woxui.Call(func() {
					if runtimeOverlayByID(instance.id) == instance {
						instance.applyLayout(false)
					}
				})
			case <-stop:
				return
			}
		}
	}()
}

// stopStickyTracking detaches the platform hook or stops its polling fallback.
func (instance *runtimeOverlay) stopStickyTracking() {
	if instance.stickyStop != nil {
		close(instance.stickyStop)
		instance.stickyStop = nil
	}
	if instance.stickyDetach != nil {
		instance.stickyDetach()
		instance.stickyDetach = nil
	}
}

// WorkArea resolves the logical display work area used for sizing and clamping.
func WorkArea(options WindowOptions, current woxui.Rect) woxui.Rect {
	if options.WorkArea != nil && options.WorkArea.Width > 0 && options.WorkArea.Height > 0 {
		return *options.WorkArea
	}
	displays, err := screen.ListDisplays()
	if err != nil || len(displays) == 0 {
		active := screen.GetActiveScreen()
		return woxui.Rect{X: float32(active.X), Y: float32(active.Y), Width: float32(active.Width), Height: float32(active.Height)}
	}
	pointX, pointY := float32(options.OffsetX), float32(options.OffsetY)
	if options.PreservePosition && current.Width > 0 {
		pointX, pointY = current.X, current.Y
	}
	if sticky, ok := stickyBounds(options, displays); ok {
		pointX, pointY = sticky.X+sticky.Width/2, sticky.Y+sticky.Height/2
	}
	if !options.AbsolutePosition && options.StickyWindowPid <= 0 && !(options.PreservePosition && current.Width > 0) {
		for _, display := range displays {
			if display.Primary {
				return screenRect(display.WorkArea)
			}
		}
		return screenRect(displays[0].WorkArea)
	}
	for _, display := range displays {
		work := screenRect(display.WorkArea)
		if pointX >= work.X && pointX < work.X+work.Width && pointY >= work.Y && pointY < work.Y+work.Height {
			return work
		}
	}
	return screenRect(displays[0].WorkArea)
}

// Bounds positions an overlay in logical virtual-desktop coordinates.
func Bounds(options WindowOptions, current, workArea woxui.Rect, size woxui.Size) woxui.Rect {
	if options.PreservePosition && current.Width > 0 {
		return clampBounds(woxui.Rect{X: current.X, Y: current.Y, Width: size.Width, Height: size.Height}, workArea)
	}
	target := workArea
	if sticky, ok := stickyBounds(options, nil); ok {
		target = sticky
	} else if options.AbsolutePosition {
		target = woxui.Rect{}
	}
	return boundsForTarget(options, target, workArea, size)
}

// boundsForTarget aligns one overlay against its resolved window or display rectangle.
func boundsForTarget(options WindowOptions, target, workArea woxui.Rect, size woxui.Size) woxui.Rect {
	if options.Anchor == AnchorBelowCenter {
		return clampBounds(woxui.Rect{
			X:     target.X + target.Width/2 - size.Width/2 + float32(options.OffsetX),
			Y:     target.Y + target.Height + float32(options.OffsetY),
			Width: size.Width, Height: size.Height,
		}, workArea)
	}
	column := options.Anchor % 3
	row := options.Anchor / 3
	x, y := target.X, target.Y
	if column == 1 {
		x += target.Width/2 - size.Width/2
	} else if column == 2 {
		x += target.Width - size.Width
	}
	if row == 1 {
		y += target.Height/2 - size.Height/2
	} else if row == 2 {
		y += target.Height - size.Height
	}
	x += float32(options.OffsetX)
	y += float32(options.OffsetY)
	return clampBounds(woxui.Rect{X: x, Y: y, Width: size.Width, Height: size.Height}, workArea)
}

// stickyBounds converts a tracked native window into runtime logical coordinates.
func stickyBounds(options WindowOptions, displays []screen.Display) (woxui.Rect, bool) {
	if options.StickyWindowPid <= 0 {
		return woxui.Rect{}, false
	}
	target, err := window.GetManagedWindow(options.StickyWindowId, options.StickyWindowPid, "")
	if err != nil || target.Bounds.Width <= 0 || target.Bounds.Height <= 0 {
		return woxui.Rect{}, false
	}
	bounds := woxui.Rect{X: float32(target.Bounds.X), Y: float32(target.Bounds.Y), Width: float32(target.Bounds.Width), Height: float32(target.Bounds.Height)}
	if runtime.GOOS != "windows" {
		return bounds, true
	}
	if displays == nil {
		displays, _ = screen.ListDisplays()
	}
	centerX := target.Bounds.X + target.Bounds.Width/2
	centerY := target.Bounds.Y + target.Bounds.Height/2
	for _, display := range displays {
		pixel := display.PixelBounds
		if centerX >= pixel.X && centerX < pixel.Right() && centerY >= pixel.Y && centerY < pixel.Bottom() && display.Scale > 0 {
			scale := float32(display.Scale)
			return woxui.Rect{X: bounds.X / scale, Y: bounds.Y / scale, Width: bounds.Width / scale, Height: bounds.Height / scale}, true
		}
	}
	return bounds, true
}

func screenRect(rect screen.Rect) woxui.Rect {
	return woxui.Rect{X: float32(rect.X), Y: float32(rect.Y), Width: float32(rect.Width), Height: float32(rect.Height)}
}

func clampBounds(bounds, workArea woxui.Rect) woxui.Rect {
	bounds.Width = min(bounds.Width, workArea.Width)
	bounds.Height = min(bounds.Height, workArea.Height)
	bounds.X = min(max(bounds.X, workArea.X), workArea.X+workArea.Width-bounds.Width)
	bounds.Y = min(max(bounds.Y, workArea.Y), workArea.Y+workArea.Height-bounds.Height)
	return bounds
}

func sameBounds(left, right woxui.Rect) bool {
	return math.Abs(float64(left.X-right.X)) < 0.5 && math.Abs(float64(left.Y-right.Y)) < 0.5 && math.Abs(float64(left.Width-right.Width)) < 0.5 && math.Abs(float64(left.Height-right.Height)) < 0.5
}
