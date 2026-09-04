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
	Loading            bool
	QuickSelectNumber  string
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
	TailColor         woxui.Color
	SelectedTailColor woxui.Color
	ScrollDetached    bool
	Complete          bool
	ExtentRevision    uint64
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
	Image   *woxui.Image
	Width   float32
	Height  float32
	Fit     woxwidget.ImageFit
	Loading bool
	Color   woxui.Color
}

func (p launcherGridIconProps) Equal(other launcherGridIconProps) bool {
	return p == other
}

// launcherGridVisualRow is one painted row: a group header or a wrapped cell line.
type launcherGridVisualRow struct {
	isHeader bool
	header   LauncherGridResult
	cells    []LauncherGridResult
}

// launcherGridVisualRows folds results into header and cell rows so LazyList can virtualize mixed extents.
func launcherGridVisualRows(results []LauncherGridResult, columns int) []launcherGridVisualRow {
	if columns <= 0 {
		columns = 1
	}
	rows := make([]launcherGridVisualRow, 0)
	for index := 0; index < len(results); {
		if results[index].Group {
			rows = append(rows, launcherGridVisualRow{isHeader: true, header: results[index]})
			index++
			continue
		}
		rowStart := index
		for index < len(results) && !results[index].Group && index-rowStart < columns {
			index++
		}
		rows = append(rows, launcherGridVisualRow{cells: results[rowStart:index]})
	}
	return rows
}

// LauncherGridView builds wrapped grid rows and group headers.
func LauncherGridView(props LauncherGridProps) woxwidget.Widget {
	rows := launcherGridVisualRows(props.Results, props.Columns)
	innerWidth := max(float32(0), props.Width-28)
	content := woxwidget.Container{
		Width: props.Width, Height: props.ContentHeight, Padding: woxwidget.Insets{Left: 14, Right: 14},
		Child: woxwidget.LazyList{
			Key: "launcher-grid-rows", Width: innerWidth, Viewport: props.Height, ItemCount: len(rows),
			ExtentRevision: props.ExtentRevision,
			ItemExtentAt: func(index int) float32 {
				if rows[index].isHeader {
					return props.GroupHeaderHeight
				}
				return props.CellHeight
			},
			ItemKey: func(index int) woxwidget.Key {
				if rows[index].isHeader {
					return woxwidget.Key("grid-row-header:" + rows[index].header.ID)
				}
				return woxwidget.Key("grid-row-cells:" + rows[index].cells[0].ID)
			},
			ItemBuilder: func(index int) woxwidget.Widget { return launcherGridVisualRowView(rows[index], props) },
		},
	}
	return WrapLauncherResultsStatus(props.Complete, woxcomponent.WoxScrollView(woxcomponent.ScrollViewProps{
		Key: "launcher-result-scroll", Content: content, Width: props.Width, Height: props.Height, ContentHeight: props.ContentHeight, Offset: props.Offset,
		ThumbColor: props.Theme.ResultTitle, OnScroll: props.OnScroll,
	}))
}

