package overlay

import (
	"image"
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
)

// OverlayOptions defines the configuration for displaying an overlay window.
type OverlayOptions struct {
	// Name is a unique identifier for the overlay. Reusing the same name updates the existing overlay.
	Name string
	// Title of the overlay, primarily for accessibility or window management (platform dependent).
	Title string
	// Message is the main text content to display.
	Message string
	// Icon is the image for the icon. If nil, no icon is shown.
	Icon image.Image
	// Closable determines if a close button (X) is shown.
	Closable bool
	// StickyWindowPid determines the positioning context.
	// If 0, the overlay is positioned relative to the screen (work area).
	// If > 0, the overlay is positioned relative to the window owned by this PID.
	StickyWindowPid int
	// Anchor defines the reference point on the target (Screen or Window) and the Overlay itself.
	// For example, AnchorBottomRight means the bottom-right corner of the overlay aligns with the bottom-right corner of the target.
	// Use the Anchor* constants.
	Anchor int
	// OffsetX moves the overlay horizontally from the anchor point.
	// Positive values move to the Right. Negative values move to the Left.
	OffsetX float64
	// OffsetY moves the overlay vertically from the anchor point.
	// Positive values move Down. Negative values move Up.
	OffsetY float64
	// AutoCloseSeconds is the duration in seconds after which the overlay automatically closes.
	// If 0, it does not auto-close.
	// If the mouse is hovering over the overlay, the timer is paused until the mouse leaves.
	AutoCloseSeconds int
	// Movable determines if the overlay can be dragged by the user.
	Movable bool
	// Width of the overlay. If 0, it auto-sizes based on content/default.
	Width float64
	// Height of the overlay. If 0, it auto-sizes based on content.
	Height float64
	// FontSize controls message font size in points.
	// If 0, use the current system font size.
	FontSize float64
	// IconSize controls icon size in DIP/pt.
	// If 0, defaults to 16.
	IconSize float64
	// Tooltip is the text to display when hovering over the tooltip icon.
	Tooltip string
	// TooltipIcon is the image for the tooltip icon. If nil, a default icon is shown (if Tooltip is set).
	TooltipIcon image.Image
	// TooltipIconSize controls tooltip icon size in DIP/pt.
	// If 0, defaults to 16.
	TooltipIconSize float64
	// OnClick is a callback function invoked when the overlay body is clicked.
	OnClick func()
}
