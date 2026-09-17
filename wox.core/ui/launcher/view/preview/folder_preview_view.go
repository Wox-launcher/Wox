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
	folderPreviewSectionGap    = float32(16)
	folderPreviewIconSize      = float32(32)
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

// FolderPreviewProps contains the resolved folder identity, counts, and child peek.
type FolderPreviewProps struct {
	Width      float32
	Height     float32
	Theme      woxcomponent.Theme
	Path       string
	Name       string
	Metadata   string
	Icon       *woxui.Image
	FolderIcon *woxui.Image
	FileIcon   *woxui.Image
	Entries    []FolderPreviewEntry
	More       string
	Empty      string
	Error      string
}

// FolderPreviewView shows a top-aligned folder identity, cheap metadata, and a contents peek.
func FolderPreviewView(props FolderPreviewProps) woxwidget.Widget {
	innerWidth := max(float32(0), props.Width-folderPreviewPaddingX*2)
	return woxwidget.Container{
		Width: props.Width, Height: props.Height,
		Padding: woxwidget.Insets{Left: folderPreviewPaddingX, Top: folderPreviewPaddingTop, Right: folderPreviewPaddingX, Bottom: folderPreviewPaddingBottom},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: folderPreviewSectionGap, Children: []woxwidget.Widget{
			folderPreviewHeader(props, innerWidth),
			woxwidget.Expanded{Child: folderPreviewBody(props, innerWidth)},
		}},
	}
}

// folderPreviewHeader uses the catalog folder mark so the preview matches the result row.
func folderPreviewHeader(props FolderPreviewProps, width float32) woxwidget.Widget {
	textWidth := max(float32(0), width-folderPreviewIconSize-12)
	lines := make([]woxwidget.Widget, 0, 3)
	if props.Name != "" {
		lines = append(lines, woxwidget.TextBlock{
			Value: props.Name, Width: textWidth, MaxLines: 1, Style: woxui.TextStyle{Size: 15, Weight: woxui.FontWeightSemibold},
			LineHeight: 20, Color: props.Theme.PreviewText,
		})
	}
	if props.Path != "" && props.Path != props.Name {
		lines = append(lines, woxwidget.TextBlock{
			Value: props.Path, Width: textWidth, MaxLines: 2, Style: woxui.TextStyle{Size: 12},
			LineHeight: 16, Color: props.Theme.ResultSubtitle,
		})
	}
	if props.Metadata != "" {
		lines = append(lines, woxwidget.TextBlock{
			Value: props.Metadata, Width: textWidth, MaxLines: 2, Style: woxui.TextStyle{Size: 12},
			LineHeight: 16, Color: props.Theme.PreviewPropertyContent,
		})
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, CrossAxisAlignment: woxwidget.CrossAxisStart, Children: []woxwidget.Widget{
		folderPreviewCatalogIcon(props.Icon, folderPreviewIconSize, props.Theme.QueryBackground),
		woxwidget.Expanded{Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 4, Children: lines}},
	}}
}

// folderPreviewBody fills the remaining preview height with empty, error, or a scrollable peek.
func folderPreviewBody(props FolderPreviewProps, width float32) woxwidget.Widget {
	if props.Error != "" {
		return woxwidget.TextBlock{Value: props.Error, Width: width, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ErrorText}
	}
	if len(props.Entries) == 0 {
		return woxwidget.TextBlock{Value: props.Empty, Width: width, Style: woxui.TextStyle{Size: 13}, LineHeight: 18, Color: props.Theme.ResultSubtitle}
	}
	rows := folderPreviewEntryRows(props, width)
	return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: woxwidget.Key("folder-preview-" + props.Path), FillWidth: true, FillHeight: true,
		ThumbColor: props.Theme.PreviewPropertyContent, Theme: props.Theme.Controls,
		Content: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	})
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
