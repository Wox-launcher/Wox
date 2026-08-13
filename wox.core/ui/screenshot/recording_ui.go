package screenshot

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/util/keyboard"
)

const (
	recordingOverlayWindowID WindowID = "wox.screenshot.recording.overlay"
	recordingBorderWindowID  WindowID = "wox.screenshot.recording.border"
	// Match screenshot toolbar slots: 16px pad, 40px buttons, 8px gaps (48px stride).
	recordingToolbarPad          = float32(16)
	recordingToolbarGap          = float32(8)
	recordingToolbarButton       = float32(40)
	recordingToolbarSlot         = float32(48)
	recordingToolbarTimeWidth    = float32(56)
	recordingToolbarFPSWidth     = float32(54)
	recordingToolbarWidth        = float32(442)
	recordingToolbarHeight       = float32(60) // Same bar height as the screenshot toolbar.
	recordingToolbarSelectionGap = float32(16) // Same selection-to-toolbar gap as the screenshot toolbar.
	recordingBorderMargin        = float32(40)
)

type recordingKeycap struct {
	label     string
	expiresAt time.Time
}

type recordingToolbarState struct {
	mu                   sync.Mutex
	window               *Window
	overlay              *Window
	overlayManaged       *ManagedWindow
	border               *Window
	borderManaged        *ManagedWindow
	editor               *screenshotEditorOverlayState
	options              ScreenshotOptions
	platform             screenshotEditorPlatform
	selection            Rect
	frameSize            Size
	session              *recordingSession
	fps                  int
	showPointer          bool
	showKeypress         bool
	keyboardUnavailable  string
	keycaps              []recordingKeycap
	finishing            bool
	lastError            string
	hoverTooltip         string
	hoverTooltipRect     Rect
	result               chan recordingUIResult
	timeRect             Rect
	fpsRect              Rect
	primaryRect          Rect
	restartRect          Rect
	pointerRect          Rect
	keypressRect         Rect
	finishRect           Rect
	cancelRect           Rect
	collapsed            bool
	collapseWhileActive  bool
	expandedBounds       Rect
	collapsedBounds      Rect
	borderOrigin         Point
	borderMargin         float32
	borderInteractive    bool
	borderMonitorStop    chan struct{}
	durationTickerStop   chan struct{}
	scrollBorderClose    func()
	keyboardSubscription keyboard.RawKeySubscription
	previewPoster        *Image
	previewFrame         *Image
	previewPlaying       bool
	previewStop          context.CancelFunc
	previewWidth         int
	previewHeight        int
}

type recordingUIResult struct {
	result ScreenshotResult
	err    error
}

// runScreenshotRecording reuses the screenshot window as outside-selection chrome, matching scrolling capture:
// the selection interior has no covering HWND while recording, edge strips mark the region, and a
// selection-sized overlay appears only for countdown and in-place preview.
func runScreenshotRecording(options ScreenshotOptions, editor *screenshotEditorOverlayState, selection Rect, frameSize Size, platform screenshotEditorPlatform) (ScreenshotResult, error) {
	if !options.AllowVideoRecording {
		return ScreenshotResult{}, errors.New("video recording is not enabled for this capture request")
	}
	fps := options.RecordingDefaults.FPS
	if fps != 60 {
		fps = 30
	}
	selection, err := normalizeRecordingLogicalSelection(image.Rect(0, 0, editor.image.Width, editor.image.Height), selection, frameSize)
	if err != nil {
		return ScreenshotResult{}, err
	}
	state := &recordingToolbarState{
		editor: editor, options: options, platform: platform, selection: selection, frameSize: frameSize, fps: fps,
		showPointer: options.RecordingDefaults.ShowPointer, showKeypress: options.RecordingDefaults.ShowKeypress,
		result: make(chan recordingUIResult, 1), borderMonitorStop: make(chan struct{}), durationTickerStop: make(chan struct{}),
	}
	editor.mu.Lock()
	editor.activeTool = screenshotEditorToolSelect
	editor.hasSelectedMark = false
	editor.selection = selection
	editor.annotations = nil
	editor.toolbarRect = Rect{}
	editor.toolRects = [screenshotEditorToolCount]Rect{}
	editor.editBarRect = Rect{}
	editor.undoRect = Rect{}
	editor.scrollRect = Rect{}
	editor.cursorRect = Rect{}
	editor.pinRect = Rect{}
	editor.recordRect = Rect{}
	editor.cancelRect = Rect{}
	editor.confirmRect = Rect{}
	if editor.chromeScale != nil {
		editor.uiScale = max(float32(1), editor.chromeScale(selection))
	}
	editor.recordingUI = state
	state.window = editor.window
	editor.mu.Unlock()
	defer func() {
		editor.mu.Lock()
		editor.recordingUI = nil
		editor.mu.Unlock()
		state.closeScrollBorder()
	}()

	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	var overlayManaged, borderManaged *ManagedWindow
	var openErr error
	if err := Call(func() {
		var created bool
		overlayManaged, created, openErr = manager.Open(recordingOverlayWindowID, WindowOptions{
			Title: "Wox Recording Overlay", Size: Size{Width: 100, Height: 100}, Role: WindowRoleScreenshot,
			Nonactivating: true, HideOnBlur: false, OnFrame: state.drawOverlay, OnPointer: state.overlayPointer, OnClosed: state.closed,
		})
		if openErr != nil {
			return
		}
		if !created {
			openErr = errors.New("a recording overlay window is already active")
			return
		}
		borderManaged, created, openErr = manager.Open(recordingBorderWindowID, WindowOptions{
			Title: "Wox Recording Border", Size: Size{Width: 100, Height: 100}, Role: WindowRoleScreenshot,
			Nonactivating: true, HideOnBlur: false, OnFrame: state.drawBorder, OnPointer: state.borderPointer, OnClosed: state.closed,
		})
		if openErr != nil {
			return
		}
		if !created {
			openErr = errors.New("a recording border window is already active")
			return
		}
		state.overlayManaged = overlayManaged
		state.borderManaged = borderManaged
		state.overlay = overlayManaged.Window()
		state.border = borderManaged.Window()
		openErr = state.positionOverlay()
		if openErr == nil {
			openErr = state.positionBorder()
		}
		if openErr == nil {
			openErr = state.border.SetPointerPassthrough(true)
		}
		if openErr == nil {
			openErr = state.positionToolbar()
		}
		if openErr == nil {
			_, openErr = borderManaged.Show()
		}
		if openErr == nil && state.window != nil {
			openErr = state.window.Invalidate()
		}
	}); err != nil {
		openErr = err
	}
	if openErr != nil {
		if overlayManaged != nil {
			_ = overlayManaged.Close()
		}
		if borderManaged != nil {
			_ = borderManaged.Close()
		}
		return ScreenshotResult{}, openErr
	}
	defer overlayManaged.Close()
	defer borderManaged.Close()
	state.startBorderMonitor()
	defer state.stopBorderMonitor()
	state.startDurationTicker()
	defer state.stopDurationTicker()
	state.probeKeyboard()
	defer state.closeKeyboard()

	outcome := <-state.result
	return outcome.result, outcome.err
}

