package main

import (
	webview "github.com/webview/webview_go"
	"runtime"
	"wox/glfw"
)

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

	window, err := glfw.CreateWindow(640, 580, "Testing", nil, nil)
	if err != nil {
		panic(err)
	}

	wb := webview.NewWindow(false, window.GetCocoaWindow())
	wb.Navigate("http://www.sina.com.cn")

	window.MakeContextCurrent()

	for !window.ShouldClose() {
		// Do OpenGL stuff.
		window.SwapBuffers()
		glfw.PollEvents()
	}
}
