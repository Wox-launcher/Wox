//go:build linux

package screenshot

import (
	"errors"
	"fmt"
	"time"
	"wox/util"
)

type linuxScreenshotOutputSelection struct {
	output    int
	selection Rect
	outcome   screenshotEditorOverlayOutcome
}

type linuxScreenshotSelectionSurface struct {
	state   *screenshotEditorOverlayState
	host    *screenshotEditorWindowHost
	managed *ManagedWindow
}

// linuxScreenshotSelectionSession owns one frozen, native-resolution surface per output.
// Input and lifecycle fields stay on the UI thread; only result crosses to the capture worker.
type linuxScreenshotSelectionSession struct {
	outputs   []linuxDesktopOutput
	surfaces  []linuxScreenshotSelectionSurface
	active    int
	drag      int
	completed bool
	closed    bool
	result    chan linuxScreenshotOutputSelection
}

// selectLinuxScreenshotOutput keeps all masks visible until the selected output's editor takes over.
func selectLinuxScreenshotOutput(options ScreenshotOptions, outputs []linuxDesktopOutput, windows []linuxScreenshotWindow) (linuxScreenshotOutputSelection, func(), error) {
	session, err := newLinuxScreenshotSelectionSession(options, outputs, windows)
	if err != nil {
		return linuxScreenshotOutputSelection{}, nil, err
	}
	manager := options.WindowManager
	if manager == nil {
		manager = NewWindowManager()
	}
	var openErr error
	if err := Call(func() { openErr = session.open(manager) }); err != nil {
		session.close()
		return linuxScreenshotOutputSelection{}, nil, err
	}
	if openErr != nil {
		session.close()
		return linuxScreenshotOutputSelection{}, nil, openErr
	}
	timer := time.NewTimer(175 * time.Second)
	defer timer.Stop()
	select {
	case selected := <-session.result:
		return selected, session.close, nil
	case <-timer.C:
		return linuxScreenshotOutputSelection{outcome: screenshotEditorOverlayOutcome{cancelled: true}}, session.close, nil
	}
}

// newLinuxScreenshotSelectionSession reuses the editor's hover, color inspector, and region gestures without exporting.
func newLinuxScreenshotSelectionSession(options ScreenshotOptions, outputs []linuxDesktopOutput, windows []linuxScreenshotWindow) (*linuxScreenshotSelectionSession, error) {
	if len(outputs) == 0 {
		return nil, errors.New("screenshot selection requires at least one output")
	}
	session := &linuxScreenshotSelectionSession{
		outputs: outputs, active: -1, drag: -1, result: make(chan linuxScreenshotOutputSelection, 1),
	}
	options.AutoConfirm = true
	options.HideAnnotationToolbar = true
	options.AllowVideoRecording = false
	options.ExtraActions = nil
	for _, output := range outputs {
		if output.source == nil || output.bounds.Width <= 0 || output.bounds.Height <= 0 {
			return nil, errors.New("screenshot output has no pixels or logical bounds")
		}
		prepared, err := newScreenshotEditorImage(output.source)
		if err != nil {
			return nil, fmt.Errorf("prepare output selection image: %w", err)
		}
		candidates, query := linuxScreenshotObjectSelection(windows, output.bounds, output.source)
		platform := screenshotEditorPlatform{
			frameSize:        Size{Width: output.bounds.Width, Height: output.bounds.Height},
			windowCandidates: candidates, objectQuery: query,
			displayCandidates:  []Rect{{Width: output.bounds.Width, Height: output.bounds.Height}},
			desktopPixelOrigin: screenshotEditorDesktopPixelOrigin(output.bounds, output.source),
		}
		state := newScreenshotEditorOverlayState(options, prepared, platform)
		state.originalSource = output.source
		session.surfaces = append(session.surfaces, linuxScreenshotSelectionSurface{state: state, host: &screenshotEditorWindowHost{}})
	}
	return session, nil
}

