package dictationoverlay

import (
	"math"
	"sync"
	"time"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util/overlay"
)

const (
	dictationOverlayWidth         = 132
	dictationOverlayHeight        = 48
	dictationOverlayContentHeight = 24
	dictationOverlayCloseReserve  = 36
)

// Options configures the dictation overlay.
type Options struct {
	Window   overlay.WindowOptions
	Active   bool
	Closable bool
}

type dictationState struct {
	sync.Mutex
	active bool
	phase  float64
	stop   chan struct{}
}

var states = struct {
	sync.Mutex
	byID map[string]*dictationState
}{byID: map[string]*dictationState{}}

// Show creates or updates the runtime dictation HUD.
func Show(opts Options) {
	window := opts.Window
	if window.Width <= 0 {
		window.Width = dictationOverlayWidth
		if opts.Closable {
			window.Width += dictationOverlayCloseReserve
		}
	}
	if window.Height <= 0 {
		window.Height = dictationOverlayHeight
	}
	state := stateForID(window.ID)
	state.setActive(window.ID, opts.Active)

	overlay.ShowWindow(window, overlay.View{
		Kind: "dictation",
		Build: func(_ *woxui.Window, frame woxui.FrameInfo) woxwidget.Widget {
			state.Lock()
			active, phase := state.active, state.phase
			state.Unlock()
			children := []woxwidget.StackChild{{Child: waveform(dictationWaveformWidth(frame.Size.Width, opts.Closable), frame.Size.Height, active, phase)}}
			if opts.Closable {
				children = append(children, woxwidget.StackChild{Right: 8, Top: 10, AnchorRight: true, Child: woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{
					ID: "dictation-overlay-close", Label: "Close", Icon: woxcomponent.CloseGlyph(13, woxui.Color{R: 245, G: 245, B: 245, A: 255}),
					Width: 28, Height: 28, Radius: 6, HoverBackground: woxui.Color{R: 255, G: 255, B: 255, A: 28}, OnTap: func() { overlay.RequestClose(window.ID) },
				})})
			}
			return overlay.HUDSurface(frame.Size.Width, frame.Size.Height, 12, window.LightAppearance, woxwidget.Stack{Width: frame.Size.Width, Height: frame.Size.Height, Children: children})
		},
		OnDispose: func() { releaseState(window.ID) },
	})
}

// dictationWaveformWidth centers the bars before the close-button region.
func dictationWaveformWidth(width float32, closable bool) float32 {
	if closable {
		return max(float32(1), width-dictationOverlayCloseReserve)
	}
	return width
}

// UpdateActive updates the voice activity animation state without rebuilding the window.
func UpdateActive(id string, active bool) {
	states.Lock()
	state := states.byID[id]
	states.Unlock()
	if state != nil {
		state.setActive(id, active)
	}
}

// Release drops dictation-only state while leaving a replacement overlay window alone.
func Release(id string) {
	releaseState(id)
}

// Close closes the shared runtime overlay.
func Close(id string) {
	overlay.Close(id)
}

func stateForID(id string) *dictationState {
	states.Lock()
	defer states.Unlock()
	if state := states.byID[id]; state != nil {
		return state
	}
	state := &dictationState{}
	states.byID[id] = state
	return state
}

// setActive owns the single animation ticker associated with one dictation HUD.
func (state *dictationState) setActive(id string, active bool) {
	state.Lock()
	if state.active == active {
		state.Unlock()
		overlay.Invalidate(id)
		return
	}
	state.active = active
	if active {
		state.stop = make(chan struct{})
		stop := state.stop
		state.Unlock()
		go func() {
			ticker := time.NewTicker(33 * time.Millisecond)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					state.Lock()
					state.phase++
					state.Unlock()
					overlay.Invalidate(id)
				case <-stop:
					return
				}
			}
		}()
		return
	}
	stop := state.stop
	state.stop = nil
	state.Unlock()
	if stop != nil {
		close(stop)
	}
	overlay.Invalidate(id)
}

func releaseState(id string) {
	states.Lock()
	state := states.byID[id]
	delete(states.byID, id)
	states.Unlock()
	if state == nil {
		return
	}
	state.Lock()
	stop := state.stop
	state.stop = nil
	state.active = false
	state.Unlock()
	if stop != nil {
		close(stop)
	}
}

// waveform paints the compact idle or animated seven-bar voice indicator.
func waveform(width, height float32, active bool, phase float64) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(list *woxui.DisplayList, bounds woxui.Rect) {
		const barWidth, gap = float32(4), float32(5)
		idle := [...]float32{.32, .46, .36, .56, .36, .46, .32}
		totalWidth := float32(len(idle))*barWidth + float32(len(idle)-1)*gap
		x := (bounds.Width - totalWidth) / 2
		for i, scale := range idle {
			if active {
				scale = .28 + .72*float32(.5+.5*math.Sin(phase*.32+float64(i)*.85))
			}
			barHeight := dictationOverlayContentHeight * scale
			list.FillRoundedRect(woxui.Rect{X: x, Y: (bounds.Height - barHeight) / 2, Width: barWidth, Height: barHeight}, barWidth/2, woxui.Color{R: 245, G: 245, B: 245, A: 255})
			x += barWidth + gap
		}
	}}
}
