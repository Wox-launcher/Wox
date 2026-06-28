//go:build !windows && !darwin

package ui

// On platforms without a native renderer (Linux etc.), these stubs allow the
// rest of the project to compile. The real implementations live in
// ui_windows.go and ui_darwin.go.

type stubRenderer struct{}
type stubTextMeasurer struct{}

func (stubTextMeasurer) MeasureText(text string, fontSize float32, fontFamily string) (width, height float32) {
	runeCount := float32(0)
	for range text {
		runeCount++
	}
	return runeCount * fontSize * 0.6, fontSize * 1.2
}

func NewNativeRenderer(width, height int, theme Theme) (*stubRenderer, error) {
	return nil, &WindowError{Op: "create", Err: "native renderer not implemented on this platform"}
}

func SetEventHandler(cb EventCallback) {}

func (r *stubRenderer) Render(commands *CommandList) error { return nil }
func (r *stubRenderer) TextMeasurer() TextMeasurer         { return stubTextMeasurer{} }
func (r *stubRenderer) Show() error                        { return nil }
func (r *stubRenderer) Hide() error                        { return nil }
func (r *stubRenderer) SetPosition(x, y int) error         { return nil }
func (r *stubRenderer) SetSize(w, h int) error             { return nil }
func (r *stubRenderer) Close() error                        { return nil }
func (r *stubRenderer) IsVisible() bool                    { return false }
func (r *stubRenderer) GetSize() (int, int)                { return 0, 0 }
func (r *stubRenderer) SetDarkMode(dark bool)              {}
func (r *stubRenderer) ReleaseMemory()                     {}
func (r *stubRenderer) RequestRepaint()                    {}
func (r *stubRenderer) RunMessageLoop(onRender func() *CommandList) {}