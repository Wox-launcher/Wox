package window

import (
	input "wox/ui/runtime/internal/input"
)

// Hotkey is a normal key combination shared by launcher and embedded previews.
type Hotkey = input.Hotkey

// ParseHotkey adapts the shared shortcut parser to UI event names and modifier bits.
func ParseHotkey(value string) (Hotkey, bool) { return input.ParseHotkey(value) }

// Key names the portable semantic keys needed by widgets and shortcuts.
// Printable keys use their lowercase Unicode text, such as Key("a") or Key("1").
type Key = input.Key

const (
	KeyUnknown    = input.KeyUnknown
	KeyBackspace  = input.KeyBackspace
	KeyTab        = input.KeyTab
	KeyEnter      = input.KeyEnter
	KeyEscape     = input.KeyEscape
	KeySpace      = input.KeySpace
	KeyPageUp     = input.KeyPageUp
	KeyPageDown   = input.KeyPageDown
	KeyEnd        = input.KeyEnd
	KeyHome       = input.KeyHome
	KeyArrowLeft  = input.KeyArrowLeft
	KeyArrowUp    = input.KeyArrowUp
	KeyArrowRight = input.KeyArrowRight
	KeyArrowDown  = input.KeyArrowDown
	KeyDelete     = input.KeyDelete
	KeyAlt        = input.KeyAlt
	KeyMeta       = input.KeyMeta
)

// KeyModifiers is a platform-neutral modifier bit set.
type KeyModifiers = input.KeyModifiers

const (
	KeyModifierShift   = input.KeyModifierShift
	KeyModifierControl = input.KeyModifierControl
	KeyModifierAlt     = input.KeyModifierAlt
	KeyModifierMeta    = input.KeyModifierMeta
)

// KeyEvent reports a semantic key transition before text input processing.
type KeyEvent = input.KeyEvent

// TextInputEventKind distinguishes committed text from an in-progress IME composition.
type TextInputEventKind = input.TextInputEventKind

const (
	TextInputCommit  = input.TextInputCommit
	TextInputCompose = input.TextInputCompose
)

// TextInputEvent carries UTF-8 text from the platform input method.
// An empty composition clears the current marked text.
type TextInputEvent = input.TextInputEvent

// TextInputState tells the platform whether an editor is active and where IME UI should appear.
type TextInputState = input.TextInputState

// PointerEventKind identifies one mouse or trackpad transition.
type PointerEventKind = input.PointerEventKind

const (
	PointerMove   = input.PointerMove
	PointerEnter  = input.PointerEnter
	PointerLeave  = input.PointerLeave
	PointerDown   = input.PointerDown
	PointerUp     = input.PointerUp
	PointerScroll = input.PointerScroll
)

// PointerButton names the button involved in a pointer transition.
type PointerButton = input.PointerButton

const (
	PointerButtonNone      = input.PointerButtonNone
	PointerButtonPrimary   = input.PointerButtonPrimary
	PointerButtonSecondary = input.PointerButtonSecondary
	PointerButtonMiddle    = input.PointerButtonMiddle
)

// PointerCursor names the native cursor shown for a portable pointer target.
type PointerCursor = input.PointerCursor

const (
	PointerCursorDefault          = input.PointerCursorDefault
	PointerCursorText             = input.PointerCursorText
	PointerCursorMove             = input.PointerCursorMove
	PointerCursorCrosshair        = input.PointerCursorCrosshair
	PointerCursorResizeHorizontal = input.PointerCursorResizeHorizontal
	PointerCursorResizeVertical   = input.PointerCursorResizeVertical
	PointerCursorResizeNWSE       = input.PointerCursorResizeNWSE
	PointerCursorResizeNESW       = input.PointerCursorResizeNESW
	PointerCursorHand             = input.PointerCursorHand
	PointerCursorHidden           = input.PointerCursorHidden
)

// PointerEvent uses logical client coordinates; positive scroll Y means upward motion.
type PointerEvent = input.PointerEvent

// TextSelection stores anchor and focus as rune offsets so UTF-8 editing stays deterministic.
// Movement and deletion snap those offsets onto Unicode grapheme boundaries.
type TextSelection = input.TextSelection

// TextEditingState is an immutable snapshot of committed text, selection, and marked text.
type TextEditingState = input.TextEditingState

// TextEditor applies portable key and IME events to one UTF-8 value.
type TextEditor = input.TextEditor

// NewTextEditor creates an editor with its caret at the end of text.
func NewTextEditor(text string) *TextEditor { return input.NewTextEditor(text) }

// FilterSingleLineNewlines strips carriage returns and newlines for single-line editors.
func FilterSingleLineNewlines(text string) string { return input.FilterSingleLineNewlines(text) }

// MaskProtectedText replaces each user-perceived character with a bullet for password display.
func MaskProtectedText(text string) string { return input.MaskProtectedText(text) }

// MapSelectionToProtectedDisplay remaps rune selection offsets onto the bullet-masked display.
func MapSelectionToProtectedDisplay(text string, selection TextSelection) TextSelection {
	return input.MapSelectionToProtectedDisplay(text, selection)
}

// MapProtectedDisplayOffsetToRune maps a bullet-display grapheme index back onto the real text.
func MapProtectedDisplayOffsetToRune(text string, displayOffset int) int {
	return input.MapProtectedDisplayOffsetToRune(text, displayOffset)
}

// GraphemeSpan is one user-perceived character with absolute rune offsets.
type GraphemeSpan = input.GraphemeSpan

// GraphemeSpans returns contiguous grapheme spans for text.
func GraphemeSpans(text string) []GraphemeSpan { return input.GraphemeSpans(text) }
