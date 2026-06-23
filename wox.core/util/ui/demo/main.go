//go:build windows && cgo

package main

import (
	"fmt"
	"os"
	"runtime"

	"wox/util/ui"
)

func main() {
	runtime.LockOSThread()

	theme := ui.DefaultTheme()
	renderer, err := ui.NewWindowsRenderer(800, 400, theme)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create renderer: %v\n", err)
		os.Exit(1)
	}
	defer renderer.Close()

	// State
	queryValue := ""
	selectedIdx := 0
	scrollOffset := float32(0)

	// List viewport dimensions (matches layout: y=72, height=400-72-20)
	const listTopY = 72.0
	const listViewportH = 308.0

	// ensureVisible scrolls the list so the selected item is visible.
	ensureVisible := func() {
		itemH := theme.ListItemHeight
		itemTop := float32(selectedIdx) * itemH
		itemBottom := itemTop + itemH

		if itemTop < scrollOffset {
			scrollOffset = itemTop
		} else if itemBottom > scrollOffset+listViewportH {
			scrollOffset = itemBottom - listViewportH
		}
	}

	// Build 100 demo items
	items := make([]ui.ListItem, 100)
	for i := range items {
		items[i] = ui.ListItem{
			Title:    fmt.Sprintf("Result Item %d — 结果项 %d", i+1, i+1),
			Subtitle: fmt.Sprintf("Plugin: system.app  •  Score: %d", 1000-i*7),
		}
	}

	// Event handler
	ui.SetEventHandler(func(ev ui.Event) {
		switch ev.Type {
		case ui.EventKeyPress:
			switch ev.Key {
			case ui.KeyEscape:
				renderer.Close()
				os.Exit(0)
			case ui.KeyDown:
				if selectedIdx < len(items)-1 {
					selectedIdx++
					ensureVisible()
				}
			case ui.KeyUp:
				if selectedIdx > 0 {
					selectedIdx--
					ensureVisible()
				}
			}
		case ui.EventTextInput:
			queryValue += ev.Text
		case ui.EventFocusLost:
			// Keep running but could hide window
		}
	})

	// Build the widget tree function
	engine := &ui.LayoutEngine{
		Theme:    theme,
		Measurer: renderer.TextMeasurer(),
	}

	buildTree := func() *ui.CommandList {
		root := ui.VBox{
			Padding: 16,
			Gap:     12,
			Children: []ui.Widget{
				ui.TextBox{
					ID:           "query",
					Placeholder:  "Type to search... (try Chinese IME)",
					FontSize:     16,
					FontColor:    ui.ColorTextPrimary,
					BgColor:      ui.RGBA(1, 1, 1, 0.06),
					CornerRadius: 8,
					CursorColor:  ui.ColorCursor,
					Value:        queryValue,
					Focused:      true,
				},
				ui.ListBox{
					ID:           "results",
					ItemHeight:   48,
					Items:        items,
					ScrollOffset: scrollOffset,
					Selected:     selectedIdx,
					SelectedColor: &ui.ColorSelected,
				},
			},
		}

		result := engine.Layout(root, 800, 400)
		return &result.Commands
	}

	renderer.Show()
	fmt.Println("Window shown. Press ESC to close, Up/Down to navigate.")

	// Run the message loop — blocks until window closes
	renderer.RunMessageLoop(buildTree)
}