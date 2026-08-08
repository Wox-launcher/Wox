package iconoverlay

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"wox/util/overlay"
)

// Options configures an overlay whose native content is a single transparent icon.
type Options struct {
	Window overlay.WindowOptions
	Icon   image.Image

	// IconSize is the logical icon size inside the overlay surface. If unset, the smaller
	// overlay dimension is used.
	IconSize float64
	OnClick  func() bool
}

type iconRenderer struct {
	id         string
	generation uint64
	handle     uintptr
	width      float64
	height     float64
}

var showMu sync.Mutex

// Show displays an icon overlay and keeps the native icon lifecycle tied to its base window.
func Show(opts Options) {
	showMu.Lock()
	defer showMu.Unlock()

	overlay.RegisterClickCallback(opts.Window.ID, opts.OnClick)
	window := opts.Window
	if window.Width <= 0 || window.Height <= 0 {
		if opts.Icon != nil {
			bounds := opts.Icon.Bounds()
			if window.Width <= 0 {
				window.Width = float64(bounds.Dx())
			}
			if window.Height <= 0 {
				window.Height = float64(bounds.Dy())
			}
		}
	}

	if renderer, ok := newIconRenderer(Options{Window: window, Icon: opts.Icon, IconSize: opts.IconSize, OnClick: opts.OnClick}); ok {
		attachment := renderer.nativeAttachment()
		attachment.OnRelease = renderer.destroy
		window.NativeAttachment = attachment
	}
	overlay.ShowWindow(window)
}

func imageToPNG(img image.Image) ([]byte, error) {
	if img == nil {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
