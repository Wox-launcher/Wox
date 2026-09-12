package preview

import (
	"testing"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestPreviewThemeColors verifies independent surfaces preserve exact authored alpha.
func TestPreviewThemeColors(t *testing.T) {
	for _, color := range []woxui.Color{{}, {R: 23, G: 45, B: 67, A: 89}} {
		theme := woxcomponent.Theme{PreviewText: woxui.Color{A: 255}, PreviewSplit: woxui.Color{A: 255}, PreviewBackgroundColor: &color, PreviewBorderColor: &color, PreviewTagFontColor: &color, PreviewTagBackgroundColor: &color, PreviewTagBorderColor: &color}
		surface := previewSurface(woxwidget.Container{}, theme, 300, 180).(woxwidget.Container)
		if surface.Color != color || surface.BorderColor != color {
			t.Fatal("preview overwrote authored alpha")
		}
		tags := PreviewTags([]PreviewTag{{Label: "Tag"}}, theme, &woxui.Window{}, 200, nil).(woxwidget.ScrollView)
		pill := tags.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
		label := pill.Child.(woxwidget.Align)
		if label.Height != pill.Height || label.Vertical != 0.5 || pill.Padding.Top != 0 || pill.Padding.Bottom != 0 {
			t.Fatal("tag label must be centered within its full height")
		}
		if pill.Color != color || pill.BorderColor != color || label.Child.(woxwidget.Text).Color != color {
			t.Fatal("tag overwrote authored alpha")
		}
	}
}

// TestPreviewCornerGeometry preserves legacy defaults and explicit square corners.
func TestPreviewCornerGeometry(t *testing.T) {
	for _, value := range []int{-1, 0, 16, 200} {
		theme := woxcomponent.Theme{}
		wantSurface, wantTag := float32(8), float32(8)
		if value >= 0 {
			theme.PreviewBorderRadius, theme.PreviewTagBorderRadius = &value, &value
			wantSurface, wantTag = min(float32(value), 90), min(float32(value), 13)
		}
		surface := previewSurface(woxwidget.Container{}, theme, 300, 180).(woxwidget.Container)
		tags := PreviewTags([]PreviewTag{{Label: "Tag"}}, theme, &woxui.Window{}, 200, nil).(woxwidget.ScrollView)
		pill := tags.Child.(woxwidget.Flex).Children[0].(woxwidget.Container)
		if surface.Radius != wantSurface || pill.Radius != wantTag {
			t.Fatalf("radius %d: surface=%v tag=%v", value, surface.Radius, pill.Radius)
		}
	}
}
