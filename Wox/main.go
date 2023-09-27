package main

import (
	"fmt"
	webview "github.com/webview/webview_go"
	"math"
	"runtime"
	"wox/glfw"
)

var isLeftPressed = false
var startPressWindowX = 0
var startPressWindowY = 0
var startPressMouseX = 0
var startPressMouseY = 0

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
	glfw.WindowHint(glfw.Resizable, glfw.True) // Optional: Prevent resizing if desired
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
		cursorX, cursorY := window.GetCursorPos()
		println(fmt.Sprintf("Mouse move: %f, %f", cursorX, cursorY))

		if isLeftPressed {
			x1, y1 := window.GetCursorPos()
			println("cursor position:", int(x1), int(y1))

			x2 := int(math.Floor(x1)) - startPressMouseX + startPressWindowX
			y2 := int(math.Floor(y1)) - startPressMouseY + startPressWindowY

			println("new position:", x2, y2)

			window.SetPos(x2, y2)
		}
	})
	wb.Bind("OnMouseDown", func() {
		isLeftPressed = true
		startPressWindowX, startPressWindowY = window.GetPos()
		cursorX, cursorY := window.GetCursorPos()
		startPressMouseX, startPressMouseY = int(cursorX), int(cursorY)
		println("Mouse down:", startPressWindowX, startPressWindowY, startPressMouseX, startPressMouseY)
	})
	wb.Bind("OnMouseUp", func() {
		isLeftPressed = false
		println("Mouse up:")
	})
	wb.Bind("OnMouseLeave", func() {
		isLeftPressed = false
		println("Mouse leave:")
	})
	window.MakeContextCurrent()

	for !window.ShouldClose() {
		// Do OpenGL stuff.
		window.SwapBuffers()
		glfw.PollEvents()
	}
}
