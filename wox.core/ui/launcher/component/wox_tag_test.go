package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxTagsUseScaledMetadataStyle(t *testing.T) {
	for _, scale := range []float32{0.9, 1, 1.1, 1.5} {
		for _, foreground := range []woxui.Color{{R: 240, G: 240, B: 240, A: 255}, {R: 30, G: 30, B: 30, A: 255}} {
			theme := ControlTheme{DensityScale: scale, Text: foreground, TextSecondary: foreground}
			for _, compact := range []bool{false, true} {
				tag := WoxTag("系统", foreground, theme).(woxwidget.Container)
				size, height := TagFontSize, float32(22)
				if compact {
					tag = WoxCompactTag("系统", foreground, theme).(woxwidget.Container)
					size, height = CompactTagFontSize, 18
				}
				background := foreground
				background.A = 13
				if tag.Radius != theme.Scaled(4) || tag.BorderWidth != 0 || tag.Color != background || tag.Height != theme.Scaled(height) || tag.Padding != (woxwidget.Insets{Left: theme.Scaled(7), Right: theme.Scaled(7)}) {
					t.Fatalf("scale %v compact %v: unexpected tag chrome %+v", scale, compact, tag)
				}
				label := tag.Child.(woxwidget.TextBlock)
				if label.Value != "系统" || label.Style.Size != theme.Scaled(size) || label.Color != foreground || !label.ShrinkWrap || label.AlignmentY != 0.5 || label.Height != tag.Height {
					t.Fatalf("scale %v compact %v: unexpected tag label %+v", scale, compact, label)
				}
			}
		}
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
