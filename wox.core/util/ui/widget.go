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
	Content   string
	FontSize  float32
	FontColor Color
	FontFamily string
	Bold      bool
	MaxWidth  float32 // 0 = no wrap
}

func (w Text) Type() WidgetType { return WidgetText }

// TextBox is an editable text input field with IME support.
// Events are delivered through the EventCallbacks set on the Window.
type TextBox struct {
	ID          string
	Placeholder string
	FontSize    float32
	FontColor   Color
	BgColor     Color
	CornerRadius float32
	CursorColor Color
	Value       string
	Focused     bool
}

func (w TextBox) Type() WidgetType { return WidgetTextBox }

// ListBox is a scrollable list of items.
// ItemRenderer builds a widget subtree for each visible item.
type ListBox struct {
	ID           string
	Items        []ListItem
	ItemHeight   float32
	ScrollOffset float32
	Selected     int // index of highlighted item, -1 = none
	ItemRenderer func(index int, item ListItem) Widget
	BgColor      *Color
	SelectedColor *Color
}

func (w ListBox) Type() WidgetType { return WidgetListBox }

// ListItem is a single entry in a ListBox.
type ListItem struct {
	Title      string
	Subtitle   string
	IconPNG    []byte // optional pre-rasterized icon (PNG bytes)
	IconSVG    string // optional raw SVG (rasterized by Go side before draw)
	Data       any    // opaque payload for the caller
}

// Image is a static image element (PNG bytes).
type Image struct {
	PNGData []byte
	Width   float32 // 0 = natural size
	Height  float32
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