// startBorderMonitor keeps the transparent selection interior clickable while retaining edge drag handles.
func (state *recordingToolbarState) startBorderMonitor() {
	state.updateBorderPassthrough()
	go func() {
		ticker := time.NewTicker(16 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				state.updateBorderPassthrough()
			case <-state.borderMonitorStop:
				return
			}
		}
	}()
}

func (state *recordingToolbarState) stopBorderMonitor() {
	select {
	case <-state.borderMonitorStop:
	default:
		close(state.borderMonitorStop)
	}
}

// startDurationTicker redraws the toolbar once a second so the timer advances without pointer motion.
func (state *recordingToolbarState) startDurationTicker() {
	go func() {
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if state.recordingStatus() != recordingStateRecording {
					continue
				}
				state.mu.Lock()
				collapsed := state.collapsed
				window := state.window
				state.mu.Unlock()
				if !collapsed && window != nil {
					_ = window.Invalidate()
				}
			case <-state.durationTickerStop:
				return
			}
		}
	}()
}

func (state *recordingToolbarState) stopDurationTicker() {
	select {
	case <-state.durationTickerStop:
	default:
		close(state.durationTickerStop)
	}
}

// updateBorderPassthrough claims input only on the visible border or during an active selection edit.
func (state *recordingToolbarState) updateBorderPassthrough() {
	state.mu.Lock()
	border := state.border
	selection := state.selection
	session := state.session
	interactive := state.borderInteractive
	state.mu.Unlock()
	if border == nil {
		return
	}
	state.editor.mu.Lock()
	editing := state.editor.editMode != screenshotEditorEditNone || state.editor.dragging
	state.editor.mu.Unlock()
	status := recordingStateReady
	if session != nil {
		status = session.currentState()
	}
	ready := status == recordingStateReady
	if !ready {
		return
	}
	shouldIntercept := false
	if state.platform.cursorPosition != nil {
		if pointer := state.platform.cursorPosition(); pointer != nil {
			shouldIntercept = editing || recordingSelectionEdgeContains(selection, *pointer, 14)
		}
	}
	if interactive == shouldIntercept {
		return
	}
	if err := border.SetPointerPassthrough(!shouldIntercept); err != nil {
		return
	}
	state.mu.Lock()
	state.borderInteractive = shouldIntercept
	state.mu.Unlock()
}

func recordingSelectionEdgeContains(selection Rect, point Point, tolerance float32) bool {
	inner := Rect{X: selection.X + tolerance, Y: selection.Y + tolerance, Width: max(float32(0), selection.Width-tolerance*2), Height: max(float32(0), selection.Height-tolerance*2)}
	if screenshotEditorRectContains(selection, point) && !screenshotEditorRectContains(inner, point) {
		return true
	}
	for _, handle := range screenshotEditorRectHandlePoints(selection) {
		if screenshotEditorPointsNear(handle, point, tolerance) {
			return true
		}
	}
	return false
}

// normalizeRecordingLogicalSelection maps the even H.264 crop back to the right and bottom UI edges.
func normalizeRecordingLogicalSelection(source image.Rectangle, selection Rect, frame Size) (Rect, error) {
	pixels, err := screenshotEditorPixelSelection(source, selection, frame)
	if err != nil {
		return Rect{}, err
	}
	pixels = normalizeRecordingPixelBounds(pixels)
	if pixels.Empty() {
		return Rect{}, errors.New("recording selection is too small")
	}
	right := float32(pixels.Max.X-source.Min.X) * frame.Width / float32(source.Dx())
	bottom := float32(pixels.Max.Y-source.Min.Y) * frame.Height / float32(source.Dy())
	selection.Width = max(float32(0), right-selection.X)
	selection.Height = max(float32(0), bottom-selection.Y)
	return selection, nil
}

// positionToolbar tries every outside edge before enabling the in-selection collapsed hotspot.
func (state *recordingToolbarState) positionToolbar() error {
	if state.window == nil {
		return errors.New("recording toolbar window is unavailable")
	}
	scale := state.toolbarChromeScale()
	frame := Rect{Width: state.frameSize.Width, Height: state.frameSize.Height}
	expanded, collapsed, collapseWhileActive := recordingToolbarLayout(frame, state.selection, recordingToolbarWidth*scale, recordingToolbarHeight*scale)
	state.expandedBounds = expanded
	state.collapsedBounds = collapsed
	state.collapseWhileActive = collapseWhileActive
	return setScreenshotScrollingWindowBounds(state.window, state.platform, expanded, state.frameSize)
}

// toolbarChromeScale sizes recording controls with the same DPI used by screenshot chrome.
func (state *recordingToolbarState) toolbarChromeScale() float32 {
	if state.editor != nil && state.editor.chromeScale != nil {
		return max(float32(1), state.editor.chromeScale(state.selection))
	}
	return 1
}

// positionOverlay covers only the selected rectangle for countdown and preview.
func (state *recordingToolbarState) positionOverlay() error {
	if state.overlay == nil {
		return nil
	}
	if state.platform.setRecordingBounds == nil {
		return errors.New("recording selection bounds are unavailable")
	}
	return state.platform.setRecordingBounds(state.overlay, state.selection, state.frameSize, 0)
}

// positionBorder limits the interactive ready-state surface to the selection and its resize handles.
func (state *recordingToolbarState) positionBorder() error {
	margin := state.scaledBorderMargin()
	state.mu.Lock()
	selection := state.selection
	state.borderOrigin = Point{X: selection.X - margin, Y: selection.Y - margin}
	state.borderMargin = margin
	border := state.border
	state.mu.Unlock()
	if border == nil {
		return nil
	}
	if state.platform.setRecordingBounds == nil {
		return errors.New("recording selection bounds are unavailable")
	}
	return state.platform.setRecordingBounds(border, selection, state.frameSize, margin)
}

// scaledBorderMargin leaves room above the selection for the DPI-scaled size chip.
func (state *recordingToolbarState) scaledBorderMargin() float32 {
	return recordingBorderMargin * state.toolbarChromeScale()
}

