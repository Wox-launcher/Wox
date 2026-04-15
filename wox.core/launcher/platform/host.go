package platform

import (
	"context"
	"wox/common"
	launchertheme "wox/launcher/theme"
)

type WindowAppearance struct {
	Transparent    bool
	Acrylic        bool
	RoundedCorners bool
}

type StartOptions struct {
	Appearance WindowAppearance
}

type ShowRequest struct {
	ShowContext common.ShowContext
	Query       common.PlainQuery
	QueryBox    QueryBoxState
	Results     ResultListState
	Theme       launchertheme.PaintTheme
}

type Host interface {
	Start(ctx context.Context, options StartOptions) error
	Stop(ctx context.Context) error
	Show(ctx context.Context, request ShowRequest) error
	Hide(ctx context.Context) error
	IsVisible(ctx context.Context) bool
}

type NativeWindowProvider interface {
	NativeWindowHandle(ctx context.Context) uintptr
}

type HostDebugSnapshot struct {
	Visible            bool
	NativeWindowHandle uintptr
}

type DebugHost interface {
	DebugSnapshot(ctx context.Context) HostDebugSnapshot
}

type EmbeddedTextInputSupport interface {
	SupportsEmbeddedTextInput(ctx context.Context) bool
}
