package overlay

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

// NativeAttachmentKind identifies the platform handle type embedded inside an overlay.
type NativeAttachmentKind int

const (
	NativeAttachmentKindNone NativeAttachmentKind = iota
	NativeAttachmentKindView
	NativeAttachmentKindWindow
)

// NativeAttachment lets overlay subpackages attach platform-owned content
// without adding business-specific fields to the base overlay API.
type NativeAttachment struct {
	Kind      NativeAttachmentKind
	Handle    uintptr
	Width     float64
	Height    float64
	OnRelease func()
}

func (attachment NativeAttachment) active() bool {
	return attachment.Kind != NativeAttachmentKindNone && attachment.Handle != 0
}

// WindowOptions defines only the window-level behavior of an overlay.
type WindowOptions struct {
	// ID is a unique identifier for the overlay. Reusing the same ID updates the existing overlay.
	ID string
	// Transparent makes the overlay a clear drawing surface instead of the default HUD background.
	Transparent bool
	// HitTestIconOnly lets transparent overlay whitespace pass through while keeping content interactive.
	HitTestIconOnly bool
	// CloseOnEscape lets a focused overlay close itself on Esc.
	CloseOnEscape bool
	// TakeFocus makes the overlay steal keyboard focus when it appears so Esc
	// works without an extra click. Only meaningful on Windows; macOS already
	// focuses CloseOnEscape overlays via NonactivatingPanel. Use this only for
	// overlays that need immediate keyboard dismissal (e.g. dictation recording).
	TakeFocus bool
	// NativeAttachment embeds platform-native content supplied by overlay subpackages.
	NativeAttachment NativeAttachment
	// Topmost puts the overlay above Wox's launcher window instead of using the default notification level.
	Topmost bool
	// AbsolutePosition treats OffsetX/OffsetY as desktop-absolute coordinates for AnchorTopLeft.
	AbsolutePosition bool
	// PreservePosition keeps an existing overlay at its current window position during content updates.
	PreservePosition bool
	// StickyWindowPid determines the positioning context. If 0, the overlay is positioned relative to the screen.
	StickyWindowPid int
	// Anchor defines the reference point on the target and the overlay itself.
	Anchor int
	// OffsetX moves the overlay horizontally from the anchor point.
	OffsetX float64
	// OffsetY moves the overlay vertically from the anchor point.
	OffsetY float64
	// Movable determines if the overlay can be dragged by the user.
	Movable bool
	// Resizable lets native overlay windows be resized by dragging their edges.
	Resizable bool
	// CornerRadius controls the overlay window corner radius in DIP/pt.
	CornerRadius float64
	// AspectRatio keeps a resizable overlay at width/height while the user resizes it.
	AspectRatio float64
	// Width of the overlay. If 0, it auto-sizes based on content/default.
	Width float64
	// MinWidth overrides the platform default minimum width when auto-sizing overlays.
	MinWidth float64
	// MaxWidth caps auto-sized overlays.
	MaxWidth float64
	// Height of the overlay. If 0, it auto-sizes based on content.
	Height float64
	// MaxHeight caps auto-sized overlays.
	MaxHeight float64
	// OnClose is invoked when the overlay is closed by the user.
	OnClose func()
}

// ShowWindow displays an overlay using only window-level configuration.
func ShowWindow(opts WindowOptions) {
	RegisterCloseCallback(opts.ID, opts.OnClose)
	releaseOldAttachment := RegisterNativeAttachment(opts.ID, opts.NativeAttachment)
	showWindow(opts)
	if releaseOldAttachment != nil {
		releaseOldAttachment()
	}
}
