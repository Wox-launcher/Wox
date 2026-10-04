package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestFolderPreviewViewStartsWithContents guards against reintroducing the duplicate header.
func TestFolderPreviewViewStartsWithContents(t *testing.T) {
	view := FolderPreviewView(FolderPreviewProps{
		Width: 320, Height: 280, Path: "/tmp/folder",
		Entries: []FolderPreviewEntry{{Name: "src", IsDir: true}},
		Theme:   folderPreviewTestTheme(),
	}).(woxwidget.Container)
	if view.Padding.Left != 18 || view.Padding.Top != 16 {
		t.Fatalf("folder preview padding = %#v", view.Padding)
	}
	if _, ok := view.Child.(woxwidget.LayoutBuilder); !ok {
		t.Fatalf("body = %T, want the scroll view directly inside the preview", view.Child)
	}
	scroll := resolvedScrollViewProps(view.Child, woxui.Size{Width: 284, Height: 248})
	row := scroll.Content.(woxwidget.Flex).Children[0].(woxwidget.Container)
	if !scroll.ReserveScrollbarSpace || row.Width != scroll.ContentViewportWidth() || row.Width >= scroll.Width {
		t.Fatalf("folder row width %v overlaps the scrollbar in viewport %v", row.Width, scroll.Width)
	}
}

// TestFolderPreviewViewEmptyAndError keeps feedback visible without the removed header.
func TestFolderPreviewViewEmptyAndError(t *testing.T) {
	for _, failed := range []bool{false, true} {
		props := FolderPreviewProps{
			Width: 280, Height: 180, Path: "/tmp/empty",
			Empty: "This folder is empty", Theme: folderPreviewTestTheme(),
		}
		want := props.Empty
		if failed {
			props.Error = "Unable to read folder"
			want = props.Error
		}
		view := FolderPreviewView(props).(woxwidget.Container)
		if body := view.Child.(woxwidget.TextBlock); body.Value != want {
			t.Fatalf("body = %#v, want %q", body, want)
		}
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
