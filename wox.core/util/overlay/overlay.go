package overlay

import (
	"image"
)

// OverlayImageKind identifies the transport used for an overlay image.
type OverlayImageKind string

const (
	OverlayImageKindImage OverlayImageKind = "image"
	OverlayImageKindFile  OverlayImageKind = "file"
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

// OverlayImage describes how the overlay icon is supplied to the native layer.
// Large pinned screenshots already exist as files, so a file-backed icon avoids
// the previous image.Image decode plus PNG re-encode cost in the Go bridge while
// preserving the in-memory image path used by notification overlays.
type OverlayImage struct {
	Kind     OverlayImageKind
	Image    image.Image
	FilePath string
}

func NewImageIcon(img image.Image) OverlayImage {
	return OverlayImage{
		Kind:  OverlayImageKindImage,
		Image: img,
	}
}

func NewFileIcon(filePath string) OverlayImage {
	return OverlayImage{
		Kind:     OverlayImageKindFile,
		FilePath: filePath,
	}
}

func (img OverlayImage) activeKind() OverlayImageKind {
	if img.Kind != "" {
		return img.Kind
	}
	if img.Image != nil {
		// Compatibility fallback for callers that construct OverlayImage literals
		// without a Kind while migrating from the old image.Image-only API.
		return OverlayImageKindImage
	}
	if img.FilePath != "" {
		return OverlayImageKindFile
	}
	return ""
}

// OverlayOptions defines the configuration for displaying an overlay window.
type OverlayOptions struct {
	// Name is a unique identifier for the overlay. Reusing the same name updates the existing overlay.
	Name string
	// Title of the overlay, primarily for accessibility or window management (platform dependent).
	Title string
	// Message is the main text content to display.
	Message string
	// Icon is the image source for the icon. If empty, no icon is shown.
	Icon OverlayImage
	// Transparent makes the overlay a clear drawing surface instead of the default notification HUD.
	// This is a generic surface mode for modules that need custom drawing without the default frame.
	Transparent bool
	// HitTestIconOnly lets transparent overlay whitespace pass through while keeping the icon interactive.
	// It keeps non-content regions from acting like invisible blocking windows.
	HitTestIconOnly bool
	// IconX and IconY position Icon inside a transparent overlay using DIP/pt coordinates from the top-left.
	IconX float64
	IconY float64
	// IconWidth and IconHeight draw Icon at a custom size inside a transparent overlay.
	// If either value is 0, IconSize or the source image size is used as a fallback.
	IconWidth  float64
	IconHeight float64
	// Closable determines if a close button (X) is shown.
	Closable bool
	// CloseOnEscape lets a focused overlay close itself on Esc.
	// This is intentionally handled inside the overlay window instead of a global key listener so
	// only the overlay with keyboard focus is dismissed.
	CloseOnEscape bool
	// Loading shows a native indeterminate spinner next to the message.
	// Use it for short-lived progress surfaces where the caller should not keep refreshing text.
	Loading bool
	// CenterContent centers the leading icon/spinner and message as a single group inside fixed-size HUD overlays.
	CenterContent bool
	// Topmost puts the overlay above Wox's launcher window instead of using the default notification
	// level. Use it for user-requested pinned/preview surfaces, not transient notifications.
	Topmost bool
	// AbsolutePosition treats OffsetX/OffsetY as desktop-absolute coordinates for AnchorTopLeft.
	// It is used when callers already resolved their own anchor, such as screenshot pins or
	// pointer-following progress overlays, and the native layer must not re-anchor to the
	// primary work area like a notification.
	AbsolutePosition bool
	// PreservePosition keeps an existing overlay at its current window position during content updates.
	PreservePosition bool
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
	// Resizable lets native overlay windows be resized by dragging their edges.
	// It is opt-in so transient notification overlays keep their compact fixed-size behavior.
	Resizable bool
	// CornerRadius controls the overlay window corner radius in DIP/pt.
	// If 0, platform defaults are used by the specific overlay style.
	CornerRadius float64
	// AspectRatio keeps a resizable overlay at width/height while the user resizes it.
	// If 0, resizing can change width and height independently.
	AspectRatio float64
	// Width of the overlay. If 0, it auto-sizes based on content/default.
	Width float64
	// MinWidth overrides the platform default minimum width when auto-sizing overlays. If 0, platform defaults are used.
	MinWidth float64
	// MaxWidth caps auto-sized overlays. It does not replace Width; callers can omit Width to grow with content until this cap.
	MaxWidth float64
	// Height of the overlay. If 0, it auto-sizes based on content.
	Height float64
	// MaxHeight caps auto-sized overlays. It does not replace Height; callers can omit Height to grow with content until this cap.
	MaxHeight float64
	// FollowScroll keeps scrollable text pinned to the bottom until the user scrolls away.
	FollowScroll bool
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
	// ShowCopyButton displays a copy affordance in the bottom-right corner.
	ShowCopyButton bool
	// CopyButtonTooltip is displayed when hovering over ShowCopyButton.
	CopyButtonTooltip string
	// CopyButtonSuccessTooltip is displayed briefly after OnClick reports success.
	CopyButtonSuccessTooltip string
	// OnClick is a callback function invoked when the overlay body or copy button is clicked.
	OnClick func() bool
}
