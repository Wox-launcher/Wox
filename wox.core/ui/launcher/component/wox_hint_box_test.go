package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestWoxHintBoxUsesIntrinsicTextHeight(t *testing.T) {
	accent := woxui.Color{R: 64, G: 196, B: 255, A: 255}
	dark := Theme{Background: woxui.Color{R: 20, G: 20, B: 20, A: 255}}
	single := WoxHintBox(HintBoxProps{Text: "Single", Width: 400, MaxLines: 1, Accent: accent, Theme: dark}).(woxwidget.Container)
	multi := WoxHintBox(HintBoxProps{Text: "Multiple lines", Width: 400, MaxLines: 2, Accent: accent, Theme: dark}).(woxwidget.Container)

	if single.Height != 0 || single.Padding != woxwidget.UniformInsets(12) || single.Child.(woxwidget.Flex).CrossAxisAlignment != woxwidget.CrossAxisStart {
		t.Fatalf("single-line hint geometry = height %v padding %v alignment %v", single.Height, single.Padding, single.Child.(woxwidget.Flex).CrossAxisAlignment)
	}
	text := multi.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if multi.Height != 0 || text.Height != 0 || text.MaxLines != 2 {
		t.Fatalf("multiline hint geometry = container height %v text height %v max lines %v", multi.Height, text.Height, text.MaxLines)
	}
	if _, ok := single.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded); !ok {
		t.Fatal("hint text should take the remaining row width")
	}
	if single.Color.A != 36 || single.BorderColor.A != 89 || multi.Color.A != 36 || multi.BorderColor.A != 89 {
		t.Fatalf("dark hint colors = single %v/%v multi %v/%v", single.Color.A, single.BorderColor.A, multi.Color.A, multi.BorderColor.A)
	}
}
