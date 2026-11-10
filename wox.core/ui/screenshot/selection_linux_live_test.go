//go:build linux

package screenshot

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/screen"
)

type linuxLiveMultiOutputCapture struct{ outputs []linuxDesktopOutput }

func (capture linuxLiveMultiOutputCapture) capture() (image.Image, error) {
	return capture.outputs[0].source, nil
}
func (capture linuxLiveMultiOutputCapture) logicalBounds() Rect { return capture.outputs[0].bounds }
func (capture linuxLiveMultiOutputCapture) close()              {}
func (capture linuxLiveMultiOutputCapture) captureOutputs() ([]linuxDesktopOutput, error) {
	return capture.outputs, nil
}

// TestLinuxLiveMultiOutputSelection checks real masks, handoff, cleanup, and native-resolution export on every Plasma output.
// It uses solid test rasters, never desktop screenshots, and does not move the system pointer or change the clipboard.
func TestLinuxLiveMultiOutputSelection(t *testing.T) {
	if os.Getenv("WOX_LIVE_KWIN_SELECTION") != "1" {
		t.Skip("set WOX_LIVE_KWIN_SELECTION=1 to briefly show synthetic masks on every Plasma output")
	}
	if !util.IsKDEWayland() {
		t.Fatal("this live check requires KDE Wayland")
	}
	displays, err := screen.GetKWinDisplays()
	if err != nil {
		t.Fatal(err)
	}
	if len(displays) < 2 {
		t.Fatal("this live check requires at least two outputs")
	}
	var outputs []linuxDesktopOutput
	for index, display := range displays {
		source := image.NewRGBA(image.Rect(0, 0, display.PixelWidth, display.PixelHeight))
		draw.Draw(source, source.Bounds(), image.NewUniform(color.RGBA{R: uint8(40 + index*30), G: 80, B: 120, A: 255}), image.Point{}, draw.Src)
		outputs = append(outputs, linuxDesktopOutput{
			source:  source,
			bounds:  Rect{X: float32(display.Logical.X), Y: float32(display.Logical.Y), Width: float32(display.Logical.Width), Height: float32(display.Logical.Height)},
			capture: func() (image.Image, error) { return source, nil },
		})
	}
	manager, directory := NewWindowManager(), t.TempDir()
	checked := make(chan error, 1)
	err = woxui.Run(func() error {
		keeper, err := woxui.Open(WindowOptions{Title: "Wox Screenshot", Size: Size{Width: 10, Height: 10}})
		if err != nil {
			return err
		}
		go func() {
			var checkErr error
			for target := range outputs {
				if checkErr = checkLinuxLiveOutputSelection(manager, outputs, directory, target); checkErr != nil {
					break
				}
			}
			checked <- checkErr
			_ = Call(func() { _ = manager.CloseAll(); _ = keeper.Close() })
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := <-checked; err != nil {
		t.Fatal(err)
	}
	t.Logf("verified masks and native-resolution exports on %d outputs", len(outputs))
}

// checkLinuxLiveOutputSelection drives the real capture workflow through its portable input boundary.
func checkLinuxLiveOutputSelection(manager *WindowManager, outputs []linuxDesktopOutput, directory string, target int) error {
	type completion struct {
		result ScreenshotResult
		err    error
	}
	done := make(chan completion, 1)
	go func() {
		result, err := captureLinuxWaylandScreenshot(ScreenshotOptions{
			WindowManager: manager, AutoConfirm: true, ExportDirectory: directory,
		}, linuxLiveMultiOutputCapture{outputs: outputs}, nil, nil)
		done <- completion{result, err}
	}()
	deadline := time.Now().Add(8 * time.Second)
	for {
		ready := true
		for index := range outputs {
			managed, ok := manager.Get(WindowID(fmt.Sprintf("%s.output.%d", ScreenshotWindowID, index)))
			if !ok || managed.Lifecycle() != WindowLifecycleVisible {
				ready = false
				break
			}
		}
		if ready {
			break
		}
		select {
		case completed := <-done:
			return fmt.Errorf("capture ended before masks appeared: %+v", completed)
		default:
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("multi-output masks did not appear")
		}
		time.Sleep(20 * time.Millisecond)
	}
	// Let the compositor deliver configure events and each GtkGLArea submit its first frame.
	time.Sleep(250 * time.Millisecond)
	var nativeErr error
	if err := Call(func() {
		for index, output := range outputs {
			managed, _ := manager.Get(WindowID(fmt.Sprintf("%s.output.%d", ScreenshotWindowID, index)))
			actual, err := managed.Window().Bounds()
			if err != nil || actual != output.bounds {
				nativeErr = fmt.Errorf("output %d native bounds=%+v expected=%+v: %v", index, actual, output.bounds, err)
				return
			}
			path := filepath.Join(directory, fmt.Sprintf("mask-%d-%d.png", target, index))
			if nativeErr = managed.Window().CapturePNG(path); nativeErr != nil {
				return
			}
		}
		managed, _ := manager.Get(WindowID(fmt.Sprintf("%s.output.%d", ScreenshotWindowID, target)))
		for _, event := range []PointerEvent{
			{Kind: PointerEnter, Position: Point{X: 10, Y: 20}},
			{Kind: PointerDown, Button: PointerButtonPrimary, Position: Point{X: 10, Y: 20}},
			{Kind: PointerMove, Position: Point{X: 50, Y: 60}},
			{Kind: PointerUp, Button: PointerButtonPrimary, Position: Point{X: 50, Y: 60}},
		} {
			if nativeErr = managed.Window().DispatchPointer(event); nativeErr != nil {
				return
			}
		}
	}); err != nil {
		return err
	}
	if nativeErr != nil {
		return nativeErr
	}
	select {
	case completed := <-done:
		if completed.err != nil {
			return completed.err
		}
		file, err := os.Open(completed.result.ScreenshotPath)
		if err != nil {
			return err
		}
		defer file.Close()
		exported, err := png.DecodeConfig(file)
		if err != nil {
			return err
		}
		output := outputs[target]
		pixels, err := screenshotEditorPixelSelection(output.source.Bounds(), Rect{X: 10, Y: 20, Width: 40, Height: 40}, Size{Width: output.bounds.Width, Height: output.bounds.Height})
		if err != nil || exported.Width != pixels.Dx() || exported.Height != pixels.Dy() || completed.result.LogicalSelection != (Rect{X: output.bounds.X + 10, Y: output.bounds.Y + 20, Width: 40, Height: 40}) {
			return fmt.Errorf("output %d export=%+v crop=%v result=%+v error=%v", target, exported, pixels, completed.result, err)
		}
		for index := range outputs {
			if _, ok := manager.Get(WindowID(fmt.Sprintf("%s.output.%d", ScreenshotWindowID, index))); ok {
				return fmt.Errorf("output %d mask survived editor handoff", index)
			}
		}
		return nil
	case <-time.After(8 * time.Second):
		return fmt.Errorf("output %d selection did not complete", target)
	}
}
