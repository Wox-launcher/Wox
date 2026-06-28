package ui

// EventType identifies an input event from the native layer.
type EventType int32

const (
	EventKeyPress EventType = iota
	EventKeyRelease
	EventTextInput    // IME final text or direct character
	EventIMECompose   // IME composition string update
	EventClick
	EventScroll
	EventFocusLost
	EventResize
)

// Key represents a keyboard key.
type Key int32

const (
	KeyUnknown Key = iota
	KeyEscape
	KeyEnter
	KeyBackspace
	KeyTab
	KeySpace
	KeyUp
	KeyDown
	KeyLeft
	KeyRight
	KeyHome
	KeyEnd
	KeyPageUp
	KeyPageDown
	KeyDelete
	KeyF1
	KeyF2
	KeyF3
	KeyF4
	KeyF5
	KeyF6
	KeyF7
	KeyF8
	KeyF9
	KeyF10
	KeyF11
	KeyF12
	KeyA
	KeyB
	KeyC
	KeyD
	KeyE
	KeyF
	KeyG
	KeyH
	KeyI
	KeyJ
	KeyK
	KeyL
	KeyM
	KeyN
	KeyO
	KeyP
	KeyQ
	KeyR
	KeyS
	KeyT
	KeyU
	KeyV
	KeyW
	KeyX
	KeyY
	KeyZ
	Key0
	Key1
	Key2
	Key3
	Key4
	Key5
	Key6
	Key7
	Key8
	Key9
)

// Modifiers holds keyboard modifier state.
// Values match the native ABI defined in ui_native.h (ModShift=1, etc.)
// so Go and C sides agree without translation.
type Modifiers int32

const (
	ModNone    Modifiers = 0
	ModShift   Modifiers = 1
	ModControl Modifiers = 2
	ModAlt     Modifiers = 4
	ModSuper   Modifiers = 8
)

// Event is an input event from the native layer.
type Event struct {
	Type EventType

	// Key press/release
	Key  Key
	Mods Modifiers

	// Text input / IME composition
	Text           string // final committed text
	ComposeText    string // current IME composition string (may be empty)
	ComposeCursor  int    // cursor position within ComposeText (byte offset)

	// Click
	X, Y float32

	// Scroll
	DeltaY float32

	// Resize
	Width  int32
	Height int32
}

// EventCallback is invoked by the native layer for each input event.
// The Go side updates widget state and triggers a redraw.
type EventCallback func(Event)