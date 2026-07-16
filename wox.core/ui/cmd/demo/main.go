//go:build windows || darwin || linux

package main

import (
	"fmt"
	"log"
	"os"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

var results = []struct {
	title    string
	subtitle string
	color    woxui.Color
}{
	{title: "Wox", subtitle: "C:\\dev\\Wox", color: woxui.Color{R: 238, G: 241, B: 246, A: 255}},
	{title: "Wox.Plugin.Projects", subtitle: "Open recent projects", color: woxui.Color{R: 255, G: 119, B: 81, A: 255}},
	{title: "Wox.Plugin.CodexUsage", subtitle: "Inspect local Codex usage", color: woxui.Color{R: 61, G: 205, B: 175, A: 255}},
	{title: "Wox.Plugin.ColorPicker", subtitle: "Pick a color from the screen", color: woxui.Color{R: 177, G: 104, B: 255, A: 255}},
	{title: "Wox.Plugin.Clipboard", subtitle: "Search clipboard history", color: woxui.Color{R: 66, G: 153, B: 225, A: 255}},
}

func main() {
	err := woxui.Run(func() error {
		state := &launcherState{selected: 2, webView: os.Getenv("WOX_UI_DEMO_WEBVIEW") == "1"}
		host := woxwidget.NewHost(state.build)
		window, err := woxui.Open(woxui.WindowOptions{
			Title:       "Wox Go UI - Native GPU",
			Size:        woxui.Size{Width: 760, Height: 500},
			OnFrame:     host.Frame,
			OnPointer:   host.Pointer,
			OnKey:       state.onKey,
			OnTextInput: state.onTextInput,
			OnFocus: func(event woxui.FocusEvent) {
				log.Printf("focus epoch=%d active=%t", event.Epoch, event.Active)
			},
		})
		if err != nil {
			return err
		}
		state.window = window
		host.Attach(window)
		if err := state.updateTextInputState(); err != nil {
			return err
		}
		epoch, err := window.Show()
		if err != nil {
			return err
		}
		fmt.Printf("window shown with focus epoch %d\n", epoch)
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}
}

type launcherState struct {
	window       *woxui.Window
	query        string
	composition  string
	selected     int
	webView      bool
	webViewError string
}

// onKey handles non-text launcher navigation while leaving printable keys to the platform IME.
func (s *launcherState) onKey(event woxui.KeyEvent) bool {
	if !event.Down || event.Composing {
		return false
	}
	switch event.Key {
	case woxui.KeyBackspace:
		runes := []rune(s.query)
		if len(runes) > 0 {
			s.query = string(runes[:len(runes)-1])
			_ = s.updateTextInputState()
			_ = s.window.Invalidate()
		}
		return true
	case woxui.KeyArrowUp:
		s.selectResult(s.selected - 1)
		return true
	case woxui.KeyArrowDown:
		s.selectResult(s.selected + 1)
		return true
	case woxui.KeyEnter:
		s.activateResult()
		return true
	case woxui.KeyEscape:
		log.Printf("escape returned from the active focus surface")
		_ = s.window.Hide()
		return true
	default:
		return false
	}
}

// onTextInput applies committed text separately from the current IME composition.
func (s *launcherState) onTextInput(event woxui.TextInputEvent) {
	switch event.Kind {
	case woxui.TextInputCommit:
		s.query += event.Text
		s.composition = ""
	case woxui.TextInputCompose:
		s.composition = event.Text
	}
	_ = s.updateTextInputState()
	_ = s.window.Invalidate()
}

func (s *launcherState) selectResult(index int) {
	s.selected = max(0, min(len(results)-1, index))
	_ = s.window.Invalidate()
}

func (s *launcherState) activateResult() {
	log.Printf("activate result=%d query=%q", s.selected, s.query)
}

// updateTextInputState positions native candidate UI after the shaped query text.
func (s *launcherState) updateTextInputState() error {
	logo, err := s.window.MeasureText("WOX", woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold})
	if err != nil {
		return err
	}
	query, err := s.window.MeasureText(s.query+s.composition, woxui.TextStyle{Size: 16, Weight: woxui.FontWeightRegular})
	if err != nil {
		return err
	}
	return s.window.SetTextInputState(woxui.TextInputState{
		Enabled:    true,
		CursorRect: woxui.Rect{X: 48 + logo.Size.Width + query.Size.Width, Y: 29, Width: 2, Height: 24},
	})
}

// build creates the query widget tree for the current frame and state.
func (s *launcherState) build(frame woxui.FrameInfo) woxwidget.Widget {
	width := frame.Size.Width
	height := frame.Size.Height
	headerHeight := float32(88)
	footerHeight := float32(44)
	contentHeight := max(0, height-headerHeight-footerHeight)
	splitX := width * 0.58
	return woxwidget.Container{
		Width:  width,
		Height: height,
		Color:  woxui.Color{R: 24, G: 29, B: 38, A: 242},
		Radius: 14,
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
			s.buildHeader(width, headerHeight),
			woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
				s.buildResults(splitX, contentHeight),
				woxwidget.Painter{Width: 1, Height: contentHeight, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
					displayList.FillRect(bounds, woxui.Color{R: 85, G: 96, B: 112, A: 150})
				}},
				s.buildPreview(width-splitX-1, contentHeight),
			}},
			s.buildFooter(width, footerHeight),
		}},
	}
}