// open prepares every hidden surface before mapping any of them, so setup failures cannot leave a partial selector.
func (session *linuxScreenshotSelectionSession) open(manager *WindowManager) error {
	for index := range session.surfaces {
		surface := &session.surfaces[index]
		if err := surface.host.begin(surface.state); err != nil {
			return err
		}
		managed, created, err := manager.Open(WindowID(fmt.Sprintf("%s.output.%d", ScreenshotWindowID, index)), WindowOptions{
			Title: "Wox Screenshot", Size: Size{Width: 100, Height: 100}, Role: WindowRoleScreenshot,
			OnFrame:   surface.host.draw,
			OnPointer: func(event PointerEvent) { session.pointer(index, event) },
			OnKey:     session.key,
			OnClosed: func() {
				surface.host.closed()
				session.finish(index, screenshotEditorOverlayOutcome{cancelled: true})
			},
		})
		if err != nil {
			return err
		}
		if !created {
			return errors.New("a screenshot output selector is already active")
		}
		surface.managed = managed
		surface.state.window = managed.Window()
		surface.state.writeClipboardText = managed.Window().WriteClipboardText
		if err := managed.Window().SetBounds(session.outputs[index].bounds); err != nil {
			return err
		}
		surface.state.startObjectSelection()
	}
	for _, surface := range session.surfaces {
		if _, err := surface.managed.Show(); err != nil {
			return err
		}
	}
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[screenshot] multi-output selector ready outputs=%d", len(session.surfaces)))
	return nil
}

// pointer keeps a drag on its starting output while allowing hover to move freely between masks.
func (session *linuxScreenshotSelectionSession) pointer(index int, event PointerEvent) {
	if session.completed || session.closed {
		return
	}
	target := index
	if session.drag >= 0 {
		if event.Kind == PointerLeave || event.Kind == PointerEnter {
			return
		}
		target = session.drag
		origin, bounds := session.outputs[index].bounds, session.outputs[target].bounds
		event.Position = clampScreenshotEditorPoint(Point{X: origin.X + event.Position.X - bounds.X, Y: origin.Y + event.Position.Y - bounds.Y}, Rect{Width: bounds.Width, Height: bounds.Height})
	} else if event.Kind != PointerLeave {
		if session.active >= 0 && session.active != target {
			session.surfaces[session.active].state.pointer(PointerEvent{Kind: PointerLeave})
		}
		session.active = target
	}
	if event.Kind == PointerEnter {
		event.Kind = PointerMove
	}
	if event.Kind == PointerDown && event.Button == PointerButtonPrimary {
		session.drag = target
	}
	session.surfaces[target].state.pointer(event)
	if event.Kind == PointerUp && event.Button == PointerButtonPrimary {
		session.drag = -1
	}
	if event.Kind == PointerLeave && session.active == target {
		session.active = -1
	}
	session.poll(target)
}

// key routes shortcuts to the pointer's output even when layer-shell gives keyboard focus to another mask.
func (session *linuxScreenshotSelectionSession) key(event KeyEvent) bool {
	if session.completed || session.closed || !event.Down || event.Composing {
		return false
	}
	if event.Key == KeyEscape {
		session.finish(0, screenshotEditorOverlayOutcome{cancelled: true})
		return true
	}
	if session.active < 0 {
		return false
	}
	switch event.Key {
	case Key("g"), Key("h"), KeyArrowLeft, KeyArrowUp, KeyArrowRight, KeyArrowDown:
		handled := session.surfaces[session.active].state.key(event)
		session.poll(session.active)
		return handled
	default:
		return false
	}
}

// poll consumes the selected surface's ordinary editor outcome without running clipboard/file export.
func (session *linuxScreenshotSelectionSession) poll(index int) {
	select {
	case outcome := <-session.surfaces[index].state.result:
		session.finish(index, outcome)
	default:
	}
}

// finish publishes at most one output and freezes input during the editor handoff.
func (session *linuxScreenshotSelectionSession) finish(index int, outcome screenshotEditorOverlayOutcome) {
	if session.completed || session.closed {
		return
	}
	session.completed = true
	state := session.surfaces[index].state
	state.mu.Lock()
	selection := state.selection
	state.mu.Unlock()
	util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[screenshot] multi-output selection output=%d selection=%+v cancelled=%t copiedColor=%t", index, selection, outcome.cancelled, outcome.copiedColor != ""))
	session.result <- linuxScreenshotOutputSelection{output: index, selection: selection, outcome: outcome}
}

// close also releases unselected rasters as soon as the editor is shown, rather than retaining them until export.
func (session *linuxScreenshotSelectionSession) close() {
	_ = Call(func() {
		if session.closed {
			return
		}
		session.closed, session.completed = true, true
		for _, surface := range session.surfaces {
			surface.host.end(surface.state)
			surface.state.stopObjectSelection()
			if surface.managed != nil {
				surface.state.setPointerCursor(PointerCursorDefault)
				_ = surface.managed.Close()
			}
		}
		session.surfaces = nil
		session.outputs = nil
	})
}
