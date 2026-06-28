package ui

// WidgetType identifies the kind of widget for native-side dispatch.
type WidgetType int32

const (
	WidgetVBox WidgetType = iota
	WidgetHBox
	WidgetText
	WidgetTextBox
	WidgetListBox
	WidgetImage
	WidgetSeparator
	WidgetSpacer
	WidgetPreviewPanel
	WidgetToolbar
)

// Widget is the base interface for all DSL elements.
// Each concrete widget is a plain struct — the layout engine reads
// fields through type switches rather than virtual dispatch.
type Widget interface {
	Type() WidgetType
}

// VBox is a vertical stack container — children laid out top to bottom.
type VBox struct {
	Children  []Widget
	Gap       float32 // spacing between children in DIP
	Padding   float32 // inner padding in DIP
	BgColor   *Color  // optional background fill
	MinWidth  float32
	MinHeight float32
}

func (w VBox) Type() WidgetType { return WidgetVBox }

// HBox is a horizontal stack container — children laid out left to right.
type HBox struct {
	Children  []Widget
	Gap       float32
	Padding   float32
	BgColor   *Color
	MinWidth  float32
	MinHeight float32
}

func (w HBox) Type() WidgetType { return WidgetHBox }

// Text is a static text label.
type Text struct {
	Content    string
	FontSize   float32
	FontColor  Color
	FontFamily string
	Bold       bool
	MaxWidth   float32 // 0 = no wrap
}

func (w Text) Type() WidgetType { return WidgetText }

// TextBox is an editable text input field with IME support.
// Events are delivered through the EventCallbacks set on the Window.
type TextBox struct {
	ID           string
	Placeholder  string
	FontSize     float32
	FontColor    Color
	BgColor      Color
	CornerRadius float32
	CursorColor  Color
	Value        string
	Focused      bool

	// CursorPos is the byte offset of the caret within Value. 0 = before the
	// first character. The layout engine measures Value[:CursorPos] to position
	// the caret bar, so callers must keep it in sync after every edit.
	CursorPos int

	// SelectionStart / SelectionEnd delimit the selected range as byte offsets.
	// When both are -1 there is no selection. SelectionStart <= SelectionEnd.
	SelectionStart int
	SelectionEnd   int

	// SelectionColor fills the selection highlight rectangle behind the text.
	SelectionColor Color

	// BlinkVisible toggles caret visibility on each blink tick. The caller
	// drives a timer that flips this flag; layoutTextBox only draws the caret
	// when both Focused and BlinkVisible are true.
	BlinkVisible bool
}

func (w TextBox) Type() WidgetType { return WidgetTextBox }

// ListBox is a scrollable list of items.
// ItemRenderer builds a widget subtree for each visible item.
type ListBox struct {
	ID            string
	Items         []ListItem
	ItemHeight    float32
	ScrollOffset  float32
	Selected      int // index of highlighted item, -1 = none
	ItemRenderer  func(index int, item ListItem) Widget
	BgColor       *Color
	SelectedColor *Color
	// Width, when > 0, fixes the list width so an HBox parent allocates
	// exactly this much horizontal space instead of the natural measured width.
	Width float32
}

func (w ListBox) Type() WidgetType { return WidgetListBox }

// ListItem is a single entry in a ListBox.
type ListItem struct {
	Title    string
	Subtitle string
	IconPNG  []byte // optional pre-rasterized icon (PNG bytes)
	IconKey  string // stable key for native bitmap cache
	IconSVG  string // optional raw SVG (rasterized by Go side before draw)
	Data     any    // opaque payload for the caller
}

// Image is a static image element (PNG bytes).
type Image struct {
	PNGData  []byte
	ImageKey string
	Width    float32 // 0 = natural size
	Height   float32
}

func (w Image) Type() WidgetType { return WidgetImage }

// Separator is a horizontal or vertical divider line.
type Separator struct {
	Orientation Orientation
	Color       Color
	Thickness   float32
}

func (w Separator) Type() WidgetType { return WidgetSeparator }

// Spacer is an elastic gap that expands to fill available space.
type Spacer struct {
	Size float32 // minimum size in DIP
}

func (w Spacer) Type() WidgetType { return WidgetSpacer }

// Orientation for separators and layout directions.
type Orientation int32

const (
	OrientHorizontal Orientation = iota
	OrientVertical
)

// PreviewTag is a metadata chip rendered below the preview content.
// Label is the visible text; Tooltip is the explanatory hover text.
type PreviewTag struct {
	Label   string
	Tooltip string
}

// PreviewPanel renders the preview for the active query result.
//
// Supported PreviewType values:
//   - "text":     PreviewData is plain text, auto-wrapped within the panel width.
//   - "markdown": PreviewData is a minimal markdown subset (headings, lists, code
//     blocks, quotes, separators). Inline styling (e.g. **bold**) is stripped.
//   - "image":    PreviewData is a WoxImage.String() ("type:data"). The caller
//     rasterizes it to PNG and fills ImagePNG/ImageKey; until then "Loading..."
//     is shown.
//   - "remote":   handled by the caller (gpuUIImpl) — it fetches the real
//     preview via HTTP and replaces this panel before layout. If a remote
//     panel reaches layout, "Loading..." is rendered.
//
// ScrollOffset shifts content vertically; the panel clips to its own rect so
// only the visible portion is drawn.
type PreviewPanel struct {
	ID           string
	PreviewType  string
	PreviewData  string
	PreviewTags  []PreviewTag
	ScrollOffset float32
	BgColor      *Color
	SplitColor   Color
	FontColor    Color
	FontSize     float32
	FontFamily   string
	// ImagePNG / ImageKey hold the rasterized preview image for the "image" type.
	// Filled asynchronously by the caller before requesting a repaint.
	ImagePNG []byte
	ImageKey string
	// Width, when > 0, fixes the panel width so an HBox parent allocates
	// exactly this much horizontal space instead of the natural measured width.
	Width float32
}

func (w PreviewPanel) Type() WidgetType { return WidgetPreviewPanel }

// ToolbarAction describes one clickable action rendered on the toolbar right side.
// Hotkey is the display text for the shortcut (e.g. "Ctrl+U"). Action is the
// Go-side callback invoked on click; nil means the action is display-only.
type ToolbarAction struct {
	Label  string
	Hotkey string
	Action func()
}

// Toolbar renders the launcher's bottom toolbar: a left status/message area
// (icon + text + optional progress) and a row of right-aligned action buttons
// with hotkey hints. When Visible is false the toolbar contributes zero height
// and draws nothing, so the root VBox collapses cleanly.
type Toolbar struct {
	ID            string
	Height        float32
	Visible       bool
	LeftIcon      []byte // optional PNG icon on the left
	LeftIconKey   string
	LeftText      string
	Progress      *int // 0-100, nil means no progress bar
	Indeterminate bool // indeterminate spinner instead of percentage
	Actions       []ToolbarAction
	BgColor       Color
	FontColor     Color
	PaddingLeft   float32
	PaddingRight  float32
	TopBorder     bool // draw a 1px top separator line (shown when results exist)
}

func (w Toolbar) Type() WidgetType { return WidgetToolbar }