// recordingToolbarLayout prefers bottom, top, left, and right before using an edge hotspot.
func recordingToolbarLayout(globalFrame, globalSelection Rect, width, height float32) (Rect, Rect, bool) {
	if width <= 0 {
		width = recordingToolbarWidth
	}
	if height <= 0 {
		height = recordingToolbarHeight
	}
	gap := recordingToolbarSelectionGap * height / recordingToolbarHeight
	frameRight, frameBottom := globalFrame.X+globalFrame.Width, globalFrame.Y+globalFrame.Height
	left := min(max(globalFrame.X+8, globalSelection.X+globalSelection.Width-width), frameRight-width-8)
	candidates := []Rect{
		{X: left, Y: globalSelection.Y + globalSelection.Height + gap, Width: width, Height: height},
		{X: left, Y: globalSelection.Y - height - gap, Width: width, Height: height},
		{X: globalSelection.X - width - gap, Y: min(max(globalFrame.Y+8, globalSelection.Y), frameBottom-height-8), Width: width, Height: height},
		{X: globalSelection.X + globalSelection.Width + gap, Y: min(max(globalFrame.Y+8, globalSelection.Y), frameBottom-height-8), Width: width, Height: height},
	}
	for _, candidate := range candidates {
		if candidate.X >= globalFrame.X+8 && candidate.Y >= globalFrame.Y+8 && candidate.X+candidate.Width <= frameRight-8 && candidate.Y+candidate.Height <= frameBottom-8 {
			return candidate, Rect{}, false
		}
	}
	expanded := Rect{
		X:     min(max(globalFrame.X+8, globalSelection.X+(globalSelection.Width-width)/2), frameRight-width-8),
		Y:     min(max(globalFrame.Y+8, globalSelection.Y+globalSelection.Height-height-16), frameBottom-height-8),
		Width: width, Height: height,
	}
	hotspotWidth := 18 * width / recordingToolbarWidth
	hotspotHeight := height
	collapsed := Rect{X: globalSelection.X + globalSelection.Width - hotspotWidth, Y: globalSelection.Y + globalSelection.Height/2 - hotspotHeight/2, Width: hotspotWidth, Height: hotspotHeight}
	return expanded, collapsed, true
}

// recordingStatus is ready until a session exists, then follows that session.
func (state *recordingToolbarState) recordingStatus() recordingState {
	state.mu.Lock()
	session := state.session
	state.mu.Unlock()
	if session == nil {
		return recordingStateReady
	}
	return session.currentState()
}

// syncRecordingSurfaces uncovers the selection while capturing, matching scrolling screenshot.
func (state *recordingToolbarState) syncRecordingSurfaces() {
	status := state.recordingStatus()
	ready := status == recordingStateReady
	coverSelection := status == recordingStateCountdown || status == recordingStateSave
	if ready {
		state.closeScrollBorder()
		_ = state.positionBorder()
		_ = state.setManagedVisible(state.borderManaged, true)
		_ = state.setManagedVisible(state.overlayManaged, false)
		return
	}
	_ = state.setManagedVisible(state.borderManaged, false)
	state.ensureScrollBorder()
	if !coverSelection {
		_ = state.setManagedVisible(state.overlayManaged, false)
		return
	}
	_ = state.positionOverlay()
	if state.overlay != nil {
		_ = state.overlay.SetPointerPassthrough(status != recordingStateSave)
	}
	_ = state.setManagedVisible(state.overlayManaged, true)
	if state.overlay != nil {
		_ = state.overlay.Invalidate()
	}
}

// setManagedVisible skips redundant Show calls so countdown redraws do not steal focus.
func (state *recordingToolbarState) setManagedVisible(managed *ManagedWindow, visible bool) error {
	if managed == nil {
		return nil
	}
	lifecycle := managed.Lifecycle()
	if visible {
		if lifecycle == WindowLifecycleVisible || lifecycle == WindowLifecyclePresenting {
			return nil
		}
		_, err := managed.Show()
		return err
	}
	return managed.Hide()
}

func (state *recordingToolbarState) ensureScrollBorder() {
	state.mu.Lock()
	if state.scrollBorderClose != nil {
		state.mu.Unlock()
		return
	}
	selection := state.selection
	frameSize := state.frameSize
	platform := state.platform
	state.mu.Unlock()
	if platform.showScrollBorder == nil {
		return
	}
	closeBorder, err := platform.showScrollBorder(selection, frameSize)
	if err != nil || closeBorder == nil {
		return
	}
	state.mu.Lock()
	if state.scrollBorderClose != nil {
		state.mu.Unlock()
		closeBorder()
		return
	}
	state.scrollBorderClose = closeBorder
	state.mu.Unlock()
}

func (state *recordingToolbarState) closeScrollBorder() {
	state.mu.Lock()
	closeBorder := state.scrollBorderClose
	state.scrollBorderClose = nil
	state.mu.Unlock()
	if closeBorder != nil {
		closeBorder()
	}
}

// overlayPointer toggles in-place preview when the selection-sized overlay is covering the take.
func (state *recordingToolbarState) overlayPointer(event PointerEvent) {
	if event.Kind != PointerDown || event.Button != PointerButtonPrimary {
		return
	}
	if state.recordingStatus() == recordingStateSave {
		state.togglePreview()
	}
}

// probeKeyboard discovers privacy/access limitations without preventing video capture.
func (state *recordingToolbarState) probeKeyboard() {
	subscription, err := keyboard.AddRawKeyListener(state.rawKey)
	state.mu.Lock()
	defer state.mu.Unlock()
	if err != nil {
		state.keyboardUnavailable = err.Error()
		state.showKeypress = false
		return
	}
	state.keyboardSubscription = subscription
}

// closeKeyboard releases the privacy-sensitive global listener with the recording surfaces.
func (state *recordingToolbarState) closeKeyboard() {
	state.mu.Lock()
	subscription := state.keyboardSubscription
	state.keyboardSubscription = nil
	state.mu.Unlock()
	if subscription != nil {
		_ = subscription.Close()
	}
}

// rawKey feeds text annotations or appends an ephemeral, never-persisted keycap label.
func (state *recordingToolbarState) rawKey(event keyboard.RawKeyEvent) bool {
	if event.Type != keyboard.EventTypeKeyDown {
		return false
	}
	state.mu.Lock()
	if !state.showKeypress {
		state.mu.Unlock()
		return false
	}
	label := recordingKeyLabel(event)
	if label != "" {
		state.keycaps = append(state.keycaps, recordingKeycap{label: label, expiresAt: time.Now().Add(1500 * time.Millisecond)})
		if len(state.keycaps) > 6 {
			state.keycaps = append([]recordingKeycap(nil), state.keycaps[len(state.keycaps)-6:]...)
		}
	}
	overlay := state.overlay
	state.mu.Unlock()
	if overlay != nil && label != "" {
		_ = overlay.Invalidate()
	}
	return false
}

