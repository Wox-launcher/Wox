package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// LauncherGridResult contains one resolved grid result and its controller callbacks.
type LauncherGridResult struct {
	ID         string
	Title      string
	Group      bool
	Selected   bool
	Hovered    bool
	Icon       *woxui.Image
	OnHover    func(bool)
	OnSelect   func()
	OnActivate func()
}

// LauncherGridProps contains the normalized grid geometry and resolved result visuals.
type LauncherGridProps struct {
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
	Theme             woxcomponent.Theme
	Results           []LauncherGridResult
	OnScroll          func(float32)
}

// LauncherGridView builds wrapped grid rows and group headers.
func LauncherGridView(props LauncherGridProps) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0)
	for index := 0; index < len(props.Results); {
		if props.Results[index].Group {
			result := props.Results[index]
			rows = append(rows, woxwidget.Container{
				Width: props.Width - 28, Height: props.GroupHeaderHeight, Padding: woxwidget.Insets{Left: 8, Top: 9},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle},
			})
			index++
			continue
		}
		cells := make([]woxwidget.Widget, 0, props.Columns)
		for len(cells) < props.Columns && index < len(props.Results) && !props.Results[index].Group {
			cells = append(cells, launcherGridResultView(props.Results[index], props))
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
	return launcherResultScrollView(launcherResultScrollProps{
		Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
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
		visual = woxwidget.Image{Source: result.Icon, Width: props.VisualWidth, Height: props.VisualHeight}
	}
	visual = woxwidget.Container{
		Width: props.VisualWidth + props.ItemPadding*2, Height: props.VisualHeight + props.ItemPadding*2, Radius: 8, Color: frameColor,
		Padding: woxwidget.UniformInsets(props.ItemPadding), Child: visual,
	}
	children := []woxwidget.Widget{visual}
	if props.ShowTitle {
		children = append(children, woxwidget.Container{
			Width: props.VisualWidth, Height: props.TitleHeight, Padding: woxwidget.Insets{Top: 4},
			Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle},
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
