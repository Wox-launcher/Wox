package screenshot

import (
	"errors"
	"sync"
)

type screenshotEditorWindowHost struct {
	mu    sync.RWMutex
	state *screenshotEditorOverlayState
}

func (host *screenshotEditorWindowHost) begin(state *screenshotEditorOverlayState) error {
	host.mu.Lock()
	defer host.mu.Unlock()
	if host.state != nil {
		return errors.New("a screenshot window is already active")
	}
	host.state = state
	return nil
}

func (host *screenshotEditorWindowHost) end(state *screenshotEditorOverlayState) {
	host.mu.Lock()
	if host.state == state {
		host.state = nil
	}
	host.mu.Unlock()
}

func (host *screenshotEditorWindowHost) current() *screenshotEditorOverlayState {
	host.mu.RLock()
	state := host.state
	host.mu.RUnlock()
	return state
}

func (host *screenshotEditorWindowHost) draw(displayList *DisplayList, frame FrameInfo) {
	if state := host.current(); state != nil {
		state.draw(displayList, frame)
	} else {
		displayList.Clear(Color{})
	}
}

func (host *screenshotEditorWindowHost) pointer(event PointerEvent) {
	if state := host.current(); state != nil {
		state.pointer(event)
	}
}

func (host *screenshotEditorWindowHost) key(event KeyEvent) bool {
	if state := host.current(); state != nil {
		return state.key(event)
	}
	return false
}

func (host *screenshotEditorWindowHost) textInput(event TextInputEvent) {
	if state := host.current(); state != nil {
		state.textInput(event)
	}
}

func (host *screenshotEditorWindowHost) closed() {
	if state := host.current(); state != nil {
		state.complete(true)
	}
	host.mu.Lock()
	host.state = nil
	host.mu.Unlock()
}

// prepareScreenshotEditorWindow creates one session-owned hidden native surface on the UI thread.
func prepareScreenshotEditorWindow(manager *WindowManager, host *screenshotEditorWindowHost) (*ManagedWindow, error) {
	managed, created, err := manager.Open(ScreenshotWindowID, WindowOptions{
		Title:       "Wox Screenshot",
		Size:        Size{Width: 100, Height: 100},
		Role:        WindowRoleScreenshot,
		HideOnBlur:  false,
		OnFrame:     host.draw,
		OnPointer:   host.pointer,
		OnKey:       host.key,
		OnTextInput: host.textInput,
		OnClosed:    host.closed,
	})
	if err != nil {
		return nil, err
	}
	if !created {
		return nil, errors.New("a screenshot window is already active")
	}
	return managed, nil
}

// prepareScreenshotEditorWindowOnUIThread overlaps cold renderer creation with desktop capture.
func prepareScreenshotEditorWindowOnUIThread(manager *WindowManager, host *screenshotEditorWindowHost) (*ManagedWindow, error) {
	if manager == nil {
		return nil, nil
	}
	var managed *ManagedWindow
	var prepareErr error
	if err := Call(func() {
		managed, prepareErr = prepareScreenshotEditorWindow(manager, host)
	}); err != nil {
		return nil, err
	}
	return managed, prepareErr
}
