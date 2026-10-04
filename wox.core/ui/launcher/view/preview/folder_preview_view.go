package preview

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	folderPreviewPaddingX      = float32(18)
	folderPreviewPaddingTop    = float32(16)
	folderPreviewPaddingBottom = float32(16)
	folderPreviewRowIconSize   = float32(20)
	folderPreviewRowHeight     = float32(32)
	folderPreviewSizeWidth     = float32(64)
)

// FolderPreviewEntry is one child row in the folder preview peek.
type FolderPreviewEntry struct {
	Name  string
	IsDir bool
	Size  string
}

// FolderPreviewProps contains the resolved child peek and its display state.
type FolderPreviewProps struct {
	Width      float32
	Height     float32
	Theme      woxcomponent.Theme
	Path       string
	FolderIcon *woxui.Image
	FileIcon   *woxui.Image
	Entries    []FolderPreviewEntry
	More       string
	Empty      string
	Error      string
}

// FolderPreviewView shows the folder contents without repeating the selected result identity.
func FolderPreviewView(props FolderPreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-folderPreviewPaddingX*2)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height,
		Padding: woxwidget.Insets{Left: folderPreviewPaddingX, Top: folderPreviewPaddingTop, Right: folderPreviewPaddingX, Bottom: folderPreviewPaddingBottom},
		Child:   folderPreviewBody(props, innerWidth),
	}
}

// folderPreviewBody fills the remaining preview height with empty, error, or a scrollable peek.
func folderPreviewBody(props FolderPreviewProps, width float32) woxwidget.Widget {
	if props.Error != "" {
		return woxwidget.TextBlock{Value: props.Error, Width: width, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ErrorText}
	}
	if len(props.Entries) == 0 {
		return woxwidget.TextBlock{Value: props.Empty, Width: width, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle}
	}
	scroll := woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("folder-preview-" + props.Path), Width: width, FillWidth: true, FillHeight: true,
		ReserveScrollbarSpace: true,
		ThumbColor:            props.Theme.PreviewPropertyContent, Theme: props.Theme.Controls,
	}
	scroll.Content = woxwidget.Flex{Axis: woxwidget.Vertical, Children: folderPreviewEntryRows(props, scroll.ContentViewportWidth())}
	return woxcomponent.WoxScrollView(scroll)
}

func folderPreviewEntryRows(props FolderPreviewProps, width float32) []woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0, len(props.Entries)+1)
	for _, entry := range props.Entries {
		rows = append(rows, folderPreviewEntryRow(entry, width, props))
	}
	if props.More != "" {
		rows = append(rows, woxwidget.Container{Width: width, Height: folderPreviewRowHeight, Child: woxwidget.Align{
			Height: folderPreviewRowHeight, Vertical: 0.5,
			Child: woxwidget.Text{Value: props.More, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle},
		}})
	}
	return rows
}

// folderPreviewEntryRow is a dense identity row so a dozen children still fit the pane.
func folderPreviewEntryRow(entry FolderPreviewEntry, width float32, props FolderPreviewProps) woxwidget.Widget {
	icon := props.FileIcon
	if entry.IsDir {
		icon = props.FolderIcon
	}
	sizeWidth := float32(0)
	var size woxwidget.Widget
	if entry.Size != "" {
		sizeWidth = folderPreviewSizeWidth
		size = woxwidget.Align{Width: sizeWidth, Height: folderPreviewRowHeight, Horizontal: 1, Vertical: 0.5, Child: woxwidget.Text{
			Value: entry.Size, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		}}
	}
	nameWidth := max(float32(0), width-folderPreviewRowIconSize-8-sizeWidth)
	if sizeWidth > 0 {
		nameWidth = max(float32(0), nameWidth-8)
	}
	children := []woxwidget.Widget{
		folderPreviewCatalogIcon(icon, folderPreviewRowIconSize, props.Theme.QueryBackground),
		woxwidget.Expanded{Child: woxwidget.Align{Height: folderPreviewRowHeight, Vertical: 0.5, Child: woxwidget.TextBlock{
			Value: entry.Name, Width: nameWidth, MaxLines: 1, Style: woxui.TextStyle{Size: 13},
			LineHeight: 18, AlignmentY: 0.5, Color: props.Theme.PreviewText,
		}}},
	}
	if size != nil {
		children = append(children, size)
	}
	return woxwidget.Container{Width: width, Height: folderPreviewRowHeight, Child: woxwidget.Flex{
		Axis: woxwidget.Horizontal, Gap: 8, CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: children,
	}}
}

// folderPreviewCatalogIcon paints a brand-colored catalog SVG without chrome tinting.
func folderPreviewCatalogIcon(image *woxui.Image, size float32, fallback woxui.Color) woxwidget.Widget {
	if image != nil {
		return woxwidget.Image{Source: image, Width: size, Height: size}
	}
	return woxwidget.Container{Width: size, Height: size, Radius: 6, Color: fallback}
}
