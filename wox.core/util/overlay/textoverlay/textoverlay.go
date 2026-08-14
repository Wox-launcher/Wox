package textoverlay

import (
	"image"

	"wox/util/overlay"
)

// Options configures a text-based runtime overlay.
type Options struct {
	Window    overlay.WindowOptions
	Title     string
	TitleIcon image.Image
	Message   string
	Icon      image.Image
	Loading   bool
	Closable  bool
	// AutoCloseSeconds closes the text overlay after the delay unless the cursor is still over it.
	AutoCloseSeconds int

	CenterContent bool
	FollowScroll  bool
	IconSize      float64

	Tooltip         string
	TooltipIcon     image.Image
	TooltipIconSize float64

	ShowCopyButton           bool
	CopyButtonTooltip        string
	CopyButtonSuccessTooltip string
	OnClick                  func() bool
}

// Show displays or updates a text overlay through the shared Go UI runtime.
func Show(options Options) {
	showRuntimeTextOverlay(options)
}
