package main

import (
	webview "github.com/webview/webview_go"
	"math"
	"runtime"
	"wox/glfw"
)

var isLeftPressed = false
var lastWindowX = 0
var lastWindowY = 0
var lastCursorX float64 = 0
var lastCursorY float64 = 0

func init() {
	// This is needed to arrange that main() runs on main thread.
	// See documentation for functions that are only allowed to be called from the main thread.
	runtime.LockOSThread()
}

func main() {
	err := glfw.Init()
	if err != nil {
		panic(err)
	}
	defer glfw.Terminate()

	glfw.WindowHint(glfw.Decorated, glfw.False)
	window, err := glfw.CreateWindow(640, 680, "Testing", nil, nil)
	if err != nil {
		panic(err)
	}

	window.SetCursorPosCallback(func(w *glfw.Window, xpos float64, ypos float64) {
		println("Cursor position:", xpos, ypos)
	})

	window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mod glfw.ModifierKey) {
		println("Mouse button:", button, action, mod)
	})

	window.SetFocusCallback(func(w *glfw.Window, focused bool) {
		println("Focus:", focused)
	})

	wb := webview.NewWindow(false, window.GetCocoaWindow())
	wb.Navigate("https://1609052.playcode.io/")
	wb.Bind("MousePos", func(x, y int) {
		currentCursorX, currentCursorY := window.GetCursorPos()
		currentWindowX, currentWindowY := window.GetPos()

		if isLeftPressed {
			newWindowX := int(math.Round((currentCursorX-lastCursorX)*0.97)) + currentWindowX
			newWindowY := int(math.Round((currentCursorY-lastCursorY)*0.97)) + currentWindowY
			window.SetPos(newWindowX, newWindowY)
		}

		lastWindowX, lastWindowY = window.GetPos()
		lastCursorX, lastCursorY = window.GetCursorPos()
	})
	wb.Bind("OnMouseDown", func() {
		isLeftPressed = true
		lastWindowX, lastWindowY = window.GetPos()
		lastCursorX, lastCursorY = window.GetCursorPos()
	})
	wb.Bind("OnMouseUp", func() {
		isLeftPressed = false
	})
	wb.Bind("OnMouseLeave", func() {
		isLeftPressed = false
	})
	window.MakeContextCurrent()

	for !window.ShouldClose() {
		// Do OpenGL stuff.
		window.SwapBuffers()
		glfw.PollEvents()
	}
}