// recordingKeyLabel formats one printable or functional chord for the keycap overlay.
func recordingKeyLabel(event keyboard.RawKeyEvent) string {
	if event.Key.IsModifier() {
		switch event.Key {
		case keyboard.KeyCtrl, keyboard.KeyLeftCtrl, keyboard.KeyRightCtrl:
			return "Ctrl"
		case keyboard.KeyShift, keyboard.KeyLeftShift, keyboard.KeyRightShift:
			return "Shift"
		case keyboard.KeyAlt, keyboard.KeyLeftAlt, keyboard.KeyRightAlt:
			return "Alt"
		case keyboard.KeySuper, keyboard.KeyLeftSuper, keyboard.KeyRightSuper:
			return "Meta"
		}
	}
	parts := make([]string, 0, 5)
	if event.Modifiers&keyboard.ModifierCtrl != 0 {
		parts = append(parts, "Ctrl")
	}
	if event.Modifiers&keyboard.ModifierAlt != 0 {
		parts = append(parts, "Alt")
	}
	if event.Modifiers&keyboard.ModifierShift != 0 {
		parts = append(parts, "Shift")
	}
	if event.Modifiers&keyboard.ModifierSuper != 0 {
		parts = append(parts, "Meta")
	}
	character := event.Character
	if character == "" {
		character = event.Key.Character()
	}
	if character == "" {
		character = recordingFunctionalKeyName(event.Key)
	}
	character = strings.TrimSpace(character)
	if character == "" {
		return ""
	}
	if len([]rune(character)) == 1 {
		character = strings.ToUpper(character)
	}
	parts = append(parts, character)
	return strings.Join(parts, "+")
}

// recordingFunctionalKeyName supplies readable labels for non-printing keys.
func recordingFunctionalKeyName(key keyboard.Key) string {
	switch key {
	case keyboard.KeySpace:
		return "Space"
	case keyboard.KeyReturn:
		return "Enter"
	case keyboard.KeyEscape:
		return "Esc"
	case keyboard.KeyTab:
		return "Tab"
	case keyboard.KeyDelete:
		return "Backspace"
	case keyboard.KeyLeft:
		return "Left"
	case keyboard.KeyRight:
		return "Right"
	case keyboard.KeyUp:
		return "Up"
	case keyboard.KeyDown:
		return "Down"
	case keyboard.KeyCapsLock:
		return "Caps Lock"
	default:
		return ""
	}
}

// drawOverlay renders only capture chrome, live annotations, countdown, and key hints over the desktop.
func (state *recordingToolbarState) drawOverlay(displayList *DisplayList, frame FrameInfo) {
	state.editor.mu.Lock()
	uiScale := max(float32(1), state.editor.uiScale)
	state.editor.mu.Unlock()
	state.mu.Lock()
	now := time.Now()
	keycaps := make([]recordingKeycap, 0, len(state.keycaps))
	for _, keycap := range state.keycaps {
		if now.Before(keycap.expiresAt) {
			keycaps = append(keycaps, keycap)
		}
	}
	state.keycaps = keycaps
	visibleKeycaps := append([]recordingKeycap(nil), keycaps...)
	session := state.session
	state.mu.Unlock()

	displayList.Clear(Color{})
	local := Rect{Width: frame.Size.Width, Height: frame.Size.Height}
	if session != nil && session.currentState() == recordingStateSave {
		state.drawPreview(displayList, local, uiScale)
	}
	if session != nil && session.currentState() == recordingStateCountdown {
		remaining := session.CountdownRemaining()
		seconds := max(1, int((remaining+time.Second-1)/time.Second))
		drawRecordingCountdown(displayList, local, seconds, uiScale)
		if state.overlay != nil {
			_ = state.overlay.Invalidate()
		}
	}
	if len(visibleKeycaps) > 0 && session != nil && session.currentState() == recordingStateCountdown {
		state.drawKeycaps(displayList, local, visibleKeycaps, now, uiScale)
		if state.overlay != nil {
			_ = state.overlay.Invalidate()
		}
	}
}

// drawPreview covers the selection with the recorded poster or live decode, plus a play affordance.
func (state *recordingToolbarState) drawPreview(displayList *DisplayList, selection Rect, uiScale float32) {
	state.mu.Lock()
	frame := state.previewFrame
	playing := state.previewPlaying
	state.mu.Unlock()
	if frame != nil {
		displayList.DrawImage(frame, selection)
	} else {
		displayList.FillRect(selection, Color{R: 0, G: 0, B: 0, A: 180})
	}
	if playing {
		return
	}
	displayList.FillRect(selection, Color{R: 0, G: 0, B: 0, A: 88})
	scale := max(float32(1), uiScale)
	button := min(selection.Width, selection.Height) * 0.22
	button = min(max(button, 56*scale), 120*scale)
	playRect := Rect{
		X: selection.X + (selection.Width-button)/2, Y: selection.Y + (selection.Height-button)/2,
		Width: button, Height: button,
	}
	displayList.FillRoundedRect(playRect, button/2, Color{R: 0, G: 0, B: 0, A: 168})
	displayList.StrokeRoundedRect(playRect, button/2, max(float32(2), 2*scale), Color{R: 255, G: 255, B: 255, A: 230})
	drawScreenshotEditorToolbarIconSized(displayList, "control.play-arrow", playRect, Color{R: 255, G: 255, B: 255, A: 255}, 1, button*0.42)
}

// drawBorder keeps passive blue chrome outside the exact encoded pixel rectangle.
func (state *recordingToolbarState) drawBorder(displayList *DisplayList, frame FrameInfo) {
	state.editor.mu.Lock()
	uiScale := max(float32(1), state.editor.uiScale)
	source := state.editor.image
	state.editor.mu.Unlock()
	state.mu.Lock()
	session := state.session
	selection := state.selection
	margin := state.borderMargin
	state.mu.Unlock()
	displayList.Clear(Color{})
	if margin <= 0 {
		margin = recordingBorderMargin * uiScale
	}
	localSelection := Rect{X: margin, Y: margin, Width: selection.Width, Height: selection.Height}
	blue := Color{R: 47, G: 128, B: 237, A: 255}
	thickness := max(float32(2), 2*uiScale)
	displayList.FillRect(Rect{X: localSelection.X - thickness, Y: localSelection.Y - thickness, Width: localSelection.Width + thickness*2, Height: thickness}, blue)
	displayList.FillRect(Rect{X: localSelection.X - thickness, Y: localSelection.Y + localSelection.Height, Width: localSelection.Width + thickness*2, Height: thickness}, blue)
	displayList.FillRect(Rect{X: localSelection.X - thickness, Y: localSelection.Y, Width: thickness, Height: localSelection.Height}, blue)
	displayList.FillRect(Rect{X: localSelection.X + localSelection.Width, Y: localSelection.Y, Width: thickness, Height: localSelection.Height}, blue)
	if session == nil || session.currentState() == recordingStateReady {
		drawScreenshotEditorHandles(displayList, localSelection, blue, uiScale)
	}
	label := fmt.Sprintf("%d x %d", sessionPixelWidth(session, source, selection, state.frameSize), sessionPixelHeight(session, source, selection, state.frameSize))
	drawScreenshotEditorSizeLabel(displayList, label, localSelection, frame.Size, uiScale)
}

func sessionPixelWidth(session *recordingSession, source *Image, selection Rect, frame Size) int {
	if session != nil {
		return session.config.PixelBounds.Dx()
	}
	return int(selection.Width * float32(source.Width) / frame.Width)
}

func sessionPixelHeight(session *recordingSession, source *Image, selection Rect, frame Size) int {
	if session != nil {
		return session.config.PixelBounds.Dy()
	}
	return int(selection.Height * float32(source.Height) / frame.Height)
}

