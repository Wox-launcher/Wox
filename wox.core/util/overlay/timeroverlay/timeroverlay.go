package timeroverlay

import (
	"sync"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

const (
	defaultCountdownFontSize = 22
	defaultNoteFontSize      = 11
	defaultTimerMaxWidth     = 316
	timerHorizontalPadding   = float32(24)
	timerTextSlack           = float32(8)
	timerCloseReserve        = float32(48)
)

// Options configures a runtime countdown HUD.
type Options struct {
	Window overlay.WindowOptions

	Countdown string
	Note      string
	Closable  bool

	CountdownFontSize float64
	NoteFontSize      float64
}

type timerState struct {
	sync.Mutex
	hovered bool
}

var states = struct {
	sync.Mutex
	byID map[string]*timerState
}{byID: map[string]*timerState{}}

// Show displays or updates a timer overlay.
func Show(opts Options) {
	if opts.CountdownFontSize <= 0 {
		opts.CountdownFontSize = defaultCountdownFontSize
	}
	if opts.NoteFontSize <= 0 {
		opts.NoteFontSize = defaultNoteFontSize
	}
	state := stateForID(opts.Window.ID)

	overlay.ShowWindow(opts.Window, overlay.View{
		Kind: "timer",
		Measure: func(window *woxui.Window, _ woxui.Rect) woxui.Size {
			state.Lock()
			hovered := state.hovered
			state.Unlock()
			countdown, _ := window.MeasureText(opts.Countdown, woxui.TextStyle{Size: float32(opts.CountdownFontSize), Weight: woxui.FontWeightSemibold})
			note := woxui.TextMetrics{}
			if hovered && opts.Note != "" {
				note, _ = window.MeasureText(opts.Note, woxui.TextStyle{Size: float32(opts.NoteFontSize)})
			}
			return timerSize(countdown.Size, note.Size, hovered, opts.Closable)
		},
		Build: func(_ *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget {
			state.Lock()
			hovered := state.hovered
			state.Unlock()
			showNote := hovered && opts.Note != ""
			showClose := hovered && opts.Closable
			textWidth := frame.Size.Width - timerHorizontalPadding*2
			if showClose {
				textWidth -= timerCloseReserve
			}
			children := []woxwidget.Widget{woxwidget.TextBlock{
				Value: opts.Countdown, Width: textWidth, Height: float32(opts.CountdownFontSize) + 8, MaxLines: 1, Centered: true,
				Style: woxui.TextStyle{Size: float32(opts.CountdownFontSize), Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 245, G: 245, B: 245, A: 255},
			}}
			if showNote {
				children = append(children, woxwidget.TextBlock{
					Value: opts.Note, Width: textWidth, Height: float32(opts.NoteFontSize) + 5, MaxLines: 1, Centered: true,
					Style: woxui.TextStyle{Size: float32(opts.NoteFontSize)}, Color: woxui.Color{R: 200, G: 200, B: 204, A: 255},
				})
			}
			stack := []woxwidget.StackChild{{Child: woxwidget.Align{Width: frame.Size.Width, Height: frame.Size.Height, Horizontal: .5, Vertical: .5, Child: woxwidget.Flex{
				Axis: woxwidget.Vertical, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
			}}}}
			if showClose {
				stack = append(stack, woxwidget.StackChild{Right: 8, Top: 8, AnchorRight: true, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
					ID: "timer-overlay-close", Label: "Close", Icon: woxcomponent.CloseGlyph(12, woxui.Color{R: 245, G: 245, B: 245, A: 255}),
					Width: 24, Height: 24, Radius: 6, HoverBackground: woxui.Color{R: 255, G: 255, B: 255, A: 28}, OnTap: func() { overlay.RequestClose(opts.Window.ID) },
				})})
			}
			return overlay.HUDSurface(frame.Size.Width, frame.Size.Height, 12, opts.Window.LightAppearance, woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: stack})
		},
		OnPointer: func(event woxui.PointerEvent) {
			state.Lock()
			hovered := nextTimerHovered(state.hovered, event.Kind)
			changed := state.hovered != hovered
			state.hovered = hovered
			state.Unlock()
			if changed {
				overlay.Relayout(opts.Window.ID)
			}
		},
		OnDispose: func() { releaseState(opts.Window.ID) },
	})
}

// nextTimerHovered ignores queued motion events that can arrive after a macOS tracking-area exit.
func nextTimerHovered(current bool, kind woxui.PointerEventKind) bool {
	switch kind {
	case woxui.PointerEnter:
		return true
	case woxui.PointerLeave:
		return false
	default:
		return current
	}
}

// timerSize keeps the compact countdown fixed until hover reveals details and close chrome.
func timerSize(countdown, note woxui.Size, expanded, closable bool) woxui.Size {
	width := max(countdown.Width, note.Width) + timerHorizontalPadding*2 + timerTextSlack
	if expanded && closable {
		width += timerCloseReserve
	}
	if expanded {
		width = max(width, 132)
	}
	width = min(width, defaultTimerMaxWidth)
	height := countdown.Height + 24
	if expanded && note.Height > 0 {
		height += note.Height + 8
	}
	return woxui.Size{Width: max(width, 52), Height: max(height, 48)}
}

func stateForID(id string) *timerState {
	states.Lock()
	defer states.Unlock()
	if state := states.byID[id]; state != nil {
		return state
	}
	state := &timerState{}
	states.byID[id] = state
	return state
}

func releaseState(id string) {
	states.Lock()
	delete(states.byID, id)
	states.Unlock()
}
