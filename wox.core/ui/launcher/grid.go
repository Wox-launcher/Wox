package launcher

import (
	"math"

	launcherview "wox/ui/launcher/view"
	woxwidget "wox/ui/widget"
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
	results := make([]launcherview.LauncherGridResult, 0, len(snapshot.results))
	for index, result := range snapshot.results {
		index := index
		item := launcherview.LauncherGridResult{
			ID: result.ID, Title: result.Title, Group: result.IsGroup, Selected: index == snapshot.selected, Hovered: index == snapshot.hoveredResult,
		}
		if !result.IsGroup {
			item.Icon = a.imageFor(result.Icon)
			item.OnHover = func(inside bool) { a.hoverResult(index, inside) }
			item.OnSelect = func() { a.selectResult(index) }
			item.OnActivate = func() { a.activateResult(index) }
		}
		results = append(results, item)
	}
	contentHeight := float32(gridResultsHeight(snapshot.results, width, snapshot.layout.GridLayout))
	offset := a.configureResultScroll(snapshot.results, snapshot.layout.GridLayout, snapshot.selected, width, height, contentHeight)
	return launcherview.LauncherGridView(launcherview.LauncherGridProps{
		Width: width, Height: height, ContentHeight: contentHeight, Offset: offset, Columns: layout.Columns,
		ItemPadding: float32(layout.ItemPadding), ItemMargin: float32(layout.ItemMargin), ShowTitle: layout.ShowTitle,
		CellWidth: cellWidth, CellHeight: cellHeight, VisualWidth: visualWidth, VisualHeight: visualHeight,
		GroupHeaderHeight: gridGroupHeaderHeight, TitleHeight: gridTitleHeight, Theme: snapshot.palette.componentTheme(), Results: results,
		OnScroll: a.scrollResults,
	})
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
