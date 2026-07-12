package textoverlay

import (
	"bytes"
	"image"
	"image/png"
	"sync"

	"wox/util/overlay"
)

// Options configures a text-based overlay preset on top of a base overlay window.
type Options struct {
	Window   overlay.WindowOptions
	Message  string
	Icon     image.Image
	Loading  bool
	Closable bool
	// AutoCloseSeconds closes the text overlay after the delay unless the cursor is still over it.
	AutoCloseSeconds int

	CenterContent bool
	FollowScroll  bool
	FontSize      float64
	IconSize      float64

	Tooltip         string
	TooltipIcon     image.Image
	TooltipIconSize float64

	ShowCopyButton           bool
	CopyButtonTooltip        string
	CopyButtonSuccessTooltip string
	OnClick                  func() bool
}

type textRenderer struct {
	id         string
	generation uint64
	handle     uintptr
	width      float64
	height     float64
}

// showMu keeps renderer updates and base attachment registration in the same order.
var showMu sync.Mutex

// Show displays or updates a text overlay while keeping content concerns out of the base overlay call sites.
func Show(opts Options) {
	showMu.Lock()
	defer showMu.Unlock()

	overlay.RegisterClickCallback(opts.Window.ID, opts.OnClick)

	window := opts.Window
	renderer, ok := newTextRenderer(opts)
	if ok {
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