// drawKeycaps centers at most six fading key labels along the selection bottom edge.
func (state *recordingToolbarState) drawKeycaps(displayList *DisplayList, selection Rect, keycaps []recordingKeycap, now time.Time, scale float32) {
	widths := make([]float32, len(keycaps))
	total := float32(0)
	for index, keycap := range keycaps {
		widths[index] = max(40*scale, screenshotEditorEstimatedTextWidth(keycap.label, 14*scale)+20*scale)
		total += widths[index]
	}
	total += float32(len(keycaps)-1) * 6 * scale
	left := selection.X + (selection.Width-total)/2
	top := selection.Y + selection.Height - 52*scale
	for index, keycap := range keycaps {
		alpha := uint8(230)
		remaining := keycap.expiresAt.Sub(now)
		if remaining < 350*time.Millisecond {
			alpha = uint8(max(int64(0), remaining.Milliseconds()) * 230 / 350)
		}
		rect := Rect{X: left, Y: top, Width: widths[index], Height: 36 * scale}
		displayList.FillRoundedRect(rect, 8*scale, Color{R: 24, G: 24, B: 24, A: alpha})
		displayList.DrawText(keycap.label, Rect{X: rect.X + 10*scale, Y: rect.Y + 8*scale, Width: rect.Width - 20*scale, Height: 20 * scale}, TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: alpha})
		left += rect.Width + 6*scale
	}
}

// renderRecordingKeycaps composites ephemeral key hints into the encoded frame independently of overlay capture.
func renderRecordingKeycaps(target *image.RGBA, selection Rect, frame Size, keycaps []recordingKeycap, now time.Time, scale float32) error {
	if target == nil || len(keycaps) == 0 {
		return nil
	}
	clip, err := screenshotEditorPixelSelection(target.Bounds(), selection, frame)
	if err != nil {
		return err
	}
	widths := make([]float32, len(keycaps))
	total := float32(0)
	for index, keycap := range keycaps {
		widths[index] = max(40*scale, screenshotEditorEstimatedTextWidth(keycap.label, 14*scale)+20*scale)
		total += widths[index]
	}
	total += float32(len(keycaps)-1) * 6 * scale
	left := selection.X + (selection.Width-total)/2
	top := selection.Y + selection.Height - 52*scale
	scaleX := float32(target.Bounds().Dx()) / frame.Width
	scaleY := float32(target.Bounds().Dy()) / frame.Height
	overlay := image.NewRGBA(target.Bounds())
	for index, keycap := range keycaps {
		if !now.Before(keycap.expiresAt) {
			left += widths[index] + 6*scale
			continue
		}
		alpha := uint8(230)
		remaining := keycap.expiresAt.Sub(now)
		if remaining < 350*time.Millisecond {
			alpha = uint8(max(int64(0), remaining.Milliseconds()) * 230 / 350)
		}
		rect := screenshotEditorScaleRect(Rect{X: left, Y: top, Width: widths[index], Height: 36 * scale}, scaleX, scaleY)
		centerY := (rect.Min.Y + rect.Max.Y) / 2
		radius := float32(rect.Dy())
		drawScreenshotEditorPixelLine(overlay, clip, image.Pt(rect.Min.X+rect.Dy()/2, centerY), image.Pt(rect.Max.X-rect.Dy()/2, centerY), radius, color.RGBA{R: 24, G: 24, B: 24, A: alpha})
		textPoint := screenshotEditorScalePoint(Point{X: left + 10*scale, Y: top + 8*scale}, scaleX, scaleY)
		drawScreenshotEditorPixelText(overlay, clip, keycap.label, textPoint, 14*scale*scaleY, color.RGBA{R: 255, G: 255, B: 255, A: alpha})
		left += widths[index] + 6*scale
	}
	draw.Draw(target, clip, overlay, clip.Min, draw.Over)
	return nil
}

// recordingToolbarControlLayout uses the same 16px pad, 40px buttons, and 48px stride as the screenshot toolbar.
func recordingToolbarControlLayout(scale float32) (panel, timeRect, fpsRect, primary, restart, pointer, keypress, finish, cancel Rect) {
	if scale <= 0 {
		scale = 1
	}
	panel = Rect{Y: 0, Width: recordingToolbarWidth * scale, Height: recordingToolbarHeight * scale}
	controlTop := 10 * scale
	slotLeft := recordingToolbarPad * scale
	timeRect = Rect{X: slotLeft, Y: controlTop, Width: recordingToolbarTimeWidth * scale, Height: recordingToolbarButton * scale}
	slotLeft += (recordingToolbarTimeWidth + recordingToolbarGap) * scale
	fpsRect = Rect{X: slotLeft, Y: controlTop, Width: recordingToolbarFPSWidth * scale, Height: recordingToolbarButton * scale}
	slotLeft += (recordingToolbarFPSWidth + recordingToolbarGap) * scale
	button := func() Rect {
		rect := Rect{X: slotLeft + 4*scale, Y: controlTop, Width: recordingToolbarButton * scale, Height: recordingToolbarButton * scale}
		slotLeft += recordingToolbarSlot * scale
		return rect
	}
	primary = button()
	restart = button()
	pointer = button()
	keypress = button()
	finish = button()
	cancel = button()
	return panel, timeRect, fpsRect, primary, restart, pointer, keypress, finish, cancel
}

