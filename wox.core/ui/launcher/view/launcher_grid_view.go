package view

import (
	"fmt"
	"math"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherGridResult contains one resolved grid result and its controller callbacks.
type LauncherGridResult struct {
	ID         string
	Revision   uint64
	Title      string
	Group      bool
	Selected   bool
	Hovered    bool
	Icon       *woxui.Image
	OnHover    func(bool) `boundary:"stable"`
	OnSelect   func()     `boundary:"stable"`
	OnActivate func()     `boundary:"stable"`
}

// Equal compares every visual dependency for one prepared grid result.
func (r LauncherGridResult) Equal(other LauncherGridResult) bool {
	return r.ID == other.ID && r.Revision == other.Revision && r.Title == other.Title && r.Group == other.Group && r.Selected == other.Selected && r.Hovered == other.Hovered && r.Icon == other.Icon
}

// LauncherGridProps contains the normalized grid geometry and resolved result visuals.
type LauncherGridProps struct {
	Revision          uint64
	Width             float32
	Height            float32
	ContentHeight     float32
	Offset            float32
	Columns           int
	ItemPadding       float32
	ItemMargin        float32
	ShowTitle         bool
	CellWidth         float32
	CellHeight        float32
	VisualWidth       float32
	VisualHeight      float32
	GroupHeaderHeight float32
	TitleHeight       float32
	DensityScale      float32
	Theme             woxcomponent.Theme
	ScrollDetached    bool
	Results           []LauncherGridResult
	OnScroll          func(float32) `boundary:"stable"`
}

// Equal compares every render dependency for the grid result section.
func (p LauncherGridProps) Equal(other LauncherGridProps) bool {
	if p.Revision != other.Revision || p.Width != other.Width || p.Height != other.Height || p.ContentHeight != other.ContentHeight || p.Offset != other.Offset || p.Columns != other.Columns || p.ItemPadding != other.ItemPadding || p.ItemMargin != other.ItemMargin || p.ShowTitle != other.ShowTitle || p.CellWidth != other.CellWidth || p.CellHeight != other.CellHeight || p.VisualWidth != other.VisualWidth || p.VisualHeight != other.VisualHeight || p.GroupHeaderHeight != other.GroupHeaderHeight || p.TitleHeight != other.TitleHeight || p.DensityScale != other.DensityScale || p.Theme != other.Theme || p.ScrollDetached != other.ScrollDetached || len(p.Results) != len(other.Results) {
		return false
	}
	for index := range p.Results {
		if !p.Results[index].Equal(other.Results[index]) {
			return false
		}
	}
	return true
}

type launcherGridResultProps struct {
	Result       LauncherGridResult
	ItemPadding  float32
	ItemMargin   float32
	ShowTitle    bool
	CellWidth    float32
	CellHeight   float32
	VisualWidth  float32
	VisualHeight float32
	TitleHeight  float32
	DensityScale float32
	Theme        woxcomponent.Theme
}

func (p launcherGridResultProps) Equal(other launcherGridResultProps) bool {
	return p.Result.Equal(other.Result) && p.ItemPadding == other.ItemPadding && p.ItemMargin == other.ItemMargin && p.ShowTitle == other.ShowTitle && p.CellWidth == other.CellWidth && p.CellHeight == other.CellHeight && p.VisualWidth == other.VisualWidth && p.VisualHeight == other.VisualHeight && p.TitleHeight == other.TitleHeight && p.DensityScale == other.DensityScale && p.Theme == other.Theme
}

// LauncherGridBoundary retains the complete grid section when its prepared props are unchanged.
func LauncherGridBoundary(props LauncherGridProps) woxwidget.Widget {
	return woxwidget.Boundary[LauncherGridProps]{
		Key: "launcher-grid-boundary", Label: "results:grid", Props: props,
		Build: func(props LauncherGridProps) woxwidget.Widget { return LauncherGridView(props) },
	}
}

// LauncherGridView builds wrapped grid rows and group headers.
func LauncherGridView(props LauncherGridProps) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0)
	for index := 0; index < len(props.Results); {
		if props.Results[index].Group {
			result := props.Results[index]
			rows = append(rows, woxwidget.Container{
				Width: props.Width - 28, Height: props.GroupHeaderHeight, Padding: woxwidget.Insets{Left: 8, Top: 9},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridHeaderFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle},
			})
			index++
			continue
		}
		cells := make([]woxwidget.Widget, 0, props.Columns)
		for len(cells) < props.Columns && index < len(props.Results) && !props.Results[index].Group {
			result := props.Results[index]
			cellProps := launcherGridResultProps{
				Result: result, ItemPadding: props.ItemPadding, ItemMargin: props.ItemMargin, ShowTitle: props.ShowTitle,
				CellWidth: props.CellWidth, CellHeight: props.CellHeight, VisualWidth: props.VisualWidth, VisualHeight: props.VisualHeight,
				TitleHeight: props.TitleHeight, DensityScale: props.DensityScale, Theme: props.Theme,
			}
			cells = append(cells, woxwidget.Boundary[launcherGridResultProps]{
				Key: woxwidget.Key("launcher-grid-result-boundary-" + result.ID), Label: "grid-result:" + result.ID, Props: cellProps,
				Build: func(cellProps launcherGridResultProps) woxwidget.Widget {
					return launcherGridResultView(cellProps.Result, LauncherGridProps{
						Revision:    cellProps.Result.Revision,
						ItemPadding: cellProps.ItemPadding, ItemMargin: cellProps.ItemMargin, ShowTitle: cellProps.ShowTitle,
						CellWidth: cellProps.CellWidth, CellHeight: cellProps.CellHeight, VisualWidth: cellProps.VisualWidth, VisualHeight: cellProps.VisualHeight,
						TitleHeight: cellProps.TitleHeight, DensityScale: cellProps.DensityScale, Theme: cellProps.Theme,
					})
				},
			})
			index++
		}
		for len(cells) < props.Columns {
			cells = append(cells, woxwidget.Painter{Width: props.CellWidth, Height: props.CellHeight})
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells})
	}
	content := woxwidget.Container{
		Width: props.Width, Height: props.ContentHeight, Padding: woxwidget.Insets{Left: 14, Right: 14},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows},
	}
	return woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "launcher-result-scroll", Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
		ThumbColor: props.Theme.ResultSubtitle, OnScroll: props.OnScroll,
	})
}

