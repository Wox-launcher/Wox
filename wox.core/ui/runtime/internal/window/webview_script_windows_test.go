//go:build windows

package window

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	webviewruntime "wox/ui/runtime/internal/webview"
)

// TestWindowsWebViewScriptFixture exercises the real WebView2 with simulated plugin
// state. The optional fixture contains only HTML and scripts, never a cleanup command.
func TestWindowsWebViewScriptFixture(t *testing.T) {
	path := os.Getenv("WOX_WEBVIEW_SCRIPT_FIXTURE")
	if path == "" {
		t.Skip("set WOX_WEBVIEW_SCRIPT_FIXTURE to a simulated HTML/steps JSON fixture")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		HTML  string
		Steps []string
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	finished := make(chan error, 1)
	err = Run(func() error {
		window, err := Open(WindowOptions{Title: "Wox simulated preview update test", Size: Size{Width: 800, Height: 560}})
		if err != nil {
			return err
		}
		if _, err = window.Show(); err != nil {
			return err
		}
		driver, err := newWindowsWebViewDriver(uintptr(window.native.hwnd), window.native.renderer)
		if err != nil {
			return err
		}
		controller := webviewruntime.New(driver, Call)
		if err = controller.Show(webviewruntime.Content{DocumentKey: "smoke/query/row", HTML: fixture.HTML}, webviewruntime.Rect{Width: 800, Height: 560}, window.native.scale); err != nil {
			return err
		}
		go func() {
			runErr := func() error {
				evaluate := func(source string) (string, error) {
					type answer struct {
						value string
						err   error
					}
					done := make(chan answer, 1)
					err := Call(func() {
						err := driver.Evaluate(source, func(value string, err error) { done <- answer{value, err} })
						if err != nil {
							done <- answer{err: err}
						}
					})
					if err != nil {
						return "", err
					}
					select {
					case result := <-done:
						return result.value, result.err
					case <-time.After(10 * time.Second):
						return "", fmt.Errorf("evaluation timeout")
					}
				}
				// Queue before WebView2 finishes creating; this must wait for document readiness.
				if !controller.UpdateScript("smoke/query/row", "window.__queuedAtLoad=true") {
					return fmt.Errorf("loading update rejected")
				}
				deadline := time.Now().Add(20 * time.Second)
				for {
					value, err := evaluate("window.__queuedAtLoad === true ? 'ready' : 'pending'")
					if err != nil {
						return err
					}
					if value == "ready" {
						break
					}
					if time.Now().After(deadline) {
						return fmt.Errorf("document never became ready: %s", value)
					}
					time.Sleep(50 * time.Millisecond)
				}
				for index, source := range fixture.Steps {
					// Exercise the production queue; a later check observes its result.
					script := "try {" + source + ";window.__step=" + fmt.Sprint(index) + ";} catch(e) {window.__failure=String(e);window.__step=" + fmt.Sprint(index) + ";}"
					if !controller.UpdateScript("smoke/query/row", script) {
						return fmt.Errorf("step %d rejected", index)
					}
					// Mirror the launcher: reconcile the resolved HTML and redraw after
					// an Actions update while the script may still be queued.
					controller.RetainScriptTarget("smoke/query/row", fixture.HTML)
					if err := controller.Show(webviewruntime.Content{DocumentKey: "smoke/query/row", HTML: fixture.HTML}, webviewruntime.Rect{Width: 800, Height: 560}, window.native.scale); err != nil {
						return err
					}
					for attempt := 0; attempt < 100; attempt++ {
						value, err := evaluate("window.__failure || (window.__step === " + fmt.Sprint(index) + " ? 'ok' : 'pending')")
						if err != nil {
							return err
						}
						if value == "ok" {
							break
						}
						if value != "pending" {
							return fmt.Errorf("step %d: %s", index, value)
						}
						if attempt == 99 {
							return fmt.Errorf("step timeout")
						}
						time.Sleep(20 * time.Millisecond)
					}
					time.Sleep(80 * time.Millisecond)
				}
				return nil
			}()
			finished <- runErr
			controller.Close()
			_ = window.Close()
		}()
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err = <-finished; err != nil {
		t.Fatal(err)
	}
}
