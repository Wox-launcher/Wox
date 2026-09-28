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

func TestWoxWarningTagScalesAndKeepsOpaqueStatusColors(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1, 1.5} {
		theme := ControlTheme{DensityScale: scale, Warning: woxui.Color{R: 253, G: 230, B: 138, A: 255}, WarningText: woxui.Color{R: 102, G: 60, A: 255}}
		tag := WoxWarningTag("第三方插件已禁用", 180, theme).(woxwidget.Container)
		label := tag.Child.(woxwidget.TextBlock)
		if tag.Height != theme.Scaled(26) || tag.Radius != theme.Scaled(4) || tag.Color != theme.Warning || label.Color != theme.WarningText {
			t.Fatalf("scale=%v: warning tag lost scaled geometry or status colors: %+v", scale, tag)
		}
		if !label.Centered || label.AlignmentY != 0.5 || label.MaxLines != 1 || label.Width != max(0, tag.Width-theme.Scaled(16)) || label.Style.Size != theme.Scaled(TagFontSize) || label.Style.Weight != woxui.FontWeightSemibold {
			t.Fatalf("scale=%v: warning label lost centered alignment, truncation or density: %+v", scale, label)
		}
	}
}
