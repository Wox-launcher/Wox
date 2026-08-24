package woxui

import "runtime"

// Key names the portable semantic keys needed by widgets and shortcuts.
// Printable keys use their lowercase Unicode text, such as Key("a") or Key("1").
type Key string

const (
	KeyUnknown    Key = ""
	KeyBackspace  Key = "backspace"
	KeyTab        Key = "tab"
	KeyEnter      Key = "enter"
	KeyEscape     Key = "escape"
	KeySpace      Key = "space"
	KeyPageUp     Key = "page-up"
	KeyPageDown   Key = "page-down"
	KeyEnd        Key = "end"
	KeyHome       Key = "home"
	KeyArrowLeft  Key = "arrow-left"
	KeyArrowUp    Key = "arrow-up"
	KeyArrowRight Key = "arrow-right"
	KeyArrowDown  Key = "arrow-down"
	KeyDelete     Key = "delete"
	// KeyAlt is Alt on Windows and Linux, Option on macOS.
	KeyAlt Key = "alt"
	// KeyMeta is Windows/Super on Windows and Linux, Command on macOS.
	KeyMeta Key = "meta"
)

// HasPrimary reports Command on macOS and Control on other desktop platforms.
func (m KeyModifiers) HasPrimary() bool {
	if runtime.GOOS == "darwin" {
		return m&KeyModifierMeta != 0
	}
	return m&KeyModifierControl != 0
}

// HasWordModifier reports Option on macOS and Control elsewhere for word-wise editing.
func (m KeyModifiers) HasWordModifier() bool {
	if runtime.GOOS == "darwin" {
		return m&KeyModifierAlt != 0
	}
	return m&KeyModifierControl != 0
}

// HasLineModifier reports Command on macOS for line-boundary navigation.
// Other platforms use Home/End (and multiline soft-line Home/End) instead.
func (m KeyModifiers) HasLineModifier() bool {
	if runtime.GOOS == "darwin" {
		return m&KeyModifierMeta != 0
	}
	return false
}

// KeyModifiers is a platform-neutral modifier bit set.
type KeyModifiers uint8

const (
	KeyModifierShift KeyModifiers = 1 << iota
	KeyModifierControl
	KeyModifierAlt
	KeyModifierMeta
)

// KeyEvent reports a semantic key transition before text input processing.
type KeyEvent struct {
	Key       Key
	Modifiers KeyModifiers
	Down      bool
	Repeat    bool
	Composing bool
}

// TextInputEventKind distinguishes committed text from an in-progress IME composition.
type TextInputEventKind uint8

const (
	TextInputCommit TextInputEventKind = iota
	TextInputCompose
)

// TextInputEvent carries UTF-8 text from the platform input method.
// An empty composition clears the current marked text.
type TextInputEvent struct {
	Kind TextInputEventKind
	Text string
}

// TextInputState tells the platform whether an editor is active and where IME UI should appear.
type TextInputState struct {
	Enabled    bool
	CursorRect Rect
}

// PointerEventKind identifies one mouse or trackpad transition.
type PointerEventKind uint8

const (
	PointerMove PointerEventKind = iota
	PointerEnter
	PointerLeave
	PointerDown
	PointerUp
	PointerScroll
)

// PointerButton names the button involved in a pointer transition.
type PointerButton uint8

const (
	PointerButtonNone PointerButton = iota
	PointerButtonPrimary
	PointerButtonSecondary
	PointerButtonMiddle
)

// PointerCursor names the native cursor shown for a portable pointer target.
type PointerCursor uint8

const (
	PointerCursorDefault PointerCursor = iota
	PointerCursorText
	PointerCursorMove
	PointerCursorCrosshair
	PointerCursorResizeHorizontal
	PointerCursorResizeVertical
	PointerCursorResizeNWSE
	PointerCursorResizeNESW
	PointerCursorHand
)

// PointerEvent uses logical client coordinates; positive scroll Y means upward motion.
type PointerEvent struct {
	Kind      PointerEventKind
	Position  Point
	Button    PointerButton
	Scroll    Point
	Modifiers KeyModifiers
}

func pointerPositionChanged(previous Point, current Point, known bool) bool {
	return !known || previous != current
}
