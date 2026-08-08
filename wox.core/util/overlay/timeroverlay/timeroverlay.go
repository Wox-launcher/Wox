package timeroverlay

import (
	"sync"

	"wox/util/overlay"
)

// Options configures a countdown HUD attachment on a base overlay window.
type Options struct {
	Window overlay.WindowOptions

	// Countdown is the primary line, e.g. "01:23" or "1h 02m 03s".
	Countdown string
	// Note is an optional secondary line shown smaller below the countdown.
	Note string
	// Closable reserves a close control that appears only while the cursor is over the overlay.
	Closable bool

	CountdownFontSize float64 // 0 => native default
	NoteFontSize      float64 // 0 => native default
}

type timerRenderer struct {
	id         string
	generation uint64
	handle     uintptr
	width      float64
	height     float64
}

// showMu keeps renderer updates and base attachment registration in the same order.
var showMu sync.Mutex

// Show displays or updates a timer overlay while keeping content concerns out of the base overlay call sites.
func Show(opts Options) {
	showMu.Lock()
	defer showMu.Unlock()

	overlay.RegisterClickCallback(opts.Window.ID, nil)

	window := opts.Window
	renderer, ok := newTimerRenderer(opts)
	if ok {
		attachment := renderer.nativeAttachment()
		attachment.OnRelease = renderer.destroy
		window.NativeAttachment = attachment
	}

	overlay.ShowWindow(window)
}
