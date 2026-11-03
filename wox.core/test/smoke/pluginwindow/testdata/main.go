//go:build darwin

package main

/*
#cgo LDFLAGS: -framework Cocoa -framework WebKit
#include <stdlib.h>
int woxSmokeInstallInput(const char *title);
int woxSmokePostPointer(int kind, double x, double y);
int woxSmokeStartSnapshot(void);
int woxSmokeSnapshotResult(void);
void woxSmokeRemoveInput(void);
*/
import "C"

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"wox/plugin"
	"wox/ui/pluginwindow"
	ui "wox/ui/runtime"
	"wox/util"
	"wox/util/overlay"
)

const title = "Wox WebView smoke"

const html = `<!doctype html><html><head><style>
html,body{width:100%;height:100%;margin:0;background:transparent}
#marker{position:absolute;left:8px;top:8px;width:16px;height:16px;background:red}
button{position:absolute;right:6px;top:6px;width:24px;height:24px;opacity:0;pointer-events:none}
body:hover button{opacity:1;pointer-events:auto}
</style></head><body><div id="marker"></div><button id="close">×</button><script>
const closeButton=document.getElementById('close');
function hover(){setTimeout(()=>woxWindow.postMessage({kind:'hover',visible:getComputedStyle(closeButton).opacity==='1'}),0)}
document.body.addEventListener('mouseenter',hover);
document.body.addEventListener('mouseleave',hover);
closeButton.addEventListener('click',()=>woxWindow.postMessage({kind:'click'}));
</script></body></html>`

type message struct {
	Kind    string `json:"kind"`
	Visible bool   `json:"visible"`
}

func main() {
	if err := util.GetLocation().Init(); err != nil {
		fail(err)
	}
	result := make(chan error, 1)
	err := ui.Run(func() error {
		overlay.SetWindowManager(ui.NewWindowManager())
		width, height := float64(128), float64(76)
		enabled := true
		events := make(chan plugin.WindowEvent, 32)
		driver, err := pluginwindow.Create("macos-webview-smoke", plugin.CreateWindowOption{
			Window:  plugin.WindowOptions{Title: title, Width: &width, Height: &height, HideChrome: &enabled, Movable: &enabled, DragContent: &enabled},
			Content: plugin.WindowContent{Type: plugin.WindowContentHTML, Data: html},
		}, func(event plugin.WindowEvent) { events <- event })
		if err != nil {
			return err
		}
		if err := driver.Show(plugin.WindowShowOption{}); err != nil {
			_ = driver.Close()
			return err
		}
		go func() {
			err := exercise(events)
			if removeErr := ui.Call(func() { C.woxSmokeRemoveInput() }); err == nil {
				err = removeErr
			}
			if closeErr := driver.Close(); err == nil {
				err = closeErr
			}
			if err == nil {
				deadline := time.NewTimer(5 * time.Second)
				defer deadline.Stop()
			waitClosed:
				for {
					select {
					case event := <-events:
						if event.Type == plugin.WindowEventClosed {
							break waitClosed
						}
					case <-deadline.C:
						err = errors.New("timed out waiting for closed event")
						break waitClosed
					}
				}
			}
			result <- err
		}()
		return nil
	})
	if err != nil {
		fail(err)
	}
	select {
	case err := <-result:
		if err != nil {
			fail(err)
		}
	case <-time.After(5 * time.Second):
		fail(errors.New("smoke fixture did not finish"))
	}
}

func fail(err error) {
	_, _ = fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func exercise(events <-chan plugin.WindowEvent) error {
	deadline := time.NewTimer(35 * time.Second)
	defer deadline.Stop()
	wait := func(check func(plugin.WindowEvent) bool) error {
		for {
			select {
			case event := <-events:
				if event.Type == plugin.WindowEventError {
					return fmt.Errorf("plugin window error: %s", event.ErrMsg)
				}
				if check(event) {
					return nil
				}
			case <-deadline.C:
				return errors.New("timed out waiting for plugin-window WebView event")
			}
		}
	}
	if err := wait(func(event plugin.WindowEvent) bool { return event.Type == plugin.WindowEventReady }); err != nil {
		return err
	}

	var started bool
	if err := ui.Call(func() {
		name := C.CString(title)
		defer C.free(unsafe.Pointer(name))
		started = C.woxSmokeInstallInput(name) != 0 && C.woxSmokeStartSnapshot() != 0
	}); err != nil || !started {
		return fmt.Errorf("prepare AppKit input and WebView snapshot: %v", err)
	}
	snapshotDeadline := time.Now().Add(10 * time.Second)
	for {
		var status int
		if err := ui.Call(func() { status = int(C.woxSmokeSnapshotResult()) }); err != nil {
			return err
		}
		if status > 0 {
			break
		}
		if status < 0 {
			if time.Now().After(snapshotDeadline) {
				return errors.New("WebView snapshot did not contain both transparent pixels and the red HTML marker")
			}
			if err := ui.Call(func() { C.woxSmokeStartSnapshot() }); err != nil {
				return err
			}
		}
		select {
		case <-deadline.C:
			return errors.New("timed out waiting for WebView snapshot")
		case <-time.After(25 * time.Millisecond):
		}
	}

	post := func(kind int, x, y float64) error {
		var posted bool
		if err := ui.Call(func() { posted = C.woxSmokePostPointer(C.int(kind), C.double(x), C.double(y)) != 0 }); err != nil {
			return err
		}
		if !posted {
			return errors.New("could not post AppKit pointer event")
		}
		return nil
	}
	waitMessage := func(kind string, visible bool) error {
		return wait(func(event plugin.WindowEvent) bool {
			if event.Type != plugin.WindowEventMessage {
				return false
			}
			var received message
			return json.Unmarshal(event.Data, &received) == nil && received.Kind == kind && (kind != "hover" || received.Visible == visible)
		})
	}
	if err := post(0, 64, 40); err != nil {
		return err
	}
	if err := waitMessage("hover", true); err != nil {
		return fmt.Errorf("hover enter: %w", err)
	}
	if err := post(1, -10, -10); err != nil {
		return err
	}
	if err := waitMessage("hover", false); err != nil {
		return fmt.Errorf("hover leave: %w", err)
	}
	if err := post(0, 110, 18); err != nil {
		return err
	}
	if err := waitMessage("hover", true); err != nil {
		return fmt.Errorf("hover re-entry: %w", err)
	}
	if err := post(2, 110, 18); err != nil {
		return err
	}
	if err := post(3, 110, 18); err != nil {
		return err
	}
	if err := waitMessage("click", false); err != nil {
		return fmt.Errorf("close button click: %w", err)
	}
	return nil
}
