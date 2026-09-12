package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxTagKeepsFullPixelOutline(t *testing.T) {
	color := woxui.Color{R: 80, G: 90, B: 100, A: 255}
	tag := WoxTag("系统", color).(woxwidget.Container)
	wantPadding := woxwidget.Insets{Left: 4, Top: 2, Right: 4, Bottom: 2}
	if tag.Radius != 3 || tag.BorderWidth != 1 || tag.Padding != wantPadding || tag.BorderColor != color {
		t.Fatalf("tag chrome = radius %v border %v padding %+v color %#v, want 3/1/%+v/%#v", tag.Radius, tag.BorderWidth, tag.Padding, tag.BorderColor, wantPadding, color)
	}
	label := tag.Child.(woxwidget.Text)
	if label.Value != "系统" || label.Style.Size != TagFontSize || label.Color != color {
		t.Fatalf("tag label = %q size %v color %#v, want 系统/%v/%#v", label.Value, label.Style.Size, label.Color, TagFontSize, color)
	}
}

func TestWoxCompactTagUsesDenseMetadataSize(t *testing.T) {
	color := woxui.Color{R: 80, G: 90, B: 100, A: 255}
	tag := WoxCompactTag("Disabled", color).(woxwidget.Container)
	label := tag.Child.(woxwidget.Text)
	if label.Value != "Disabled" || label.Style.Size != CompactTagFontSize {
		t.Fatalf("compact tag = %q size %v, want Disabled/%v", label.Value, label.Style.Size, CompactTagFontSize)
	}
}
