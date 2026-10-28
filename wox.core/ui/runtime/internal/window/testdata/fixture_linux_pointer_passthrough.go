//go:build linux

// This executable exercises the real native backend from the Hyprland fixture.
package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"time"

	window "wox/ui/runtime/internal/window"
)

type counters struct {
	Enters int    `json:"enters"`
	Error  string `json:"error,omitempty"`
}

// runWindow keeps all counters and native operations on the owning UI thread.
func runWindow() error {
	var counts counters
	// The screenshot window remains mapped as a toolbar when its border opens.
	toolbar, err := window.Open(window.WindowOptions{
		Title: "Wox toolbar input regression", Size: window.Size{Width: 200, Height: 60},
		Role: window.WindowRoleScreenshot,
	})
	if err != nil {
		return err
	}
	if err := toolbar.SetBounds(window.Rect{X: 320, Y: 40, Width: 200, Height: 60}); err != nil {
		return err
	}
	if _, err := toolbar.Show(); err != nil {
		return err
	}
	w, err := window.Open(window.WindowOptions{
		Title: "Wox input regression", Size: window.Size{Width: 240, Height: 160},
		Role: window.WindowRoleScreenshot, Nonactivating: true,
		OnFrame: func(list *window.DisplayList, frame window.FrameInfo) {
			list.FillRect(window.Rect{Width: frame.Size.Width, Height: frame.Size.Height}, window.Color{R: 30, G: 80, B: 180, A: 120})
		},
		OnPointer: func(event window.PointerEvent) {
			if event.Kind == window.PointerEnter {
				counts.Enters++
			}
		},
	})
	if err != nil {
		return err
	}
	if err := w.SetBounds(window.Rect{X: 40, Y: 40, Width: 240, Height: 160}); err != nil {
		return err
	}
	if err := w.SetPointerPassthrough(true); err != nil {
		return err
	}
	if _, err := w.Show(); err != nil {
		return err
	}
	go func() {
		// Wait for initial map/configure events before accepting commands.
		time.Sleep(500 * time.Millisecond)
		report := func() { _ = json.NewEncoder(os.Stdout).Encode(counts) }
		if err := window.Call(report); err != nil {
			return
		}
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			command := scanner.Text()
			if err := window.Call(func() {
				var err error
				switch command {
				case "release-keyboard":
					err = toolbar.SetExclusiveKeyboard(false)
				case "capture":
					err = w.SetPointerPassthrough(false)
				case "pass":
					err = w.SetPointerPassthrough(true)
				case "draw":
					err = w.Invalidate()
				case "quit":
					err = w.Close()
					if closeErr := toolbar.Close(); err == nil {
						err = closeErr
					}
				}
				counts.Error = ""
				if err != nil {
					counts.Error = err.Error()
				}
				report()
			}); err != nil || command == "quit" {
				return
			}
		}
		_ = window.Call(func() { _ = w.Close(); _ = toolbar.Close() })
	}()
	return nil
}

func main() {
	if err := window.Run(runWindow); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
