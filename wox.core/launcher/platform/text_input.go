package platform

import "context"

const DefaultQueryBoxPlaceholder = "Type to search"

type Rect struct {
	X      int
	Y      int
	Width  int
	Height int
}

func (r Rect) IsEmpty() bool {
	return r.Width <= 0 || r.Height <= 0
}

type QueryBoxState struct {
	Visible      bool
	Text         string
	Placeholder  string
	HasFocus     bool
	CaretVisible bool
	Frame        Rect
}

type TextInputState struct {
	QueryBox        QueryBoxState
	SelectionStart  int
	SelectionEnd    int
	CompositionText string
}

type TextInputHost interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	UpdateState(ctx context.Context, state TextInputState) error
	Focus(ctx context.Context) error
	Blur(ctx context.Context) error
}

type TextInputChangeHandler func(ctx context.Context, state TextInputState)
type TextInputSelectionNavigationHandler func(ctx context.Context, delta int)
type TextInputSubmitHandler func(ctx context.Context)

type ObservableTextInputHost interface {
	SetChangeHandler(ctx context.Context, handler TextInputChangeHandler) error
}

type NavigableTextInputHost interface {
	SetSelectionNavigationHandler(ctx context.Context, handler TextInputSelectionNavigationHandler) error
}

type SubmitCapableTextInputHost interface {
	SetSubmitHandler(ctx context.Context, handler TextInputSubmitHandler) error
}

type ParentWindowHost interface {
	SetParentWindow(ctx context.Context, windowHandle uintptr) error
}

type TextInputDebugSnapshot struct {
	ParentWindowHandle uintptr
	HostWindowHandle   uintptr
	EditControlHandle  uintptr
	Focused            bool
	HostVisible        bool
	EditVisible        bool
	Frame              Rect
}

type DebugTextInputHost interface {
	DebugSnapshot(ctx context.Context) TextInputDebugSnapshot
}

type NoopTextInputHost struct{}

func (n *NoopTextInputHost) Start(ctx context.Context) error { return nil }
func (n *NoopTextInputHost) Stop(ctx context.Context) error  { return nil }
func (n *NoopTextInputHost) UpdateState(ctx context.Context, state TextInputState) error {
	return nil
}
func (n *NoopTextInputHost) Focus(ctx context.Context) error { return nil }
func (n *NoopTextInputHost) Blur(ctx context.Context) error  { return nil }
