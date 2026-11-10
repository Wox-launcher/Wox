//go:build linux

package screenshot

import (
	"errors"
	"fmt"
	"image"
	"sync"
	"wox/util"
)

type linuxDesktopCapture interface {
	capture() (image.Image, error)
	logicalBounds() Rect
	close()
}

// linuxDesktopOutput keeps each frozen output in its own logical and native pixel spaces.
type linuxDesktopOutput struct {
	source  image.Image
	bounds  Rect
	capture func() (image.Image, error)
}

// linuxMultiOutputCapture is optional; single-stream backends retain their existing workflow.
type linuxMultiOutputCapture interface {
	captureOutputs() ([]linuxDesktopOutput, error)
}

// captureLinuxWaylandScreenshot hands the chosen output to the ordinary editor after a multi-output selection.
func captureLinuxWaylandScreenshot(options ScreenshotOptions, capture linuxDesktopCapture, cursor *Point, windows []linuxScreenshotWindow) (ScreenshotResult, error) {
	if multi, ok := capture.(linuxMultiOutputCapture); ok {
		outputs, err := multi.captureOutputs()
		if err != nil {
			return ScreenshotResult{}, err
		}
		if len(outputs) == 0 {
			return ScreenshotResult{}, errors.New("desktop capture returned no outputs")
		}
		if len(outputs) > 1 {
			selected, dismiss, err := selectLinuxScreenshotOutput(options, outputs, windows)
			if err != nil {
				return ScreenshotResult{}, err
			}
			defer dismiss()
			if selected.outcome.cancelled {
				return ScreenshotResult{Cancelled: true}, nil
			}
			if selected.outcome.copiedColor != "" {
				return ScreenshotResult{CopiedColor: selected.outcome.copiedColor}, nil
			}
			output := outputs[selected.output]
			platform := linuxScreenshotEditorPlatform(options, output.source, output.bounds, cursor, windows, output.capture)
			platform.frameSize = Size{Width: output.bounds.Width, Height: output.bounds.Height}
			platform.initialSelection = &selected.selection
			// Keep the frozen masks until the selected output's editor is ready, avoiding a desktop flash.
			platform.afterShow = dismiss
			return runScreenshotEditor(options, output.source, platform)
		}
		output := outputs[0]
		platform := linuxScreenshotEditorPlatform(options, output.source, output.bounds, cursor, windows, output.capture)
		return runScreenshotEditor(options, output.source, platform)
	}
	source, err := capture.capture()
	if err != nil {
		return ScreenshotResult{}, err
	}
	platform := linuxScreenshotEditorPlatform(options, source, capture.logicalBounds(), cursor, windows, capture.capture)
	return runScreenshotEditor(options, source, platform)
}

// linuxWaylandCaptureBackend describes one compositor-specific capture implementation.
type linuxWaylandCaptureBackend struct {
	name     string
	priority int
	matches  func() bool
	open     func() (linuxDesktopCapture, error)
}

var linuxWaylandCaptureBackends struct {
	sync.Mutex
	items []linuxWaylandCaptureBackend
}

// registerLinuxWaylandCaptureBackend lets a desktop-specific file add a backend without changing shared dispatch.
func registerLinuxWaylandCaptureBackend(backend linuxWaylandCaptureBackend) {
	if backend.name == "" || backend.matches == nil || backend.open == nil {
		panic("invalid Linux Wayland screenshot backend")
	}
	linuxWaylandCaptureBackends.Lock()
	defer linuxWaylandCaptureBackends.Unlock()
	linuxWaylandCaptureBackends.items = append(linuxWaylandCaptureBackends.items, backend)
}

// newLinuxWaylandDesktopCapture selects the highest-priority matching desktop backend.
func newLinuxWaylandDesktopCapture() (linuxDesktopCapture, error) {
	linuxWaylandCaptureBackends.Lock()
	backends := append([]linuxWaylandCaptureBackend(nil), linuxWaylandCaptureBackends.items...)
	linuxWaylandCaptureBackends.Unlock()

	if backend := linuxMatchingWaylandCaptureBackend(backends); backend != nil {
		util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("[screenshot] selected Linux Wayland backend=%s", backend.name))
		return backend.open()
	}
	return nil, fmt.Errorf("no Linux Wayland screenshot backend is registered")
}

func linuxMatchingWaylandCaptureBackend(backends []linuxWaylandCaptureBackend) *linuxWaylandCaptureBackend {
	selected := -1
	for index, backend := range backends {
		if backend.matches() && (selected < 0 || backend.priority > backends[selected].priority) {
			selected = index
		}
	}
	if selected < 0 {
		return nil
	}
	return &backends[selected]
}
