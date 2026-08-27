package screenshot

import woxui "wox/ui/runtime"

type (
	Color           = woxui.Color
	DisplayList     = woxui.DisplayList
	FrameInfo       = woxui.FrameInfo
	Image           = woxui.Image
	Key             = woxui.Key
	KeyEvent        = woxui.KeyEvent
	KeyModifiers    = woxui.KeyModifiers
	ManagedWindow   = woxui.ManagedWindow
	Point           = woxui.Point
	PointerEvent    = woxui.PointerEvent
	PointerCursor   = woxui.PointerCursor
	Rect            = woxui.Rect
	SaveFileOptions = woxui.SaveFileOptions
	Size            = woxui.Size
	TextInputEvent  = woxui.TextInputEvent
	TextInputState  = woxui.TextInputState
	TextStyle       = woxui.TextStyle
	Window          = woxui.Window
	WindowID        = woxui.WindowID
	WindowLifecycle = woxui.WindowLifecycle
	WindowManager   = woxui.WindowManager
	WindowOptions   = woxui.WindowOptions
)

const (
	FontWeightSemibold            = woxui.FontWeightSemibold
	KeyBackspace                  = woxui.KeyBackspace
	KeyDelete                     = woxui.KeyDelete
	KeyEnter                      = woxui.KeyEnter
	KeyEscape                     = woxui.KeyEscape
	KeyArrowLeft                  = woxui.KeyArrowLeft
	KeyArrowUp                    = woxui.KeyArrowUp
	KeyArrowRight                 = woxui.KeyArrowRight
	KeyArrowDown                  = woxui.KeyArrowDown
	KeyHome                       = woxui.KeyHome
	KeyEnd                        = woxui.KeyEnd
	KeyModifierShift              = woxui.KeyModifierShift
	KeyModifierControl            = woxui.KeyModifierControl
	KeyModifierAlt                = woxui.KeyModifierAlt
	KeyModifierMeta               = woxui.KeyModifierMeta
	PointerButtonPrimary          = woxui.PointerButtonPrimary
	PointerCursorCrosshair        = woxui.PointerCursorCrosshair
	PointerCursorDefault          = woxui.PointerCursorDefault
	PointerCursorHand             = woxui.PointerCursorHand
	PointerCursorMove             = woxui.PointerCursorMove
	PointerCursorResizeHorizontal = woxui.PointerCursorResizeHorizontal
	PointerCursorResizeNESW       = woxui.PointerCursorResizeNESW
	PointerCursorResizeNWSE       = woxui.PointerCursorResizeNWSE
	PointerCursorResizeVertical   = woxui.PointerCursorResizeVertical
	PointerCursorText             = woxui.PointerCursorText
	PointerDown                   = woxui.PointerDown
	PointerLeave                  = woxui.PointerLeave
	PointerMove                   = woxui.PointerMove
	PointerUp                     = woxui.PointerUp
	TextInputCommit               = woxui.TextInputCommit
	TextInputCompose              = woxui.TextInputCompose
	WindowLifecyclePresenting     = woxui.WindowLifecyclePresenting
	WindowLifecycleVisible        = woxui.WindowLifecycleVisible
	WindowRoleScreenshot          = woxui.WindowRoleScreenshot
	WindowRoleUtility             = woxui.WindowRoleUtility
)

var (
	Call                   = woxui.Call
	ErrPlatformUnsupported = woxui.ErrPlatformUnsupported
	NewImage               = woxui.NewImage
	NewImageFromPackedRGBA = woxui.NewImageFromPackedRGBA
	NewWindowManager       = woxui.NewWindowManager
)
