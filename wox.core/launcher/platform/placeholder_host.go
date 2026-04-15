package platform

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"wox/util/overlay"
)

const (
	placeholderOverlayName   = "wox-native-launcher-shell"
	placeholderOverlayWidth  = 760
	placeholderOverlayHeight = 420
)

type PlaceholderHost struct {
	mu         sync.RWMutex
	visible    bool
	appearance WindowAppearance
}

func NewPlaceholderHost() *PlaceholderHost {
	return &PlaceholderHost{}
}

func (h *PlaceholderHost) Start(ctx context.Context, options StartOptions) error {
	_ = ctx

	h.mu.Lock()
	defer h.mu.Unlock()

	h.appearance = options.Appearance
	return nil
}

func (h *PlaceholderHost) Stop(ctx context.Context) error {
	return h.Hide(ctx)
}

func (h *PlaceholderHost) Show(ctx context.Context, request ShowRequest) error {
	_ = ctx

	h.mu.Lock()
	h.visible = true
	appearance := h.appearance
	h.mu.Unlock()

	width := placeholderOverlayWidth
	if request.ShowContext.WindowWidth > 0 {
		width = request.ShowContext.WindowWidth
	}

	overlay.Show(overlay.OverlayOptions{
		Name:     placeholderOverlayName,
		Title:    "Wox Native Launcher",
		Message:  buildPlaceholderMessage(request, appearance),
		Closable: false,
		Movable:  true,
		Width:    float64(width),
		Height:   placeholderOverlayHeight,
	})

	return nil
}

func (h *PlaceholderHost) Hide(ctx context.Context) error {
	_ = ctx

	h.mu.Lock()
	h.visible = false
	h.mu.Unlock()

	overlay.Close(placeholderOverlayName)
	return nil
}

func (h *PlaceholderHost) IsVisible(ctx context.Context) bool {
	_ = ctx

	h.mu.RLock()
	defer h.mu.RUnlock()

	return h.visible
}

func buildPlaceholderMessage(request ShowRequest, appearance WindowAppearance) string {
	lines := []string{
		"Native launcher shell placeholder",
		fmt.Sprintf("appearance: transparent=%t acrylic=%t rounded=%t", appearance.Transparent, appearance.Acrylic, appearance.RoundedCorners),
	}

	if request.QueryBox.Visible {
		queryText := request.QueryBox.Text
		if strings.TrimSpace(queryText) == "" {
			queryText = request.QueryBox.Placeholder
		}
		if request.QueryBox.CaretVisible {
			queryText += "|"
		}
		lines = append(lines, "query box: "+queryText)
	}

	if request.ShowContext.ShowSource != "" {
		lines = append(lines, "show source: "+string(request.ShowContext.ShowSource))
	}

	if request.ShowContext.HideQueryBox {
		lines = append(lines, "query box: hidden")
	} else {
		lines = append(lines, "query box: visible")
	}

	if request.ShowContext.WindowWidth > 0 {
		lines = append(lines, fmt.Sprintf("window width: %d", request.ShowContext.WindowWidth))
	}
	if request.Results.Visible {
		lines = append(lines, fmt.Sprintf("results: %d", len(request.Results.Items)))
		for index, item := range request.Results.Items {
			if index >= 5 {
				lines = append(lines, "...")
				break
			}
			lines = append(lines, fmt.Sprintf("%d. %s", index+1, item.Title))
		}
	}

	return strings.Join(lines, "\n")
}
