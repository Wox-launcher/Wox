package component

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// WoxTag shares the quiet metadata styling used by preview and result-tail tags.
func WoxTag(label string, color woxui.Color, theme ControlTheme) woxwidget.Widget {
	return woxTag(label, color, theme, TagFontSize, 22)
}

// WoxCompactTag is the 9px table-row status chip beside 13px cell text.
func WoxCompactTag(label string, color woxui.Color, theme ControlTheme) woxwidget.Widget {
	return woxTag(label, color, theme, CompactTagFontSize, 18)
}

// WoxWarningTag uses an opaque status fill so its contrast survives custom launcher backgrounds.
func WoxWarningTag(label string, width float32, theme ControlTheme) woxwidget.Widget {
	height := theme.Scaled(26)
	return woxwidget.Container{
		Width: width, Height: height, Radius: theme.Scaled(4), Color: theme.Warning,
		Padding: woxwidget.Insets{Left: theme.Scaled(8), Right: theme.Scaled(8)},
		Child: woxwidget.TextBlock{
			Value: label, Width: max(0, width-theme.Scaled(16)), Height: height, LineHeight: height,
			MaxLines: 1, Centered: true, AlignmentY: 0.5,
			Style: woxui.TextStyle{Size: theme.Scaled(TagFontSize), Weight: woxui.FontWeightSemibold}, Color: theme.WarningText,
		},
	}
}

// woxTag sizes to its label so translations and density changes need no fixed badge slot.
func woxTag(label string, color woxui.Color, theme ControlTheme, size, height float32) woxwidget.Widget {
	background := theme.Text
	background.A = 13
	height = theme.Scaled(height)
	return woxwidget.Container{
		Height: height, Radius: theme.Scaled(4), Color: background,
		Padding: woxwidget.Insets{Left: theme.Scaled(7), Right: theme.Scaled(7)},
		Child: woxwidget.TextBlock{
			Value: label, Style: woxui.TextStyle{Size: theme.Scaled(size)}, Color: color,
			Height: height, LineHeight: height, MaxLines: 1, ShrinkWrap: true, AlignmentY: 0.5,
		},
	}
}
