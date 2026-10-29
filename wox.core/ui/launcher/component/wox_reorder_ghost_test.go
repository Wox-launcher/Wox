package component

import (
	"testing"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestReorderGhostShowsLabelWithSharedGrip(t *testing.T) {
	theme := ControlTheme{Background: woxui.Color{R: 20, A: 255}, BodyText: woxui.Color{R: 240, A: 255}}
	ghost := ReorderGhost("Dragged note", 200, 16, 1, woxui.TextStyle{Size: 13}, theme.BodyText, theme).(woxwidget.Container)
	row := ghost.Child.(woxwidget.Flex)
	label := row.Children[1].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if ghost.Color.A != 230 || label.Value != "Dragged note" || label.MaxLines != 1 {
		t.Fatalf("unexpected reorder preview: %+v, %+v", ghost, label)
	}
}
