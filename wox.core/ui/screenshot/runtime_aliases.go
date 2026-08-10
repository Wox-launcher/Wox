package screenshot

import woxui "wox/ui/runtime"

type (
	Color          = woxui.Color
	DisplayList    = woxui.DisplayList
	FrameInfo      = woxui.FrameInfo
	Image          = woxui.Image
	KeyEvent       = woxui.KeyEvent
	ManagedWindow  = woxui.ManagedWindow
	Point          = woxui.Point
	PointerEvent   = woxui.PointerEvent
	Rect           = woxui.Rect
	Size           = woxui.Size
	TextInputEvent = woxui.TextInputEvent
	TextInputState = woxui.TextInputState
	TextStyle      = woxui.TextStyle
	Window         = woxui.Window
	WindowID       = woxui.WindowID
	WindowManager  = woxui.WindowManager
	WindowOptions  = woxui.WindowOptions
)

const (
	FontWeightSemibold   = woxui.FontWeightSemibold
	KeyBackspace         = woxui.KeyBackspace
	KeyDelete            = woxui.KeyDelete
	KeyEnter             = woxui.KeyEnter
	KeyEscape            = woxui.KeyEscape
	PointerButtonPrimary = woxui.PointerButtonPrimary
	PointerDown          = woxui.PointerDown
	PointerMove          = woxui.PointerMove
	PointerUp            = woxui.PointerUp
	TextInputCommit      = woxui.TextInputCommit
	TextInputCompose     = woxui.TextInputCompose
	WindowRoleScreenshot = woxui.WindowRoleScreenshot
)

var (
	Call                   = woxui.Call
	ErrPlatformUnsupported = woxui.ErrPlatformUnsupported
	NewImage               = woxui.NewImage
	NewImageFromPackedRGBA = woxui.NewImageFromPackedRGBA
	NewWindowManager       = woxui.NewWindowManager
)
