package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestReadonlyFormTableRowDisablesOperationActions(t *testing.T) {
	icon := &woxui.Image{}
	disabledIcon := &woxui.Image{}
	cell := formTableOperationCell(FormTableFieldProps{
		ID: "roots", EditLabel: "Edit", CloneLabel: "Clone", DeleteLabel: "Delete",
		EditIcon: icon, CloneIcon: icon, DeleteIcon: icon,
		DisabledEditIcon: disabledIcon, DisabledCloneIcon: disabledIcon, DisabledDeleteIcon: disabledIcon,
		Theme: woxcomponent.Theme{},
	}, FormTableRow{Index: -1, ReadOnly: true}, 130, false).(woxwidget.Container)
	actions := cell.Child.(woxwidget.Align).Child.(woxwidget.Flex)
	if len(actions.Children) != 3 {
		t.Fatalf("readonly operation count = %d, want the same edit, clone, and delete buttons", len(actions.Children))
	}
	for index, action := range actions.Children {
		button := formTableOperationIconButton(action)
		glyph, _ := button.Icon.(woxwidget.Image)
		if !button.Disabled || button.OnTap != nil || button.HoverBackground.A != 0 || glyph.Source != disabledIcon {
			t.Fatalf("readonly action %d = %+v, want a visible faded disabled icon button", index, button)
		}
	}
}

func TestReadonlyFormTableGivesLeftoverWidthToFlexibleColumn(t *testing.T) {
	columns := []FormTableColumn{{Label: "Name", Width: 140}, {Label: "Description"}, {Label: "Enabled", Width: 50}}
	widths := formTableColumnWidthsWithOperation(columns, 800, false)
	if widths[2] > 70 {
		t.Fatalf("enabled column width = %.0f, want a compact switch column", widths[2])
	}
	if widths[1] < 500 {
		t.Fatalf("description column width = %.0f, want leftover space", widths[1])
	}

	beforeEnabled := widths[2]
	formTableExpandFlexibleColumn(columns, widths, 120)
	if widths[2] != beforeEnabled {
		t.Fatalf("enabled column grew to %.0f after leftover expansion", widths[2])
	}
}

func TestReadonlyFormTableUsesFullWidthAndCellTooltip(t *testing.T) {
	columns := []FormTableColumn{{Label: "Item"}, {Label: "Mode", Width: 220}}
	widths := formTableColumnWidthsWithOperation(columns, 800, false)
	if widths[len(widths)-1] != 0 || widths[0]+widths[1] != 800 {
		t.Fatalf("readonly widths = %v, want full width without operation column", widths)
	}

	icon := &woxui.Image{}
	cell := formTableDataCellAt(FormTableFieldProps{ID: "notes", InfoIcon: icon, Theme: woxcomponent.Theme{}}, 2, 1, FormTableCell{Text: "Platform sync", Tooltip: "Per platform"}, 220, false)
	alignment := cell.(woxwidget.Container).Child.(woxwidget.Align)
	content := alignment.Child.(woxwidget.Flex)
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
	if alignment.Height != tableSurfaceRowHeight || alignment.Vertical != 0.5 {
		t.Fatalf("tooltip cell slot = %#v, want full-height vertical center", alignment)
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
	alignment := cell.Child.(woxwidget.Align)
	content := alignment.Child.(woxwidget.Flex)
	icon := content.Children[0].(woxwidget.Image)

	if icon.Width != 24 || icon.Height != 24 || cell.Padding.Top != 0 || alignment.Height != tableSurfaceRowHeight || alignment.Vertical != 0.5 {
		t.Fatalf("24px table icon geometry = %vx%v with alignment %#v", icon.Width, icon.Height, alignment)
	}
}

func TestFormTableListRowCentersLabel(t *testing.T) {
	list := FormTableList(FormTableListProps{Width: 240, Height: 180, Rows: []string{"shortcut"}, Theme: woxcomponent.Theme{}}).(woxwidget.Flex)
	scroll := list.Children[0].(woxwidget.Stateful).Widget.(woxcomponent.ScrollViewProps)
	row := scroll.Content.(woxwidget.Flex).Children[0].(woxwidget.Gesture).Child.(woxwidget.Container)
	if row.Padding.Top != 0 || row.Padding.Bottom != 0 {
		t.Fatalf("list row padding = %#v, want horizontal insets only", row.Padding)
	}
	alignment, ok := row.Child.(woxwidget.Align)
	if !ok || alignment.Height != formTableListRowHeight || alignment.Vertical != 0.5 {
		t.Fatalf("list row alignment = %#v, want a full-height centered slot", row.Child)
	}
}

func TestFormTableHeaderCellCentersLabel(t *testing.T) {
	icon := &woxui.Image{}
	cell := formTableHeaderCell(FormTableFieldProps{ID: "commands", InfoIcon: icon, Theme: woxcomponent.Theme{}}, FormTableColumn{Label: "快捷键", Tooltip: "alias tip"}, 160, 0).(woxwidget.Container)
	if cell.Padding.Top != 0 || cell.Padding.Bottom != 0 {
		t.Fatalf("header padding = %#v, want horizontal insets only", cell.Padding)
	}
	alignment, ok := cell.Child.(woxwidget.Align)
	if !ok || alignment.Height != tableSurfaceHeaderHeight || alignment.Vertical != 0.5 {
		t.Fatalf("header alignment = %#v, want a full-height centered slot", cell.Child)
	}
	content := alignment.Child.(woxwidget.Flex)
	if content.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("header row alignment = %v, want vertical center", content.CrossAxisAlignment)
	}
	label := content.Children[0].(woxwidget.TextBlock)
	if label.Height != 18 || label.LineHeight != 18 || label.AlignmentY != 0.5 {
		t.Fatalf("header label slot = height %v line height %v alignment %v, want an 18px optically centered slot", label.Height, label.LineHeight, label.AlignmentY)
	}
}