// drawToolbar renders controls from a locked snapshot so callbacks never retain mutable state.
func (state *recordingToolbarState) drawToolbar(displayList *DisplayList, frame FrameInfo) {
	state.mu.Lock()
	if state.collapsed {
		state.mu.Unlock()
		displayList.Clear(Color{})
		return
	}
	session := state.session
	fps := state.fps
	showPointer := state.showPointer
	showKeypress := state.showKeypress
	keyboardAvailable := state.keyboardUnavailable == ""
	finishing := state.finishing
	previewPlaying := state.previewPlaying
	hoverTooltip := state.hoverTooltip
	hoverTooltipRect := state.hoverTooltipRect
	state.mu.Unlock()
	recordingStatus := recordingStateReady
	duration := time.Duration(0)
	if session != nil {
		recordingStatus = session.currentState()
		duration = session.EffectiveDuration()
	}
	scale := float32(1)
	if frame.Size.Height > 0 {
		scale = frame.Size.Height / recordingToolbarHeight
	}

	displayList.Clear(Color{})
	panel, timeRect, fpsRect, primaryRect, restartRect, pointerRect, keypressRect, finishRect, cancelRect := recordingToolbarControlLayout(scale)
	panel.Width = frame.Size.Width
	state.timeRect = timeRect
	state.fpsRect = fpsRect
	state.primaryRect = primaryRect
	state.restartRect = restartRect
	state.pointerRect = pointerRect
	state.keypressRect = keypressRect
	state.finishRect = finishRect
	state.cancelRect = cancelRect
	white := Color{R: 255, G: 255, B: 255, A: 255}
	displayList.FillRoundedRect(panel, 18*scale, Color{R: 30, G: 26, B: 24, A: 248})
	displayList.DrawText(formatRecordingDuration(duration), Rect{X: timeRect.X, Y: timeRect.Y + 10*scale, Width: timeRect.Width, Height: 20 * scale}, TextStyle{Size: 14 * scale, Weight: FontWeightSemibold}, white)
	displayList.DrawText(fmt.Sprintf("%d FPS", fps), Rect{X: fpsRect.X, Y: fpsRect.Y + 10*scale, Width: fpsRect.Width, Height: 20 * scale}, TextStyle{Size: 13 * scale, Weight: FontWeightSemibold}, Color{R: 255, G: 255, B: 255, A: 235})
	primaryIcon := "control.record"
	primaryColor := Color{R: 255, G: 75, B: 75, A: 255}
	if recordingStatus == recordingStateRecording {
		primaryIcon = "control.pause"
		primaryColor = white
	} else if recordingStatus == recordingStatePaused {
		primaryIcon = "control.play-circle"
		primaryColor = white
	} else if recordingStatus == recordingStateSave {
		primaryIcon = "control.play-circle"
		primaryColor = white
		if previewPlaying {
			primaryIcon = "control.pause"
		}
	}
	drawScreenshotEditorToolbarIcon(displayList, primaryIcon, primaryRect, primaryColor, scale)
	drawScreenshotEditorToolbarIcon(displayList, "control.refresh", restartRect, white, scale)
	state.drawToggle(displayList, pointerRect, "screenshot.cursor", showPointer, true, scale)
	state.drawToggle(displayList, keypressRect, "control.keyboard", showKeypress, keyboardAvailable, scale)
	finishIcon := "control.stop"
	if recordingStatus == recordingStateSave {
		finishIcon = "control.download"
	}
	finishColor := Color{R: 255, G: 107, B: 107, A: 255}
	if finishing {
		finishColor.A = 100
	}
	drawScreenshotEditorToolbarIcon(displayList, finishIcon, finishRect, finishColor, scale)
	drawScreenshotEditorToolbarIcon(displayList, "control.close", cancelRect, Color{R: 255, G: 107, B: 107, A: 255}, scale)
	drawScreenshotEditorToolTooltip(displayList, frame.Size, hoverTooltipRect, hoverTooltip, scale)
}

func (state *recordingToolbarState) drawToggle(displayList *DisplayList, rect Rect, icon string, enabled, available bool, scale float32) {
	if scale <= 0 {
		scale = 1
	}
	color := Color{R: 255, G: 255, B: 255, A: 255}
	if !available {
		color.A = 80
	} else if enabled {
		displayList.FillRoundedRect(rect, 10*scale, Color{R: 255, G: 255, B: 255, A: 40})
	}
	drawScreenshotEditorToolbarIconSized(displayList, icon, rect, color, scale, 20)
}

func formatRecordingDuration(duration time.Duration) string {
	seconds := int(duration / time.Second)
	return fmt.Sprintf("%02d:%02d", seconds/60, seconds%60)
}

// toolbarPointer applies state-aware controls and locks capture options after preparation begins.
func (state *recordingToolbarState) toolbarPointer(event PointerEvent) {
	if event.Kind == PointerMove || event.Kind == PointerLeave {
		state.mu.Lock()
		current := recordingStateReady
		if state.session != nil {
			current = state.session.currentState()
		}
		if event.Kind == PointerLeave {
			state.hoverTooltip = ""
			state.hoverTooltipRect = Rect{}
		} else {
			state.hoverTooltipRect, state.hoverTooltip = state.tooltipAt(event.Position, current)
		}
		window := state.window
		state.mu.Unlock()
		if window != nil {
			_ = window.Invalidate()
		}
		return
	}
	if event.Kind != PointerDown || event.Button != PointerButtonPrimary {
		return
	}
	state.mu.Lock()
	session := state.session
	if state.collapsed {
		state.mu.Unlock()
		if session != nil && session.currentState() == recordingStateRecording {
			_ = session.Pause()
			session.DiscardPendingFrames()
		}
		state.setToolbarCollapsed(false)
		return
	}
	if state.finishing {
		state.mu.Unlock()
		return
	}
	current := recordingStateReady
	if session != nil {
		current = session.currentState()
	}
	locked := current != recordingStateReady
	switch {
	case screenshotEditorRectContains(state.fpsRect, event.Position) && !locked:
		if state.fps == 30 {
			state.fps = 60
		} else {
			state.fps = 30
		}
	case screenshotEditorRectContains(state.pointerRect, event.Position) && !locked:
		state.showPointer = !state.showPointer
	case screenshotEditorRectContains(state.keypressRect, event.Position) && !locked:
		if state.keyboardUnavailable == "" {
			state.showKeypress = !state.showKeypress
		}
	case screenshotEditorRectContains(state.primaryRect, event.Position):
		state.mu.Unlock()
		state.primaryAction(current)
		return
	case screenshotEditorRectContains(state.restartRect, event.Position) && session != nil:
		state.mu.Unlock()
		state.restart()
		return
	case screenshotEditorRectContains(state.finishRect, event.Position) && session != nil:
		state.mu.Unlock()
		state.finish()
		return
	case screenshotEditorRectContains(state.cancelRect, event.Position):
		state.mu.Unlock()
		state.cancel()
		return
	}
	window := state.window
	state.mu.Unlock()
	if window != nil {
		_ = window.Invalidate()
	}
}

// tooltipAt resolves the localized hint for the visible control under the pointer.
func (state *recordingToolbarState) tooltipAt(point Point, current recordingState) (Rect, string) {
	switch {
	case screenshotEditorRectContains(state.fpsRect, point):
		return state.fpsRect, "30 / 60 FPS"
	case screenshotEditorRectContains(state.primaryRect, point):
		switch current {
		case recordingStateRecording:
			return state.primaryRect, state.options.RecordingTooltips.Pause
		case recordingStatePaused:
			return state.primaryRect, state.options.RecordingTooltips.Resume
		case recordingStateSave:
			if state.previewPlaying {
				return state.primaryRect, state.options.RecordingTooltips.Pause
			}
			return state.primaryRect, state.options.RecordingTooltips.Play
		default:
			return state.primaryRect, state.options.RecordingTooltips.Start
		}
	case screenshotEditorRectContains(state.restartRect, point):
		return state.restartRect, state.options.RecordingTooltips.Restart
	case screenshotEditorRectContains(state.pointerRect, point):
		return state.pointerRect, state.options.RecordingTooltips.ShowPointer
	case screenshotEditorRectContains(state.keypressRect, point):
		if state.keyboardUnavailable != "" {
			return state.keypressRect, state.keyboardUnavailable
		}
		return state.keypressRect, state.options.RecordingTooltips.ShowKeypress
	case screenshotEditorRectContains(state.finishRect, point):
		if current == recordingStateSave {
			return state.finishRect, state.options.RecordingTooltips.Save
		}
		return state.finishRect, state.options.RecordingTooltips.Finish
	case screenshotEditorRectContains(state.cancelRect, point):
		return state.cancelRect, state.options.RecordingTooltips.Cancel
	}
	return Rect{}, ""
}

