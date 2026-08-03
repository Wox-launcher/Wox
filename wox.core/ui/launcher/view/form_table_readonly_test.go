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
	tooltip := content.Children[1].(woxwidget.Gesture)
	if tooltip.ID != "notes-row-2-cell-1-tooltip" || tooltip.OnHoverAt == nil {
		t.Fatal("readonly cell should expose the shared table tooltip contract")
	}
}

func TestFormTableCellSupportsCustomContent(t *testing.T) {
	child := woxwidget.Text{Value: "Restore"}
	cell := formTableDataCell(FormTableFieldProps{Theme: woxcomponent.Theme{}}, FormTableCell{Child: child}, 220).(woxwidget.Container)
	if cell.Child != child || cell.Padding.Top != 6 {
		t.Fatal("custom table cell should render its child with control padding")
	}
}
