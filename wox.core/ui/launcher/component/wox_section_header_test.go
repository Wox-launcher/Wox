package component

import (
	"testing"

	woxwidget "wox/ui/widget"
)

func TestWoxSectionHeaderAllocatesRemainingWidthToTitle(t *testing.T) {
	action := woxwidget.Container{Width: 40, Height: 32}
	header := WoxSectionHeader(SectionHeaderProps{Label: "General", Width: 300, Action: action, ActionWidth: 40}).(woxwidget.Container)
	row := header.Child.(woxwidget.Flex).Children[1].(woxwidget.Container).Child.(woxwidget.Flex)

	title, ok := row.Children[0].(woxwidget.Expanded)
	if !ok {
		t.Fatalf("section title slot = %T, want Expanded", row.Children[0])
	}
	alignment, ok := title.Child.(woxwidget.Align)
	if !ok || alignment.Height != 42 || alignment.Vertical != 0.5 {
		t.Fatalf("section title alignment = %#v, want a full-height centered slot", title.Child)
	}
	if row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("section header alignment = %v, want vertical center", row.CrossAxisAlignment)
	}
	slot, ok := row.Children[1].(woxwidget.Constrained)
	if !ok || slot.MinWidth != 40 || slot.MaxWidth != 40 || !slot.FillWidth {
		t.Fatalf("section action slot = %#v, want constrained 40px width", row.Children[1])
	}
}
