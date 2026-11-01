package textoverlay

import (
	"image"

	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

// Options configures a text-based runtime overlay.
type Options struct {
	Window       overlay.WindowOptions
	Title        string
	TitleIcon    image.Image
	Message      string
	HotkeyLabels []string
	Icon         image.Image
	Loading      bool
	Closable     bool
	// AutoCloseSeconds closes the text overlay after the delay unless the cursor is still over it.
	AutoCloseSeconds int
	// FontSize overrides the default message font size; zero keeps DefaultFontSize.
	FontSize float32
	// Padding overrides the panel padding; zero keeps the shared overlay padding.
	Padding woxwidget.Insets

	CenterContent bool
	FollowScroll  bool
	IconSize      float64
	IconTextGap   float32

	Tooltip         string
	TooltipIcon     image.Image
	TooltipIconSize float64

	ShowCopyButton           bool
	CopyButtonTooltip        string
	CopyButtonSuccessTooltip string
	OnClick                  func() bool
	// ToastTitle and ToastAction are used for compact notification windows.
	ToastTitle       string
	ToastActionLabel string
	OnToastAction    func()
}

// Show displays or updates a text overlay through the shared Go UI runtime.
func Show(options Options) {
	showRuntimeTextOverlay(options)
}

// ShowWithResult reports whether the overlay could be displayed.
func ShowWithResult(options Options) bool {
	return showRuntimeTextOverlay(options)
}
