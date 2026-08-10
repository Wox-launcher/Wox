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
	ID                 string
	Title              string
	Group              bool
	Selected           bool
	Hovered            bool
	Icon               *woxui.Image
	OnHover            func(bool) `boundary:"stable"`
	OnSelect           func()     `boundary:"stable"`
	OnSecondaryTapDown func()     `boundary:"stable"`
	OnActivate         func()     `boundary:"stable"`
	OnDragStart        func()     `boundary:"stable"`
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
	DensityScale      float32
	Theme             woxcomponent.Theme
	ScrollDetached    bool
	Results           []LauncherGridResult
	OnScroll          func(float32) `boundary:"stable"`
}

type launcherGridFrameProps struct {
	Width       float32
	Height      float32
	BorderColor woxui.Color
}

func (p launcherGridFrameProps) Equal(other launcherGridFrameProps) bool {
	return p == other
}

type launcherGridIconProps struct {
	Image  *woxui.Image
	Width  float32
	Height float32
	Fit    woxwidget.ImageFit
}

func (p launcherGridIconProps) Equal(other launcherGridIconProps) bool {
	return p == other
}

// LauncherGridView builds wrapped grid rows and group headers.
func LauncherGridView(props LauncherGridProps) woxwidget.Widget {
	rows := make([]woxwidget.Widget, 0)
	for index := 0; index < len(props.Results); {
		if props.Results[index].Group {
			result := props.Results[index]
			titleProps := launcherResultTextProps{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridHeaderFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}
			rows = append(rows, woxwidget.Container{
				Width: props.Width - 28, Height: props.GroupHeaderHeight, Padding: woxwidget.Insets{Left: 8, Top: 9},
				Child: launcherResultTextBoundary(LauncherResultTitleBoundaryKey(result.ID), "grid-title:"+result.ID, titleProps),
			})
			index++
			continue
		}
		cells := make([]woxwidget.Widget, 0, props.Columns)
		for len(cells) < props.Columns && index < len(props.Results) && !props.Results[index].Group {
			result := props.Results[index]
			cells = append(cells, launcherGridResultView(result, props))
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
	fit := woxwidget.ImageFitCover
	if math.Abs(float64(props.VisualWidth/props.VisualHeight-1)) < 0.01 {
		fit = woxwidget.ImageFitContain
	}
	iconProps := launcherGridIconProps{Image: result.Icon, Width: props.VisualWidth, Height: props.VisualHeight, Fit: fit}
	icon := woxwidget.Boundary[launcherGridIconProps]{
		Key: LauncherResultIconBoundaryKey(result.ID), Label: "grid-icon:" + result.ID, Props: iconProps,
		Build: func(props launcherGridIconProps) woxwidget.Widget {
			if props.Image == nil {
				return woxwidget.Painter{Width: props.Width, Height: props.Height}
			}
			return woxwidget.Image{Source: props.Image, Width: props.Width, Height: props.Height, Fit: props.Fit}
		},
	}
	visualWidth := props.VisualWidth + props.ItemPadding*2
	visualHeight := props.VisualHeight + props.ItemPadding*2
	frameProps := launcherGridFrameProps{Width: visualWidth, Height: visualHeight, BorderColor: frameColor}
	visual := woxwidget.Stack{Width: visualWidth, Height: visualHeight, Children: []woxwidget.StackChild{
		{Child: woxwidget.Boundary[launcherGridFrameProps]{
			Key: LauncherResultBackgroundBoundaryKey(result.ID), Label: "grid-frame:" + result.ID, Props: frameProps,
			Build: func(props launcherGridFrameProps) woxwidget.Widget {
				return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 8, BorderColor: props.BorderColor, BorderWidth: 4}
			},
		}},
		{Child: woxwidget.Container{Width: visualWidth, Height: visualHeight, Padding: woxwidget.UniformInsets(props.ItemPadding), Child: icon}},
	}}
	children := []woxwidget.Widget{visual}
	if props.ShowTitle {
		titleProps := launcherResultTextProps{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridItemTitleFontSize, props.DensityScale)}, Color: props.Theme.ResultTitle}
		children = append(children, woxwidget.Container{Width: props.VisualWidth, Height: props.TitleHeight, Padding: woxwidget.Insets{Top: 4}, Child: launcherResultTextBoundary(LauncherResultTitleBoundaryKey(result.ID), "grid-title:"+result.ID, titleProps)})
	}
	return woxwidget.Gesture{
		ID: fmt.Sprintf("grid-result-%s", result.ID),
		OnHover: func(inside bool) {
			if result.OnHover != nil {
				result.OnHover(inside)
			}
		},
		OnTap:              result.OnSelect,
		OnSecondaryTapDown: result.OnSecondaryTapDown,
		OnDragStart:        result.OnDragStart,
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
