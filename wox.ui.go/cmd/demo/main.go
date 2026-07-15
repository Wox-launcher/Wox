//go:build windows || darwin

package main

import (
	"fmt"
	"log"

	woxui "github.com/Wox-launcher/wox.ui.go"
)

func main() {
	err := woxui.Run(func() error {
		window, err := woxui.Open(woxui.WindowOptions{
			Title:   "Wox Go UI - Native GPU",
			Size:    woxui.Size{Width: 760, Height: 500},
			OnFrame: drawLauncher,
			OnFocus: func(event woxui.FocusEvent) {
				log.Printf("focus epoch=%d active=%t", event.Epoch, event.Active)
			},
		})
		if err != nil {
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

// drawLauncher exercises the first shared display-list primitives without introducing a widget tree.
func drawLauncher(displayList *woxui.DisplayList, frame woxui.FrameInfo) {
	width := frame.Size.Width
	height := frame.Size.Height
	foreground := woxui.Color{R: 244, G: 247, B: 250, A: 255}
	muted := woxui.Color{R: 166, G: 176, B: 190, A: 255}
	regular13 := woxui.TextStyle{Size: 13, Weight: woxui.FontWeightRegular}
	regular14 := woxui.TextStyle{Size: 14, Weight: woxui.FontWeightRegular}
	regular16 := woxui.TextStyle{Size: 16, Weight: woxui.FontWeightRegular}
	semibold16 := woxui.TextStyle{Size: 16, Weight: woxui.FontWeightSemibold}
	semibold17 := woxui.TextStyle{Size: 17, Weight: woxui.FontWeightSemibold}
	semibold18 := woxui.TextStyle{Size: 18, Weight: woxui.FontWeightSemibold}
	semibold22 := woxui.TextStyle{Size: 22, Weight: woxui.FontWeightSemibold}

	displayList.Clear(woxui.Color{})
	displayList.FillRoundedRect(woxui.Rect{X: 1, Y: 1, Width: width - 2, Height: height - 2}, 14, woxui.Color{R: 24, G: 29, B: 38, A: 242})
	displayList.FillRoundedRect(woxui.Rect{X: 20, Y: 18, Width: width - 40, Height: 52}, 9, woxui.Color{R: 56, G: 67, B: 82, A: 230})
	displayList.DrawText("WOX", woxui.Rect{X: 36, Y: 29, Width: 70, Height: 30}, semibold22, foreground)
	displayList.DrawText("Start typing to search", woxui.Rect{X: 105, Y: 32, Width: width - 220, Height: 24}, regular16, muted)
	displayList.DrawText("Alt + Space", woxui.Rect{X: width - 126, Y: 33, Width: 90, Height: 22}, regular13, muted)

	splitX := width * 0.58
	displayList.FillRect(woxui.Rect{X: splitX, Y: 88, Width: 1, Height: height - 142}, woxui.Color{R: 85, G: 96, B: 112, A: 150})

	results := []struct {
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

	for index, result := range results {
		y := float32(88 + index*66)
		if index == 2 {
			displayList.FillRoundedRect(woxui.Rect{X: 14, Y: y, Width: splitX - 28, Height: 58}, 9, woxui.Color{R: 43, G: 181, B: 168, A: 210})
		}
		displayList.FillRoundedRect(woxui.Rect{X: 28, Y: y + 13, Width: 32, Height: 32}, 8, result.color)
		displayList.DrawText(result.title, woxui.Rect{X: 76, Y: y + 7, Width: splitX - 94, Height: 28}, semibold17, foreground)
		subtitleColor := muted
		if index == 2 {
			subtitleColor = woxui.Color{R: 225, G: 251, B: 248, A: 255}
		}
		displayList.DrawText(result.subtitle, woxui.Rect{X: 76, Y: y + 33, Width: splitX - 94, Height: 20}, regular13, subtitleColor)
	}

	previewX := splitX + 22
	displayList.DrawText("Wox.Plugin.CodexUsage", woxui.Rect{X: previewX, Y: 96, Width: width - previewX - 20, Height: 30}, semibold18, foreground)
	displayList.DrawText("Native GPU preview", woxui.Rect{X: previewX, Y: 134, Width: width - previewX - 20, Height: 22}, regular14, muted)
	displayList.FillRoundedRect(woxui.Rect{X: previewX, Y: 174, Width: width - previewX - 22, Height: 88}, 10, woxui.Color{R: 42, G: 49, B: 61, A: 220})
	displayList.DrawText("原生 GPU 渲染预览", woxui.Rect{X: previewX + 16, Y: 192, Width: width - previewX - 54, Height: 28}, semibold16, foreground)
	displayList.DrawText("Native GPU backend", woxui.Rect{X: previewX + 16, Y: 226, Width: width - previewX - 54, Height: 22}, regular13, muted)

	displayList.FillRect(woxui.Rect{X: 1, Y: height - 44, Width: width - 2, Height: 1}, woxui.Color{R: 77, G: 88, B: 104, A: 150})
	displayList.DrawText("GPU DisplayList", woxui.Rect{X: 28, Y: height - 31, Width: 120, Height: 20}, regular13, muted)
	displayList.DrawText("Close window", woxui.Rect{X: width - 140, Y: height - 31, Width: 114, Height: 20}, regular13, muted)
}