// launcherGridResultView builds one interactive grid cell.
func launcherGridResultView(result LauncherGridResult, props LauncherGridProps) woxwidget.Widget {
	frameColor := woxui.Color{}
	if result.Selected {
		frameColor = props.Theme.SelectedBackground
	} else if result.Hovered {
		frameColor = props.Theme.SelectedBackground
		frameColor.A = uint8(float32(frameColor.A)*0.25 + 0.5)
	}
	var visual woxwidget.Widget = woxwidget.Painter{Width: props.VisualWidth, Height: props.VisualHeight}
	if result.Icon != nil {
		fit := woxwidget.ImageFitCover
		if math.Abs(float64(props.VisualWidth/props.VisualHeight-1)) < 0.01 {
			fit = woxwidget.ImageFitContain
		}
		visual = woxwidget.Image{Source: result.Icon, Width: props.VisualWidth, Height: props.VisualHeight, Fit: fit}
	}
	visual = woxwidget.Container{
		Width: props.VisualWidth + props.ItemPadding*2, Height: props.VisualHeight + props.ItemPadding*2, Radius: 8, BorderColor: frameColor, BorderWidth: 4,
		Padding: woxwidget.UniformInsets(props.ItemPadding), Child: visual,
	}
	children := []woxwidget.Widget{visual}
	if props.ShowTitle {
		children = append(children, woxwidget.Container{
			Width: props.VisualWidth, Height: props.TitleHeight, Padding: woxwidget.Insets{Top: 4},
			Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridItemTitleFontSize, props.DensityScale)}, Color: props.Theme.ResultTitle},
		})
	}
	return woxwidget.Gesture{
		ID: fmt.Sprintf("grid-result-%s", result.ID),
		OnHover: func(inside bool) {
			if result.OnHover != nil {
				result.OnHover(inside)
			}
		},
		OnTap: result.OnSelect,
		OnDoubleTap: func() {
			if result.OnSelect != nil {
				result.OnSelect()
			}
			if result.OnActivate != nil {
				result.OnActivate()
			}
		},
		Child: woxwidget.Container{
			Width: props.CellWidth, Height: props.CellHeight, Padding: woxwidget.UniformInsets(props.ItemMargin),
			Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
		},
	}
}