func (s *launcherState) buildHeader(width, height float32) woxwidget.Widget {
	foreground := woxui.Color{R: 244, G: 247, B: 250, A: 255}
	muted := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 20, Top: 18, Right: 20, Bottom: 18},
		Child: woxwidget.Container{
			Width: width - 40, Height: 52, Radius: 9, Color: woxui.Color{R: 56, G: 67, B: 82, A: 230},
			Padding: woxwidget.Insets{Left: 16, Top: 11, Right: 16, Bottom: 11},
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "WOX", Style: woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}, Color: foreground},
				s.buildQuery(max(180, width-300), 30),
				woxwidget.Text{Value: "Alt + Space", Style: woxui.TextStyle{Size: 13}, Color: muted},
			}},
		},
	}
}

func (s *launcherState) buildQuery(width, height float32) woxwidget.Widget {
	return woxwidget.Painter{Width: width, Height: height, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		foreground := woxui.Color{R: 244, G: 247, B: 250, A: 255}
		muted := woxui.Color{R: 166, G: 176, B: 190, A: 255}
		style := woxui.TextStyle{Size: 16}
		value := s.query + s.composition
		color := foreground
		if value == "" {
			value = "Start typing to search"
			color = muted
		}
		displayList.DrawText(value, bounds, style, color)
		total, _ := s.window.MeasureText(s.query+s.composition, style)
		displayList.FillRect(woxui.Rect{X: bounds.X + total.Size.Width, Y: bounds.Y, Width: 1, Height: 22}, foreground)
		if s.composition != "" {
			committed, _ := s.window.MeasureText(s.query, style)
			displayList.FillRect(woxui.Rect{X: bounds.X + committed.Size.Width, Y: bounds.Y + 23, Width: total.Size.Width - committed.Size.Width, Height: 1}, foreground)
		}
	}}
}

