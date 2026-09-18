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
		RecommendedLabel: "Recommended", DeleteLabel: "Delete", Theme: woxcomponent.ControlTheme{},
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

func TestDictationModelManagerHidesUnknownEngineStatus(t *testing.T) {
	anchor := woxui.Rect{X: 320, Y: 180, Width: 600, Height: 34}
	overlay := ModelManagerView(ModelManagerProps{
		Width: 1200, Height: 800, Anchor: anchor, Anchored: true,
		EngineKnown: false, EngineReady: false, EngineLabel: "Checking inference engine…", Theme: woxcomponent.ControlTheme{},
		Options: []ModelManagerOption{{Name: "Qwen3-ASR 0.6B", ActionLabel: "Download", ActionEnabled: true}},
	})
	stack := overlay.(woxwidget.Stack)
	menuStack := stack.Children[1].Child.(woxwidget.FocusScope).Child.(woxwidget.Stack)
	content := menuStack.Children[0].Child.(woxwidget.Container)
	if content.Height != ModelManagerDropdownRowHeight {
		t.Fatalf("unknown engine menu height = %.0f, want model-only height %.0f", content.Height, ModelManagerDropdownRowHeight)
	}
}

func TestDictationModelManagerDropdownShowsLanguagesOnly(t *testing.T) {
	detail := modelManagerDropdownDetail(ModelManagerOption{
		Name: "Qwen3-ASR 0.6B", Languages: "Chinese, English, Japanese, Korean, Cantonese + 24 more", Description: "Alibaba Qwen3-ASR 0.6B. Offline recognition with VAD segmentation.",
	}, []woxwidget.Widget{woxwidget.Text{Value: "Qwen3-ASR 0.6B"}}, woxcomponent.ControlTheme{})
	children := detail.(woxwidget.Container).Child.(woxwidget.Flex).Children
	if len(children) != 2 {
		t.Fatalf("dropdown detail children = %d, want title and languages only", len(children))
	}
	languages := children[1].(woxwidget.TextBlock)
	if languages.Value != "Chinese, English, Japanese, Korean, Cantonese + 24 more" || languages.MaxLines != 1 {
		t.Fatalf("dropdown languages = %+v, want a single language line", languages)
	}
	for _, child := range children {
		if text, ok := child.(woxwidget.TextBlock); ok && text.Value == "Alibaba Qwen3-ASR 0.6B. Offline recognition with VAD segmentation." {
			t.Fatal("dropdown should not show the long model description")
		}
	}
}

func TestModelManagerTrailingBoundaryEqualCoversAllFields(t *testing.T) {
	woxwidget.AssertEqualCoversAllFields(t, modelManagerTrailingProps{
		Index: 1, State: "not_downloaded", Progress: 10, ActionLabel: "Download", ActionEnabled: true, DeleteLabel: "Delete", Width: 96,
		Theme: woxcomponent.ControlTheme{Text: woxui.Color{A: 255}}, DownloadIcon: &woxui.Image{Width: 14, Height: 14}, DeleteIcon: &woxui.Image{Width: 16, Height: 16}, ErrorIcon: &woxui.Image{Width: 14, Height: 14},
		OnAction: func() {}, OnDelete: func() {},
	})
}

func TestModelManagerTrailingKeepsDownloadButtonWhenSiblingProgressChanges(t *testing.T) {
	idle := modelManagerTrailingProps{Index: 2, State: "not_downloaded", ActionLabel: "Download", ActionEnabled: true, Width: 96}
	if !idle.Equal(idle) {
		t.Fatal("unchanged download trailing should stay equal across sibling progress ticks")
	}
	downloading := idle
	downloading.Index = 1
	downloading.State = "downloading"
	downloading.Progress = 55
	downloading.ActionLabel = "55%"
	if idle.Equal(downloading) {
		t.Fatal("downloading trailing must not reuse an idle download button")
	}
	button := modelManagerTrailing(idle).(woxwidget.Semantics)
	if button.AutomationID != "model-action-2" || button.Label != "Download" || button.Role != woxui.AccessibilityRoleButton {
		t.Fatalf("idle trailing = %+v, want an enabled download button", button)
	}
	progress := modelManagerTrailing(downloading).(woxwidget.Semantics)
	if progress.AutomationID != "model-progress-1" || progress.Role != woxui.AccessibilityRoleProgressBar || progress.Value != "55%" {
		t.Fatalf("downloading trailing = %+v, want a progress control", progress)
	}
}

func TestModelManagerDialogDoesNotDuplicateBottomPadding(t *testing.T) {
	dialog := ModelManagerView(ModelManagerProps{
		Width: 1200, Height: 800, Title: "Models", Theme: woxcomponent.ControlTheme{},
		Options: []ModelManagerOption{{Name: "Model", ActionLabel: "Download", ActionEnabled: true}},
	}).(woxwidget.Stateful)
	props := dialog.Widget.(woxcomponent.DialogProps)
	children := props.Child.(woxwidget.Flex).Children
	list := children[2].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	footer := children[4].(woxwidget.Container)

	usedHeight := children[0].(woxwidget.Container).Height + children[1].(woxwidget.Container).Height + list.Height + children[3].(woxwidget.Container).Height + footer.Height
	if usedHeight+props.Padding.Top+props.Padding.Bottom != props.Height {
		t.Fatal("model manager dialog should not reserve extra space below its footer")
	}
	if footer.Height != 50 || footer.Padding.Top != 10 {
		t.Fatalf("model manager footer = height %v padding %+v, want action height plus top spacing only", footer.Height, footer.Padding)
	}
}
