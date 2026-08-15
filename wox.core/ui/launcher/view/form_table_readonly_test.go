package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestReadonlyFormTableUsesFullWidthAndCellTooltip(t *testing.T) {
	columns := []FormTableColumn{{Label: "Item"}, {Label: "Mode", Width: 220}}
	widths := formTableColumnWidthsWithOperation(columns, 800, false)
	if widths[len(widths)-1] != 0 || widths[0]+widths[1] != 800 {
		t.Fatalf("readonly widths = %v, want full width without operation column", widths)
	}

	icon := &woxui.Image{}
	cell := formTableDataCellAt(FormTableFieldProps{ID: "notes", InfoIcon: icon, Theme: woxcomponent.Theme{}}, 2, 1, FormTableCell{Text: "Platform sync", Tooltip: "Per platform"}, 220)
	content := cell.(woxwidget.Container).Child.(woxwidget.Flex)
	if len(content.Children) != 2 {
		t.Fatalf("tooltip cell children = %d, want text and shared tooltip trigger", len(content.Children))
	}
	text := content.Children[0].(woxwidget.TextBlock)
	if !text.ShrinkWrap {
		t.Fatal("tooltip cell text should shrink to its content so the trigger stays adjacent")
	}
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("tooltip cell alignment = %v, want vertical center", content.CrossAxisAlignment)
	}
	tooltip := content.Children[1].(woxwidget.Gesture)
	if tooltip.ID != "notes-row-2-cell-1-tooltip" || tooltip.OnHoverAt == nil {
		t.Fatal("readonly cell should expose the shared table tooltip contract")
	}
}

func TestFormTableCellSupportsCustomContent(t *testing.T) {
	child := woxwidget.Text{Value: "Restore"}
	cell := formTableDataCell(FormTableFieldProps{Theme: woxcomponent.Theme{}}, FormTableCell{Child: child}, 220).(woxwidget.Container)
	content, ok := cell.Child.(woxwidget.Align)
	if !ok || content.Width != 206 || content.Height != tableSurfaceRowHeight || content.Vertical != 0.5 || cell.Padding.Top != 0 {
		t.Fatalf("custom table cell alignment = %#v with padding top %.0f, want a full-height centered slot", cell.Child, cell.Padding.Top)
	}
}

func TestFormTableCellUsesRequestedIconSize(t *testing.T) {
	cell := formTableDataCell(FormTableFieldProps{Theme: woxcomponent.Theme{}}, FormTableCell{Icon: &woxui.Image{}, IconSize: 24}, 120).(woxwidget.Container)
	content := cell.Child.(woxwidget.Flex)
	icon := content.Children[0].(woxwidget.Image)

	if icon.Width != 24 || icon.Height != 24 || cell.Padding.Top != 6 {
		t.Fatalf("24px table icon geometry = %vx%v with top padding %v", icon.Width, icon.Height, cell.Padding.Top)
	}
}
