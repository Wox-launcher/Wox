package launcher

import (
	"fmt"
	"math"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

const (
	gridGroupHeaderHeight = 32
	gridTitleHeight       = 22
)

func normalizedGridLayout(layout *gridLayout) gridLayout {
	if layout == nil {
		return gridLayout{}
	}
	result := *layout
	if result.Columns <= 0 {
		result.Columns = 8
	}
	result.Columns = min(result.Columns, 16)
	result.ItemPadding = max(0, result.ItemPadding)
	result.ItemMargin = max(0, result.ItemMargin)
	if result.AspectRatio <= 0 {
		result.AspectRatio = 1
	}
	return result
}

func gridCellMetrics(width float32, layout gridLayout) (float32, float32, float32, float32) {
	contentWidth := max(float32(1), width-28)
	cellWidth := contentWidth / float32(layout.Columns)
	inset := float32(layout.ItemPadding + layout.ItemMargin)
	visualWidth := max(float32(1), cellWidth-inset*2)
	visualHeight := visualWidth / float32(layout.AspectRatio)
	cellHeight := visualHeight + inset*2
	if layout.ShowTitle {
		cellHeight += gridTitleHeight
	}
	return cellWidth, cellHeight, visualWidth, visualHeight
}

func gridResultsHeight(results []queryResult, width float32, raw *gridLayout) int {
	layout := normalizedGridLayout(raw)
	if layout.Columns == 0 || len(results) == 0 {
		return 0
	}
	_, cellHeight, _, _ := gridCellMetrics(width, layout)
	height := float32(0)
	for index := 0; index < len(results); {
		if results[index].IsGroup {
			height += gridGroupHeaderHeight
			index++
			continue
		}
		count := 0
		for index < len(results) && !results[index].IsGroup && count < layout.Columns {
			index++
			count++
		}
		height += cellHeight
	}
	return int(math.Ceil(float64(height)))
}

func (a *App) buildGridResults(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	layout := normalizedGridLayout(snapshot.layout.GridLayout)
	cellWidth, cellHeight, visualWidth, visualHeight := gridCellMetrics(width, layout)
	rows := make([]woxwidget.Widget, 0)
	for index := 0; index < len(snapshot.results); {
		if snapshot.results[index].IsGroup {
			result := snapshot.results[index]
			rows = append(rows, woxwidget.Container{
				Width: width - 28, Height: gridGroupHeaderHeight, Padding: woxwidget.Insets{Left: 8, Top: 9},
				Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle},
			})
			index++
			continue
		}
		cells := make([]woxwidget.Widget, 0, layout.Columns)
		for len(cells) < layout.Columns && index < len(snapshot.results) && !snapshot.results[index].IsGroup {
			cells = append(cells, a.buildGridResult(snapshot, index, cellWidth, cellHeight, visualWidth, visualHeight, layout))
			index++
		}
		for len(cells) < layout.Columns {
			cells = append(cells, woxwidget.Painter{Width: cellWidth, Height: cellHeight})
		}
		rows = append(rows, woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells})
	}
	contentHeight := float32(gridResultsHeight(snapshot.results, width, snapshot.layout.GridLayout))
	offset := a.configureResultScroll(snapshot.results, snapshot.layout.GridLayout, snapshot.selected, width, height, contentHeight)
	content := woxwidget.Container{Width: width, Height: contentHeight, Padding: woxwidget.Insets{Left: 14, Right: 14}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: rows}}
	return a.buildResultScrollSurface(content, snapshot.palette, width, height, contentHeight, offset)
}

// gridResultVerticalBounds maps a source index through group headers and wrapped grid rows.
func gridResultVerticalBounds(results []queryResult, target int, width float32, raw *gridLayout) (float32, float32) {
	layout := normalizedGridLayout(raw)
	_, cellHeight, _, _ := gridCellMetrics(width, layout)
	y := float32(0)
	for index := 0; index < len(results); {
		if results[index].IsGroup {
			if index == target {
				return y, y + gridGroupHeaderHeight
			}
			y += gridGroupHeaderHeight
			index++
			continue
		}
		rowStart := index
		for index < len(results) && !results[index].IsGroup && index-rowStart < layout.Columns {
			index++
		}
		if target >= rowStart && target < index {
			return y, y + cellHeight
		}
		y += cellHeight
	}
	return y, y
}

func (a *App) buildGridResult(snapshot viewSnapshot, index int, cellWidth, cellHeight, visualWidth, visualHeight float32, layout gridLayout) woxwidget.Widget {
	result := snapshot.results[index]
	selected := index == snapshot.selected
	hovered := index == snapshot.hoveredResult
	frameColor := woxui.Color{}
	if selected {
		frameColor = snapshot.palette.selectedBackground
	} else if hovered {
		frameColor = snapshot.palette.selectedBackground
		frameColor.A = uint8(float32(frameColor.A)*0.25 + 0.5)
	}
	var visual woxwidget.Widget = woxwidget.Painter{Width: visualWidth, Height: visualHeight}
	if image := a.imageFor(result.Icon); image != nil {
		visual = woxwidget.Image{Source: image, Width: visualWidth, Height: visualHeight}
	}
	visual = woxwidget.Container{
		Width: visualWidth + float32(layout.ItemPadding*2), Height: visualHeight + float32(layout.ItemPadding*2), Radius: 8, Color: frameColor,
		Padding: woxwidget.UniformInsets(float32(layout.ItemPadding)), Child: visual,
	}
	children := []woxwidget.Widget{visual}
	if layout.ShowTitle {
		children = append(children, woxwidget.Container{
			Width: visualWidth, Height: gridTitleHeight, Padding: woxwidget.Insets{Top: 4},
			Child: woxwidget.Text{Value: result.Title, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultTitle},
		})
	}
	return woxwidget.Gesture{
		ID: fmt.Sprintf("grid-result-%s", result.ID),
		OnHover: func(inside bool) {
			a.hoverResult(index, inside)
		},
		OnTap:       func() { a.selectResult(index) },
		OnDoubleTap: func() { a.selectResult(index); a.activateResult(index) },
		Child: woxwidget.Container{
			Width: cellWidth, Height: cellHeight,
			Padding: woxwidget.UniformInsets(float32(layout.ItemMargin)),
			Child:   woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
		},
	}
}