// primaryAction dispatches start, pause, and resume through the recording state machine.
func (state *recordingToolbarState) primaryAction(current recordingState) {
	switch current {
	case recordingStateReady:
		state.start()
	case recordingStateRecording:
		state.mu.Lock()
		session := state.session
		state.mu.Unlock()
		_ = session.Pause()
	case recordingStatePaused:
		state.mu.Lock()
		session := state.session
		collapse := state.collapseWhileActive
		fps := state.fps
		state.mu.Unlock()
		if collapse {
			state.setToolbarCollapsed(true)
			go func() {
				time.Sleep(time.Second / time.Duration(fps))
				_ = session.Resume()
			}()
		} else {
			_ = session.Resume()
		}
	case recordingStateSave:
		state.togglePreview()
	}
	_ = state.window.Invalidate()
}

// newSession binds desktop capture, live annotation composition, and H.264 encoding for one take.
func (state *recordingToolbarState) newSession() (*recordingSession, error) {
	state.mu.Lock()
	fps, showPointer, showKeypress, selection := state.fps, state.showPointer, state.showKeypress, state.selection
	state.mu.Unlock()
	pixelSelection, err := screenshotEditorPixelSelection(image.Rect(0, 0, state.editor.image.Width, state.editor.image.Height), selection, state.frameSize)
	if err != nil {
		return nil, err
	}
	pixelSelection = normalizeRecordingPixelBounds(pixelSelection)
	if pixelSelection.Empty() {
		return nil, errors.New("recording selection is too small")
	}
	captureFrame := func() (image.Image, error) {
		if state.platform.captureDesktopRect != nil {
			return state.platform.captureDesktopRect(pixelSelection)
		}
		capture, captureErr := state.platform.captureDesktop()
		if captureErr != nil {
			return nil, captureErr
		}
		defer capture.close()
		return cropRecordingFrame(capture.source, pixelSelection)
	}
	releaseCapture := func() {}
	if state.platform.openRecordingCapture != nil {
		captureFn, releaseFn, captureErr := state.platform.openRecordingCapture(pixelSelection)
		if captureErr != nil {
			return nil, captureErr
		}
		captureFrame = func() (image.Image, error) { return captureFn() }
		releaseCapture = releaseFn
	}
	return newRecordingSession(recordingSessionConfig{
		FPS: fps, ShowPointer: showPointer, ShowKeypress: showKeypress, PixelBounds: image.Rect(0, 0, pixelSelection.Dx(), pixelSelection.Dy()),
		Capture: captureFrame,
		Compose: func(source image.Image) (*image.RGBA, error) {
			frame, composeErr := cropRecordingFrame(source, pixelSelection)
			if composeErr != nil {
				return nil, composeErr
			}
			state.editor.mu.Lock()
			uiScale := max(float32(1), state.editor.uiScale)
			chromeScale := state.editor.chromeScale
			state.editor.mu.Unlock()
			if chromeScale != nil {
				uiScale = max(uiScale, chromeScale(selection))
			}
			state.mu.Lock()
			keycaps := append([]recordingKeycap(nil), state.keycaps...)
			state.mu.Unlock()
			var pointer *Point
			if showPointer && state.platform.cursorPosition != nil {
				pointer = state.platform.cursorPosition()
			}
			if overlayErr := applyRecordingOverlays(frame, pixelSelection, pointer, keycaps, showPointer, showKeypress, time.Now(), uiScale); overlayErr != nil {
				return nil, overlayErr
			}
			return frame, nil
		},
		Encoder: &ffmpegRecordingEncoder{}, Release: releaseCapture, Diagnostics: true, OnChanged: func() {
			state.syncRecordingSurfaces()
			if state.window != nil {
				_ = state.window.Invalidate()
			}
			if state.overlay != nil {
				_ = state.overlay.Invalidate()
			}
			if state.border != nil {
				_ = state.border.Invalidate()
			}
		},
	})
}

// start locks the selected region and begins the countdown.
func (state *recordingToolbarState) start() {
	session, err := state.newSession()
	if err == nil {
		state.mu.Lock()
		state.session = session
		state.lastError = ""
		state.mu.Unlock()
		err = session.Start(context.Background())
		if err == nil {
			state.syncRecordingSurfaces()
			state.mu.Lock()
			collapse := state.collapseWhileActive
			state.mu.Unlock()
			if collapse {
				state.setToolbarCollapsed(true)
			}
		}
	}
	if err != nil {
		state.mu.Lock()
		state.lastError = err.Error()
		state.mu.Unlock()
	}
	_ = state.window.Invalidate()
}

// setToolbarCollapsed switches between the full controls and transparent edge hotspot.
func (state *recordingToolbarState) setToolbarCollapsed(collapsed bool) {
	state.mu.Lock()
	if !state.collapseWhileActive || state.collapsed == collapsed {
		state.mu.Unlock()
		return
	}
	state.collapsed = collapsed
	bounds := state.expandedBounds
	if collapsed {
		bounds = state.collapsedBounds
	}
	window := state.window
	state.mu.Unlock()
	_ = setScreenshotScrollingWindowBounds(window, state.platform, bounds, state.frameSize)
	if window != nil {
		_ = window.Invalidate()
	}
}

// restart discards the current MP4 and starts a fresh countdown with the same selection.
func (state *recordingToolbarState) restart() {
	state.stopPreview()
	state.mu.Lock()
	previous := state.session
	state.session = nil
	state.keycaps = nil
	state.lastError = ""
	state.previewPoster = nil
	state.previewFrame = nil
	state.mu.Unlock()
	if previous != nil {
		_ = previous.Cancel()
	}
	state.start()
}

// finish stops an in-progress take, or opens Save As only after the user asks to download.
func (state *recordingToolbarState) finish() {
	state.mu.Lock()
	if state.finishing {
		state.mu.Unlock()
		return
	}
	session := state.session
	state.mu.Unlock()
	if session == nil {
		return
	}
	switch session.currentState() {
	case recordingStateSave:
		state.saveRecording()
	case recordingStateCountdown, recordingStatePreparing:
		_ = session.Cancel()
		state.mu.Lock()
		state.session = nil
		state.mu.Unlock()
		if state.window != nil {
			_ = state.window.Invalidate()
		}
		state.syncRecordingSurfaces()
	default:
		state.stopRecording()
	}
}

