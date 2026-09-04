package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestLargeFilePreviewViewShowsDetailsAndLoadAction(t *testing.T) {
	titleColor := woxui.Color{R: 240, G: 244, B: 248, A: 255}
	view := LargeFilePreviewView(LargeFilePreviewProps{
		Width: 320, Height: 220, Title: "Large file preview", Message: "This file is 2.0 MB.",
		Properties: []LargeFilePreviewProperty{{Label: "Size", Value: "2.0 MB"}},
		Action:     "Load preview (Ctrl+L)",
		Theme: woxcomponent.Theme{
			PreviewText: titleColor, ResultTitle: titleColor,
			PreviewPropertyTitle: woxui.Color{A: 255}, PreviewPropertyContent: woxui.Color{A: 255},
		},
	}).(woxwidget.Container)
	children := view.Child.(woxwidget.Flex).Children
	if len(children) != 3 {
		t.Fatalf("children = %d, want header, message, and properties", len(children))
	}
	headerBox := children[0].(woxwidget.Container)
	if headerBox.Height != 32 {
		t.Fatalf("header height = %.0f, want 32 so the file details stay visible below the title", headerBox.Height)
	}
	header := headerBox.Child.(woxwidget.Flex)
	if header.Axis != woxwidget.Horizontal || header.CrossAxisAlignment != woxwidget.CrossAxisCenter || len(header.Children) != 2 {
		t.Fatalf("header = %#v, want title on the left and the load button on the right", header)
	}
	if header.Children[0].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.Text).Value != "Large file preview" {
		t.Fatalf("title = %#v", header.Children[0])
	}
	if header.Children[1].(woxwidget.Semantics).Label != "Load preview (Ctrl+L)" {
		t.Fatalf("load action = %#v, want the shortcut inside the trailing header button", header.Children[1])
	}
	if children[1].(woxwidget.TextBlock).Value != "This file is 2.0 MB." {
		t.Fatalf("message = %#v", children[1])
	}
}

func TestLargeFilePreviewViewOmitsLoadActionWhenTooLarge(t *testing.T) {
	view := LargeFilePreviewView(LargeFilePreviewProps{
		Width: 320, Height: 180, Title: "File too big to preview, current size: 64.0 MB",
		Properties: []LargeFilePreviewProperty{{Label: "Type", Value: "PDF"}},
		Theme:      woxcomponent.Theme{PreviewText: woxui.Color{A: 255}},
	}).(woxwidget.Container)
	children := view.Child.(woxwidget.Flex).Children
	if len(children) != 2 {
		t.Fatalf("children = %d, want title and properties without a load action", len(children))
	}
}
