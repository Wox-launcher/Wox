package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestDictationModelManagerUsesFieldAnchoredMenu(t *testing.T) {
	anchor := woxui.Rect{X: 320, Y: 180, Width: 600, Height: 34}
	overlay := ModelManagerView(ModelManagerProps{
		Width: 1200, Height: 800, Anchor: anchor, Anchored: true, EngineReady: true,
		RecommendedLabel: "Recommended", DeleteLabel: "Delete", Theme: woxcomponent.Theme{},
		Options: []ModelManagerOption{{Name: "Qwen3-ASR 0.6B", Languages: "Chinese, English", Description: "Offline recognition", SizeMB: 600, Recommended: true, ActionLabel: "Download", ActionEnabled: true}},
	})
	stack, ok := overlay.(woxwidget.Stack)
	if !ok {
		t.Fatalf("model overlay type = %T, want anchored stack", overlay)
	}
	if len(stack.Children) != 2 {
		t.Fatalf("model overlay child count = %d, want backdrop and menu", len(stack.Children))
	}
	menu := stack.Children[1]
	if menu.Left != anchor.X || menu.Top != anchor.Y+anchor.Height {
		t.Fatalf("model menu position = (%.0f, %.0f), want (%.0f, %.0f)", menu.Left, menu.Top, anchor.X, anchor.Y+anchor.Height)
	}
	focusScope := menu.Child.(woxwidget.FocusScope)
	menuStack := focusScope.Child.(woxwidget.Stack)
	content := menuStack.Children[0].Child.(woxwidget.Container)
	if content.Width != anchor.Width || content.Radius != 4 {
		t.Fatalf("model menu geometry = width %.0f radius %.0f, want field width %.0f and radius 4", content.Width, content.Radius, anchor.Width)
	}
}
