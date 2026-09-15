package view

import (
	"reflect"
	"testing"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// Insertion paint and hit testing stay in logical units at mixed display scales and origins.
func TestQuerySuggestionInsertionPaintAndHitTesting(t *testing.T) {
	for _, scale := range []float32{1, 1.25, 1.5, 2} {
		for _, ink := range []woxui.Color{{R: 255, G: 255, B: 255, A: 255}, {A: 255}} {
			props := LauncherQueryProps{Width: 240 * scale, Height: 34 * scale, LineHeight: 34 * scale,
				CaretHeight: 30 * scale, CaretWidth: 50 * scale,
				Style: woxui.TextStyle{Size: 28 * scale},
				State: woxui.TextEditingState{Text: "gh cr repo", Selection: woxui.TextSelection{Anchor: 5, Focus: 5}},
				Lines: []LauncherQueryLine{{Text: "gh cr repo", TextWidth: 120 * scale}}, CompletionSuffix: "eated",
				CompletionInsertion: LauncherQueryCompletionInsertion{Visible: true, Line: 0, X: 50 * scale, Width: 80 * scale, Prefix: "gh cr", Remainder: " repo"},
			}
			props.Theme.QueryText = ink
			bounds := woxui.Rect{X: -1200 * scale, Y: -100 * scale, Width: props.Width, Height: props.Height}
			var actual, expected woxui.DisplayList
			launcherQueryPainter(props).(woxwidget.CaretPainter).Paint(&actual, bounds, true, false)
			ghost := ink
			ghost.A = 96
			expected.DrawText("eated", woxui.Rect{X: bounds.X + 50*scale, Y: bounds.Y, Width: 190 * scale, Height: props.LineHeight}, props.Style, ghost)
			expected.DrawText("gh cr", bounds, props.Style, ink)
			expected.DrawText(" repo", woxui.Rect{X: bounds.X + 130*scale, Y: bounds.Y, Width: 110 * scale, Height: props.LineHeight}, props.Style, ink)
			expected.DrawCaret(woxui.Rect{X: bounds.X + 50*scale, Y: bounds.Y, Width: 2, Height: props.CaretHeight}, props.Theme.Cursor, false)
			if !reflect.DeepEqual(actual, expected) {
				t.Fatalf("insertion paint differs at scale %v", scale)
			}
			for _, tc := range []struct{ x, want float32 }{{20, 20}, {50, 50}, {90, 50}, {130, 50}, {150, 70}} {
				point := launcherQueryDocumentPoint(props, woxui.Point{X: tc.x * scale, Y: 10 * scale})
				if point.X != tc.want*scale {
					t.Fatalf("hit test %v: %+v", tc, point)
				}
			}
			other := props
			other.CompletionInsertion.Remainder = " different"
			if props.Equal(other) {
				t.Fatal("insertion missing from boundary equality")
			}
		}
	}
}