func (s *launcherState) buildResults(width, height float32) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(results))
	for index, result := range results {
		index := index
		result := result
		selected := index == s.selected
		background := woxui.Color{}
		subtitle := woxui.Color{R: 166, G: 176, B: 190, A: 255}
		if selected {
			background = woxui.Color{R: 43, G: 181, B: 168, A: 210}
			subtitle = woxui.Color{R: 225, G: 251, B: 248, A: 255}
		}
		rows = append(rows, woxwidget.Gesture{
			ID: fmt.Sprintf("result-%d", index),
			OnHover: func(inside bool) {
				if inside {
					s.selected = index
				}
			},
			OnTap: s.activateResult,
			OnScroll: func(delta woxui.Point) {
				if delta.Y > 0 {
					s.selected = max(0, s.selected-1)
				} else if delta.Y < 0 {
					s.selected = min(len(results)-1, s.selected+1)
				}
			},
			Child: woxwidget.Container{
				Width: width - 28, Height: 58, Radius: 9, Color: background,
				Padding: woxwidget.Insets{Left: 14, Top: 7, Right: 14, Bottom: 7},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 16, Children: []woxwidget.Widget{
					woxwidget.Container{Width: 32, Height: 32, Radius: 8, Color: result.color},
					woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{
						woxwidget.Text{Value: result.title, Style: woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{R: 244, G: 247, B: 250, A: 255}},
						woxwidget.Text{Value: result.subtitle, Style: woxui.TextStyle{Size: 13}, Color: subtitle},
					}},
				}},
			},
		})
	}
	return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 14, Right: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows}}
}

func (s *launcherState) buildPreview(width, height float32) woxwidget.Widget {
	if s.webView {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 22, Top: 8, Right: 22, Bottom: 8}, Child: woxwidget.Painter{Width: width - 44, Height: height - 16, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
			displayList.FillRoundedRect(bounds, 10, woxui.Color{R: 42, G: 49, B: 61, A: 255})
			if s.webViewError != "" {
				displayList.DrawText(s.webViewError, bounds, woxui.TextStyle{Size: 13}, woxui.Color{R: 232, G: 95, B: 95, A: 255})
				return
			}
			html := "<!doctype html><html><body><h2>Wox system WebView</h2><p>WebView2 / WKWebView / WebKitGTK shares this host rectangle.</p><input autofocus placeholder='Type here to test browser focus'><p>Press Escape after the page leaves it unhandled.</p></body></html>"
			css := "html{color-scheme:dark}body{margin:24px;background:#202631;color:#f4f7fa;font:16px system-ui}input{box-sizing:border-box;width:100%;padding:10px;border:1px solid #64748b;border-radius:8px;background:#111827;color:white}"
			if err := s.window.ShowWebView(woxui.WebViewContent{HTML: html, InjectCSS: css, CacheDisabled: true}, bounds); err != nil {
				s.webViewError = err.Error()
				_ = s.window.Invalidate()
			}
		}}}
	}
	_ = s.window.HideWebView()
	selected := results[s.selected]
	foreground := woxui.Color{R: 244, G: 247, B: 250, A: 255}
	muted := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 22, Top: 8, Right: 22},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Text{Value: selected.title, Style: woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}, Color: foreground},
			woxwidget.Text{Value: "Native GPU preview", Style: woxui.TextStyle{Size: 14}, Color: muted},
			woxwidget.Container{
				Width: width - 44, Height: 88, Radius: 10, Color: woxui.Color{R: 42, G: 49, B: 61, A: 220},
				Padding: woxwidget.Insets{Left: 16, Top: 14, Right: 16, Bottom: 14},
				Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 6, Children: []woxwidget.Widget{
					woxwidget.Text{Value: "原生 GPU 渲染预览", Style: woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}, Color: foreground},
					woxwidget.Text{Value: selected.subtitle, Style: woxui.TextStyle{Size: 13}, Color: muted},
				}},
			},
		}},
	}
}

func (s *launcherState) buildFooter(width, height float32) woxwidget.Widget {
	muted := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	return woxwidget.Container{
		Width: width, Height: height, Color: woxui.Color{R: 20, G: 24, B: 31, A: 180},
		Padding: woxwidget.Insets{Left: 28, Top: 13, Right: 28, Bottom: 10},
		Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Children: []woxwidget.Widget{
			woxwidget.Text{Value: "GPU DisplayList + Go widgets", Style: woxui.TextStyle{Size: 13}, Color: muted},
			woxwidget.Painter{Width: max(0, width-360), Height: 1},
			woxwidget.Text{Value: "Click a result", Style: woxui.TextStyle{Size: 13}, Color: muted},
		}},
	}
}