// stopRecording finalizes the MP4 and keeps the overlay open for in-place preview.
func (state *recordingToolbarState) stopRecording() {
	state.mu.Lock()
	if state.finishing {
		state.mu.Unlock()
		return
	}
	state.finishing = true
	session := state.session
	selection := state.selection
	state.mu.Unlock()
	go func() {
		path, err := session.Finish()
		if err != nil {
			state.setFinishError(err)
			return
		}
		width, height := recordingPreviewPixelSize(selection.Width, selection.Height)
		poster, posterErr := extractRecordingPreviewFrame(path, width, height)
		var preview *Image
		if posterErr == nil {
			preview, posterErr = NewImage(poster)
		}
		state.mu.Lock()
		state.finishing = false
		state.previewWidth = width
		state.previewHeight = height
		if posterErr == nil {
			state.previewPoster = preview
			state.previewFrame = preview
			state.lastError = ""
		} else {
			state.lastError = posterErr.Error()
		}
		overlay := state.overlay
		border := state.border
		window := state.window
		state.mu.Unlock()
		if overlay != nil {
			_ = overlay.Invalidate()
		}
		if border != nil {
			_ = border.Invalidate()
		}
		if window != nil {
			_ = window.Invalidate()
		}
		state.syncRecordingSurfaces()
	}()
}

// saveRecording asks for a destination only when the download control is used.
func (state *recordingToolbarState) saveRecording() {
	state.mu.Lock()
	if state.finishing {
		state.mu.Unlock()
		return
	}
	state.finishing = true
	session := state.session
	state.mu.Unlock()
	go func() {
		state.stopPreview()
		var target string
		var saveErr error
		callErr := Call(func() {
			defaultName := time.Now().Format("20060102_150405") + "_wox_recording.mp4"
			target, saveErr = state.window.SaveFile(SaveFileOptions{Title: "Save recording", DefaultFileName: defaultName, Extension: "mp4"})
		})
		if callErr != nil {
			state.setFinishError(callErr)
			return
		}
		if saveErr != nil {
			state.setFinishError(saveErr)
			return
		}
		if target == "" {
			state.mu.Lock()
			state.finishing = false
			window := state.window
			state.mu.Unlock()
			if window != nil {
				_ = window.Invalidate()
			}
			return
		}
		if filepath.Ext(target) == "" {
			target += ".mp4"
		}
		if err := session.Save(target); err != nil {
			state.setFinishError(err)
			return
		}
		state.complete(recordingUIResult{result: ScreenshotResult{
			ArtifactKind: "video", ArtifactPath: target, LogicalSelection: state.platform.logicalSelection(state.selection, state.frameSize),
		}})
	}()
}

// setFinishError re-enables Finish so the same finalized temporary file can be retried.
func (state *recordingToolbarState) setFinishError(err error) {
	state.mu.Lock()
	state.finishing = false
	state.lastError = err.Error()
	state.mu.Unlock()
	_ = state.window.Invalidate()
}

// togglePreview starts or stops in-overlay playback of the finalized recording.
func (state *recordingToolbarState) togglePreview() {
	state.mu.Lock()
	playing := state.previewPlaying
	state.mu.Unlock()
	if playing {
		state.stopPreview()
		return
	}
	state.startPreview()
}

// stopPreview returns the overlay to the poster frame and tears down the decoder.
func (state *recordingToolbarState) stopPreview() {
	state.mu.Lock()
	cancel := state.previewStop
	state.previewStop = nil
	state.previewPlaying = false
	state.previewFrame = state.previewPoster
	overlay := state.overlay
	window := state.window
	state.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if overlay != nil {
		_ = overlay.Invalidate()
	}
	if window != nil {
		_ = window.Invalidate()
	}
}

// startPreview decodes the finalized MP4 into the selection overlay until the user pauses or it ends.
func (state *recordingToolbarState) startPreview() {
	state.mu.Lock()
	if state.previewPlaying {
		state.mu.Unlock()
		return
	}
	session := state.session
	width, height := state.previewWidth, state.previewHeight
	state.mu.Unlock()
	if session == nil || session.currentState() != recordingStateSave {
		return
	}
	path := session.TempPath()
	if path == "" || width < 2 || height < 2 {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	state.mu.Lock()
	state.previewStop = cancel
	state.previewPlaying = true
	state.mu.Unlock()
	go func() {
		err := pumpRecordingPreview(ctx, path, width, height, func(frame *image.RGBA) error {
			preview, imageErr := NewImage(frame)
			if imageErr != nil {
				return imageErr
			}
			state.mu.Lock()
			if !state.previewPlaying {
				state.mu.Unlock()
				return context.Canceled
			}
			state.previewFrame = preview
			overlay := state.overlay
			state.mu.Unlock()
			if overlay != nil {
				_ = overlay.Invalidate()
			}
			return nil
		})
		state.mu.Lock()
		state.previewPlaying = false
		state.previewStop = nil
		state.previewFrame = state.previewPoster
		if err != nil && !errors.Is(err, context.Canceled) {
			state.lastError = err.Error()
		}
		overlay := state.overlay
		window := state.window
		state.mu.Unlock()
		if overlay != nil {
			_ = overlay.Invalidate()
		}
		if window != nil {
			_ = window.Invalidate()
		}
	}()
}

func (state *recordingToolbarState) cancel() {
	state.stopPreview()
	state.mu.Lock()
	session := state.session
	state.mu.Unlock()
	if session != nil {
		_ = session.Cancel()
	}
	state.complete(recordingUIResult{result: ScreenshotResult{Cancelled: true}})
}

func (state *recordingToolbarState) complete(result recordingUIResult) {
	select {
	case state.result <- result:
	default:
	}
}

func (state *recordingToolbarState) closed() {
	state.cancel()
}

// borderPointer reuses screenshot selection editing while translating the small border-local surface to desktop coordinates.
func (state *recordingToolbarState) borderPointer(event PointerEvent) {
	state.mu.Lock()
	session := state.session
	if session != nil && session.currentState() == recordingStateSave {
		selection := state.selection
		origin := state.borderOrigin
		state.mu.Unlock()
		point := Point{X: event.Position.X + origin.X, Y: event.Position.Y + origin.Y}
		if event.Kind == PointerDown && event.Button == PointerButtonPrimary && screenshotEditorRectContains(selection, point) {
			state.togglePreview()
		}
		return
	}
	if session != nil && session.currentState() != recordingStateReady {
		state.mu.Unlock()
		return
	}
	origin := state.borderOrigin
	state.mu.Unlock()
	event.Position.X += origin.X
	event.Position.Y += origin.Y
	state.editor.pointer(event)
	state.editor.mu.Lock()
	selection := state.editor.selection
	state.editor.mu.Unlock()
	if event.Kind == PointerUp {
		normalized, err := normalizeRecordingLogicalSelection(image.Rect(0, 0, state.editor.image.Width, state.editor.image.Height), selection, state.frameSize)
		if err == nil {
			selection = normalized
			state.editor.mu.Lock()
			state.editor.selection = normalized
			state.editor.mu.Unlock()
		}
	}
	state.mu.Lock()
	state.selection = selection
	state.mu.Unlock()
	if state.border != nil {
		_ = state.border.Invalidate()
	}
	if event.Kind == PointerUp {
		if state.border != nil {
			_ = state.positionBorder()
		}
		if state.window != nil {
			_ = state.positionToolbar()
		}
		_ = state.positionOverlay()
	}
}