// launcherGridVisualRowView builds one header or one wrapped cell row.
func launcherGridVisualRowView(row launcherGridVisualRow, props LauncherGridProps) woxwidget.Widget {
	if row.isHeader {
		result := row.header
		titleProps := launcherResultTextProps{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridHeaderFontSize, props.DensityScale), Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}
		return woxwidget.Container{
			Width: props.Width - 28, Height: props.GroupHeaderHeight, Padding: woxwidget.Insets{Left: 8},
			Child: woxwidget.Align{Height: props.GroupHeaderHeight, Vertical: 0.5, Child: launcherResultTextBoundary(LauncherResultTitleBoundaryKey(result.ID), "grid-title:"+result.ID, titleProps)},
		}
	}
	cells := make([]woxwidget.Widget, 0, props.Columns)
	for _, result := range row.cells {
		cells = append(cells, launcherGridResultView(result, props))
	}
	for len(cells) < props.Columns {
		cells = append(cells, woxwidget.Painter{Width: props.CellWidth, Height: props.CellHeight})
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}
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
	iconProps := launcherGridIconProps{Image: result.Icon, Width: props.VisualWidth, Height: props.VisualHeight, Fit: fit, Loading: result.Loading, Color: props.Theme.Cursor}
	icon := woxwidget.Boundary[launcherGridIconProps]{
		Key: LauncherResultIconBoundaryKey(result.ID), Label: "grid-icon:" + result.ID, Props: iconProps,
		Build: func(props launcherGridIconProps) woxwidget.Widget {
			if props.Loading {
				spinner := min(props.Width, props.Height) * 0.55
				return woxwidget.Align{Width: props.Width, Height: props.Height, Horizontal: 0.5, Vertical: 0.5, Child: woxcomponent.WoxLoadingIndicator(spinner, props.Color)}
			}
			if props.Image == nil {
				return woxwidget.Painter{Width: props.Width, Height: props.Height}
			}
			return woxwidget.Image{Source: props.Image, Width: props.Width, Height: props.Height, Fit: props.Fit}
		},
	}
	visualWidth := props.VisualWidth + props.ItemPadding*2
	visualHeight := props.VisualHeight + props.ItemPadding*2
	frameProps := launcherGridFrameProps{Width: visualWidth, Height: visualHeight, BorderColor: frameColor}
	visualChildren := []woxwidget.StackChild{
		{Child: woxwidget.Boundary[launcherGridFrameProps]{
			Key: LauncherResultBackgroundBoundaryKey(result.ID), Label: "grid-frame:" + result.ID, Props: frameProps,
			Build: func(props launcherGridFrameProps) woxwidget.Widget {
				return woxwidget.Container{Width: props.Width, Height: props.Height, Radius: 8, BorderColor: props.BorderColor, BorderWidth: 4}
			},
		}},
		{Child: woxwidget.Container{Width: visualWidth, Height: visualHeight, Padding: woxwidget.UniformInsets(props.ItemPadding), Child: icon}},
	}
	if result.QuickSelectNumber != "" {
		fill := props.TailColor
		if result.Selected {
			fill = props.SelectedTailColor
		}
		badge := launcherQuickSelectBadge(result.QuickSelectNumber, props.DensityScale, fill, props.Theme.Background)
		inset := scaledLauncherSize(4, props.DensityScale)
		visualChildren = append(visualChildren, woxwidget.StackChild{Child: woxwidget.Align{
			Width: visualWidth, Height: visualHeight, Horizontal: 1, Vertical: 0,
			Child: woxwidget.Container{Padding: woxwidget.Insets{Top: inset, Right: inset}, Child: badge},
		}})
	}
	visual := woxwidget.Stack{Width: visualWidth, Height: visualHeight, Children: visualChildren}
	children := []woxwidget.Widget{visual}
	if props.ShowTitle {
		titleProps := launcherResultTextProps{Value: result.Title, Style: woxui.TextStyle{Size: scaledLauncherSize(woxcomponent.GridItemTitleFontSize, props.DensityScale)}, Color: props.Theme.ResultTitle}
		children = append(children, woxwidget.Container{Width: props.VisualWidth, Height: props.TitleHeight, Padding: woxwidget.Insets{Top: 4}, Child: launcherResultTextBoundary(LauncherResultTitleBoundaryKey(result.ID), "grid-title:"+result.ID, titleProps)})
	}
	return woxwidget.Semantics{
		Key: woxwidget.Key(fmt.Sprintf("launcher-result-key-%s", result.ID)), AutomationID: "launcher.result." + result.ID, Role: woxui.AccessibilityRoleListItem,
		Label: result.Title, Value: result.QuickSelectNumber, Selected: result.Selected,
		Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		OnAction: func(action woxui.AccessibilityAction, _ string) error {
			if action == woxui.AccessibilityActionActivate {
				if result.OnSelect != nil {
					result.OnSelect()
				}
				if result.OnActivate != nil {
					result.OnActivate()
				}
			}
			return nil
		},
		Child: woxwidget.Gesture{
			ID: fmt.Sprintf("grid-result-%s", result.ID),
			OnHover: func(inside bool) {
				if result.OnHover != nil {
					result.OnHover(inside)
				}
			},
			OnTap: result.OnSelect,
			OnSecondaryTapDown: func(woxui.Point) {
				if result.OnSecondaryTapDown != nil {
					result.OnSecondaryTapDown()
				}
			},
			OnDragStart: result.OnDragStart,
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
		},
	}
}
