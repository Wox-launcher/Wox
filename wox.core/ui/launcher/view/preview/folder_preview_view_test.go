package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestFolderPreviewViewIsTopAlignedWithIdentityAndEntries(t *testing.T) {
	view := FolderPreviewView(FolderPreviewProps{
		Width: 320, Height: 280, Name: "Droppy", Path: `C:\Users\qianl\Droppy`,
		Metadata: "2026-09-17 16:30:58  ·  2 files",
		Entries: []FolderPreviewEntry{
			{Name: "projects", IsDir: true},
			{Name: "src", IsDir: true},
			{Name: "readme.md", Size: "2.1 KB"},
		},
		More:  "1 more items not shown",
		Theme: folderPreviewTestTheme(),
	}).(woxwidget.Container)
	if view.Padding.Left != 18 || view.Padding.Top != 16 {
		t.Fatalf("folder preview padding = %#v, want the large-file preview inset", view.Padding)
	}
	column := view.Child.(woxwidget.Flex)
	if column.Axis != woxwidget.Vertical || column.Gap != 16 || len(column.Children) != 2 {
		t.Fatalf("column = %#v, want header and body", column)
	}
	header := column.Children[0].(woxwidget.Flex)
	if header.CrossAxisAlignment != woxwidget.CrossAxisStart || header.Gap != 12 {
		t.Fatalf("header = %#v, want a catalog icon beside the identity stack", header)
	}
	icon := header.Children[0].(woxwidget.Container)
	if icon.Width != 32 || icon.Height != 32 {
		t.Fatalf("header icon = %#v, want the 32-unit catalog folder mark", icon)
	}
	lines := header.Children[1].(woxwidget.Expanded).Child.(woxwidget.Flex).Children
	if lines[0].(woxwidget.TextBlock).Value != "Droppy" || lines[1].(woxwidget.TextBlock).Value != `C:\Users\qianl\Droppy` {
		t.Fatalf("identity = %#v", lines)
	}
	if lines[2].(woxwidget.TextBlock).Value != "2026-09-17 16:30:58  ·  2 files" {
		t.Fatalf("metadata = %#v", lines[2])
	}
	if _, ok := column.Children[1].(woxwidget.Expanded); !ok {
		t.Fatalf("body = %#v, want the contents peek to take remaining height", column.Children[1])
	}
}

func TestFolderPreviewViewShowsEmptyMessageWithoutQuoteLayout(t *testing.T) {
	view := FolderPreviewView(FolderPreviewProps{
		Width: 280, Height: 180, Name: "Empty", Path: "/tmp/empty",
		Empty: "This folder is empty", Theme: folderPreviewTestTheme(),
	}).(woxwidget.Container)
	body := view.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.TextBlock)
	if body.Value != "This folder is empty" {
		t.Fatalf("empty body = %#v", body)
	}
}

func TestFolderPreviewHeaderUsesCatalogImageWhenProvided(t *testing.T) {
	image := &woxui.Image{Width: 32, Height: 32}
	header := folderPreviewHeader(FolderPreviewProps{
		Name: "Droppy", Icon: image, Theme: folderPreviewTestTheme(),
	}, 240).(woxwidget.Flex)
	drawn := header.Children[0].(woxwidget.Image)
	if drawn.Source != image || drawn.Width != 32 {
		t.Fatalf("header icon = %#v, want the decoded plugin.folder image", drawn)
	}
}

func TestFolderPreviewEntryRowsKeepDenseFolderThenFileOrder(t *testing.T) {
	theme := folderPreviewTestTheme()
	rows := folderPreviewEntryRows(FolderPreviewProps{
		Entries: []FolderPreviewEntry{
			{Name: "src", IsDir: true},
			{Name: "notes.txt", Size: "840 B"},
		},
		More:  "1 more items not shown",
		Theme: theme,
	}, 240)
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want two entries and a more line", len(rows))
	}
	first := rows[0].(woxwidget.Container)
	if first.Height != 32 {
		t.Fatalf("row height = %.0f, want a 32-unit peek row", first.Height)
	}
	name := first.Child.(woxwidget.Flex).Children[1].(woxwidget.Expanded).Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
	if name.Value != "src" {
		t.Fatalf("first entry = %#v, want the folder name", name)
	}
	if rows[2].(woxwidget.Container).Child.(woxwidget.Align).Child.(woxwidget.Text).Value != "1 more items not shown" {
		t.Fatalf("more = %#v", rows[2])
	}
}

func folderPreviewTestTheme() woxcomponent.Theme {
	return woxcomponent.Theme{
		PreviewText:            woxui.Color{R: 240, G: 244, B: 248, A: 255},
		ResultSubtitle:         woxui.Color{R: 160, G: 168, B: 176, A: 255},
		PreviewPropertyTitle:   woxui.Color{A: 255},
		PreviewPropertyContent: woxui.Color{A: 255},
		QueryBackground:        woxui.Color{R: 40, G: 44, B: 50, A: 255},
		ErrorText:              woxui.Color{R: 220, G: 80, B: 80, A: 255},
	}
}
