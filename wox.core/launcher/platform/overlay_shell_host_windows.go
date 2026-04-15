//go:build windows

package platform

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
	"wox/util/overlay"
)

const (
	shellOverlayName             = "wox-native-launcher-shell"
	defaultShellWidth            = 760
	defaultShellHeight           = 240
	shellMessage                 = "Wox Native Launcher"
	shellWindowTopOffset float64 = 96
)

type OverlayShellHost struct {
	mu         sync.RWMutex
	visible    bool
	appearance WindowAppearance
}

func NewOverlayShellHost() *OverlayShellHost {
	return &OverlayShellHost{}
}

func (h *OverlayShellHost) Start(ctx context.Context, options StartOptions) error {
	_ = ctx

	h.mu.Lock()
	defer h.mu.Unlock()

	h.appearance = options.Appearance
	return nil
}

func (h *OverlayShellHost) Stop(ctx context.Context) error {
	return h.Hide(ctx)
}

func (h *OverlayShellHost) Show(ctx context.Context, request ShowRequest) error {
	_ = ctx

	h.mu.Lock()
	h.visible = true
	h.mu.Unlock()

	overlay.Show(buildShellOverlayOptions(request))
	return nil
}

func (h *OverlayShellHost) Hide(ctx context.Context) error {
	_ = ctx

	h.mu.Lock()
	h.visible = false
	h.mu.Unlock()

	overlay.Close(shellOverlayName)
	return nil
}

func (h *OverlayShellHost) IsVisible(ctx context.Context) bool {
	_ = ctx

	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.visible
}

func (h *OverlayShellHost) NativeWindowHandle(ctx context.Context) uintptr {
	_ = ctx

	deadline := time.Now().Add(200 * time.Millisecond)

	for {
		if hwnd := overlay.GetWindowHandle(shellOverlayName); hwnd != 0 {
			return hwnd
		}
		if time.Now().After(deadline) {
			return 0
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func (h *OverlayShellHost) DebugSnapshot(ctx context.Context) HostDebugSnapshot {
	return HostDebugSnapshot{
		Visible:            h.IsVisible(ctx),
		NativeWindowHandle: h.NativeWindowHandle(ctx),
	}
}

func (h *OverlayShellHost) SupportsEmbeddedTextInput(ctx context.Context) bool {
	_ = ctx
	return false
}

func buildShellOverlayOptions(request ShowRequest) overlay.OverlayOptions {
	width := defaultShellWidth
	if request.ShowContext.WindowWidth > 0 {
		width = request.ShowContext.WindowWidth
	}

	options := overlay.OverlayOptions{
		Name:     shellOverlayName,
		Title:    "Wox Native Launcher",
		Message:  buildShellMessage(request),
		Closable: false,
		Movable:  true,
		Width:    float64(width),
		Height:   defaultShellHeight,
		Anchor:   overlay.AnchorTopCenter,
		OffsetY:  shellWindowTopOffset,
	}

	if request.ShowContext.WindowPosition != nil {
		options.Anchor = overlay.AnchorTopLeft
		options.OffsetX = float64(request.ShowContext.WindowPosition.X)
		options.OffsetY = float64(request.ShowContext.WindowPosition.Y)
	}

	return options
}

func buildShellMessage(request ShowRequest) string {
	lines := []string{shellMessage}
	if request.ShowContext.ShowSource != "" {
		lines = append(lines, fmt.Sprintf("source: %s", request.ShowContext.ShowSource))
	}
	return strings.Join(lines, "\n")
}
