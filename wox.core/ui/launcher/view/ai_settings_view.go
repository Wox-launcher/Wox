package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AISettingsCellKind selects special inline table cell rendering.
type AISettingsCellKind uint8

const (
	AISettingsCellText AISettingsCellKind = iota
	AISettingsCellStatus
	AISettingsCellAction
)

// AISettingsColumn describes one weighted inline table column.
type AISettingsColumn struct {
	Label  string
	Weight float32
}

// AISettingsCell contains one prepared inline table value.
type AISettingsCell struct {
	Text string
	Kind AISettingsCellKind
}

// AISettingsTable contains one prepared providers, MCP, or skills table.
type AISettingsTable struct {
	Index       int
	Title       string
	Description string
	Columns     []AISettingsColumn
	Rows        [][]AISettingsCell
	OnAdd       func()
	OnOpenRow   func(int)
}

// AISettingsProps contains AI settings page presentation data.
type AISettingsProps struct {
	Width         float32
	Height        float32
	Theme         woxcomponent.Theme
	Available     bool
	Title         string
	Description   string
	AddLabel      string
	NoDataLabel   string
	Note          string
	Scroll        float32
	Tables        []AISettingsTable
	OnScroll      func(float32)
	OnSetGeometry func(viewport, content, rowsTop float32)
}

// AISettingsView builds the AI catalog tables and page scroll surface.
func AISettingsView(props AISettingsProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	if !props.Available {
		message := woxwidget.Container{Width: contentWidth, Height: 30, Child: woxwidget.Text{
			Value: "AI settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
		return SettingsPage(SettingsPageProps{ID: "ai-settings-scroll", Width: props.Width, Height: props.Height, Children: []woxwidget.Widget{message}, ContentHeight: 30})
	}
	children := []woxwidget.Widget{
		woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{Title: props.Title, Description: props.Description, Width: contentWidth, Theme: props.Theme}),
	}
	contentHeight := float32(72)
	for _, table := range props.Tables {
		widget, tableHeight := aiSettingsTable(props, table, contentWidth)
		children = append(children, widget)
		contentHeight += tableHeight
	}
	if props.Note != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: props.Note, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle,
		}})
		contentHeight += 30
	}
	return SettingsPage(SettingsPageProps{
		ID: "ai-settings-scroll", Width: props.Width, Height: props.Height, Children: children, ContentHeight: contentHeight, Scroll: props.Scroll,
		OnScroll: props.OnScroll, OnSetGeometry: func(viewport, content float32) {
			if props.OnSetGeometry != nil {
				props.OnSetGeometry(viewport, content, 0)
			}
		},
	})
}

func aiSettingsTable(props AISettingsProps, table AISettingsTable, width float32) (woxwidget.Widget, float32) {
	titleHeight := float32(42)
	if table.Description != "" {
		titleHeight = 60
	}
	bodyHeight := float32(38 + len(table.Rows)*36)
	if len(table.Rows) == 0 {
		bodyHeight = 116
	}
	sectionHeight := titleHeight + bodyHeight + 24
	titleWidth := max(float32(0), width-86)
	titleChildren := []woxwidget.Widget{
		woxwidget.Text{Value: table.Title, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle},
	}
	if table.Description != "" {
		titleChildren = append(titleChildren, woxwidget.TextBlock{Value: table.Description, Width: titleWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultSubtitle})
	}
	title := woxwidget.Container{Width: width, Height: titleHeight, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 12, Children: []woxwidget.Widget{
		woxwidget.Container{Width: titleWidth, Height: titleHeight, Padding: woxwidget.Insets{Top: 5}, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 5, Children: titleChildren}},
		woxwidget.Container{Width: 74, Height: titleHeight, Padding: woxwidget.Insets{Top: 1}, Child: woxcomponent.WoxButton(woxcomponent.ButtonProps{
			ID: fmt.Sprintf("ai-table-add-%d", table.Index), Label: props.AddLabel, Width: 74, Variant: woxcomponent.ButtonOutline, Size: woxcomponent.ButtonCompact, OnTap: table.OnAdd, Theme: props.Theme,
		})},
	}}}
	grid := aiSettingsGrid(props, table, width, bodyHeight)
	return woxwidget.Container{Width: width, Height: sectionHeight, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: []woxwidget.Widget{title, grid}}}, sectionHeight
}

func aiSettingsGrid(props AISettingsProps, table AISettingsTable, width, height float32) woxwidget.Widget {
	children := []woxwidget.Widget{aiSettingsGridRow(props, table, nil, -1, width, 38, true)}
	if len(table.Rows) == 0 {
		children = append(children, woxwidget.Container{Width: width, Height: height - 38, BorderColor: aiSettingsAlpha(props.Theme.PreviewSplit, 144), BorderWidth: 1,
			Padding: woxwidget.Insets{Left: max(float32(0), width/2-34), Top: 31}, Child: woxwidget.Text{Value: props.NoDataLabel, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ResultSubtitle}})
	} else {
		for rowIndex, row := range table.Rows {
			children = append(children, aiSettingsGridRow(props, table, row, rowIndex, width, 36, false))
		}
	}
	return woxwidget.Container{Width: width, Height: height, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
}

func aiSettingsGridRow(props AISettingsProps, table AISettingsTable, row []AISettingsCell, rowIndex int, width, height float32, header bool) woxwidget.Widget {
	background := aiSettingsAlpha(props.Theme.QueryBackground, 32)
	if header {
		background = aiSettingsAlpha(props.Theme.QueryBackground, 92)
	}
	cells := make([]woxwidget.Widget, 0, len(table.Columns))
	remaining := width
	for columnIndex, column := range table.Columns {
		cellWidth := width * column.Weight
		if columnIndex == len(table.Columns)-1 {
			cellWidth = remaining
		}
		remaining -= cellWidth
		cellData := AISettingsCell{Text: column.Label}
		if !header && columnIndex < len(row) {
			cellData = row[columnIndex]
		}
		var child woxwidget.Widget = woxwidget.TextBlock{Value: cellData.Text, Width: max(float32(0), cellWidth-14), Height: height - 10, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ResultTitle}
		if header {
			child = woxwidget.Text{Value: cellData.Text, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultTitle}
		} else if cellData.Kind == AISettingsCellStatus {
			child = woxwidget.Container{Width: 16, Height: 16, Radius: 8, Color: woxui.Color{R: 69, G: 184, B: 88, A: 255}}
		} else if cellData.Kind == AISettingsCellAction {
			child = woxwidget.Text{Value: cellData.Text, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ResultSubtitle}
		}
		cell := woxwidget.Container{Width: cellWidth, Height: height, Color: background, BorderColor: aiSettingsAlpha(props.Theme.PreviewSplit, 144), BorderWidth: 0.5,
			Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 6}, Child: child}
		if !header {
			currentRow := rowIndex
			cell = woxwidget.Container{Width: cellWidth, Height: height, Child: woxwidget.Gesture{ID: fmt.Sprintf("ai-table-%d-row-%d-column-%d", table.Index, rowIndex, columnIndex), OnTap: func() {
				if table.OnOpenRow != nil {
					table.OnOpenRow(currentRow)
				}
			}, Child: cell}}
		}
		cells = append(cells, cell)
	}
	return woxwidget.Flex{Axis: woxwidget.Horizontal, Children: cells}
}

func aiSettingsAlpha(color woxui.Color, alpha uint8) woxui.Color {
	color.A = alpha
	return color
}
