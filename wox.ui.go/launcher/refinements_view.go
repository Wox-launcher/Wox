package launcher

import (
	"fmt"
	"slices"
	"strings"

	woxui "github.com/Wox-launcher/wox.ui.go"
	woxwidget "github.com/Wox-launcher/wox.ui.go/widget"
)

func (a *App) buildRefinementToggle(snapshot viewSnapshot) woxwidget.Widget {
	label := a.refinementSummary(snapshot)
	active := snapshot.refinementOpen || label != a.translate("i18n:ui_query_refinement_filters")
	tint := snapshot.palette.queryText
	backgroundOpacity := float32(0.075)
	borderOpacity := float32(0.13)
	textOpacity := float32(0.72)
	if active {
		tint = snapshot.palette.cursor
		backgroundOpacity = 0.15
		borderOpacity = 0.32
		textOpacity = 0.94
	}
	width := a.refinementToggleWidth(snapshot)
	return woxwidget.Gesture{
		ID:    "query-refinements-toggle",
		OnTap: func() { a.toggleRefinementBar() },
		Child: woxwidget.Container{
			Width: width, Height: 34, Padding: woxwidget.Insets{Top: 4},
			Child: woxwidget.Container{
				Width: width, Height: 26, Radius: 7, Color: colorWithOpacity(tint, borderOpacity), Padding: woxwidget.UniformInsets(1),
				Child: woxwidget.Container{
					Width: width - 2, Height: 24, Radius: 6, Color: opaqueOverlay(snapshot.palette.background, tint, backgroundOpacity),
					Padding: woxwidget.Insets{Left: 7, Top: 4, Right: 8, Bottom: 3},
					Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 5, Children: []woxwidget.Widget{
						a.buildFilterIcon(colorWithOpacity(tint, 0.92)),
						woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: colorWithOpacity(snapshot.palette.queryText, textOpacity)},
					}},
				},
			},
		},
	}
}

func (a *App) refinementToggleWidth(snapshot viewSnapshot) float32 {
	label := a.refinementSummary(snapshot)
	metrics, _ := a.window.MeasureText(label, woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold})
	return min(float32(150), max(float32(72), metrics.Size.Width+37))
}

func (a *App) buildFilterIcon(color woxui.Color) woxwidget.Widget {
	return woxwidget.Painter{Width: 15, Height: 15, Paint: func(displayList *woxui.DisplayList, bounds woxui.Rect) {
		for index, lineWidth := range []float32{13, 9, 5} {
			displayList.FillRoundedRect(woxui.Rect{X: bounds.X + 1, Y: bounds.Y + 3 + float32(index)*4, Width: lineWidth, Height: 1.5}, 0.75, color)
		}
	}}
}

func (a *App) buildRefinementBar(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	controls := make([]woxwidget.Widget, 0, len(snapshot.refinements))
	for _, refinement := range snapshot.refinements {
		group := make([]woxwidget.Widget, 0, len(refinement.Options)+2)
		if refinement.Title != "" {
			group = append(group, woxwidget.Container{Height: 22, Padding: woxwidget.Insets{Left: 7, Top: 5, Right: 7, Bottom: 3}, Child: woxwidget.Text{
				Value: a.translate(refinement.Title), Style: woxui.TextStyle{Size: 11, Weight: woxui.FontWeightSemibold}, Color: colorWithOpacity(snapshot.palette.resultSubtitle, 0.68),
			}})
			group = append(group, woxwidget.Container{Width: 1, Height: 22, Padding: woxwidget.Insets{Top: 4, Bottom: 4}, Child: woxwidget.Container{Width: 1, Height: 14, Color: colorWithOpacity(snapshot.palette.resultSubtitle, 0.13)}})
		}
		options := refinement.Options
		if len(options) == 0 {
			value := "true"
			if len(refinement.DefaultValue) > 0 && refinement.DefaultValue[0] != "" {
				value = refinement.DefaultValue[0]
			}
			options = []queryRefinementOption{{Value: value, Title: refinement.Title}}
		}
		for _, option := range options {
			selected := slices.Contains(splitRefinementValues(snapshot.refinementValues[refinement.ID]), option.Value)
			group = append(group, a.buildRefinementOption(refinement, option, selected, snapshot.palette))
		}
		controls = append(controls, woxwidget.Container{
			Height: 26, Radius: 7, Color: colorWithOpacity(snapshot.palette.resultSubtitle, 0.12), Padding: woxwidget.UniformInsets(1),
			Child: woxwidget.Container{
				Height: 24, Radius: 6, Color: opaqueOverlay(snapshot.palette.background, snapshot.palette.queryText, 0.035), Padding: woxwidget.Insets{Left: 2, Top: 1, Right: 2, Bottom: 1},
				Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 1, Children: group},
			},
		})
	}
	return woxwidget.Container{
		Width: width, Height: height, Padding: woxwidget.Insets{Left: 8, Top: 10, Right: 8, Bottom: 8},
		Child: woxwidget.Clip{
			Width: max(float32(0), width-16), Height: 26,
			Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 10, Children: controls},
		},
	}
}

func (a *App) buildRefinementOption(refinement queryRefinement, option queryRefinementOption, selected bool, palette uiPalette) woxwidget.Widget {
	background := woxui.Color{}
	foreground := colorWithOpacity(palette.queryText, 0.82)
	if selected {
		background = colorWithOpacity(palette.selectedBackground, 0.22)
		foreground = palette.queryText
	}
	label := a.translate(option.Title)
	if label == "" {
		label = option.Value
	}
	if option.Count != nil {
		label = fmt.Sprintf("%s (%d)", label, *option.Count)
	}
	children := make([]woxwidget.Widget, 0, 2)
	if image := a.imageFor(option.Icon); image != nil {
		children = append(children, woxwidget.Image{Source: image, Width: 16, Height: 16})
	}
	children = append(children, woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground})
	return woxwidget.Gesture{
		ID: fmt.Sprintf("refinement-%s-%s", refinement.ID, option.Value),
		OnTap: func() {
			a.selectRefinementOption(refinement.ID, option.Value)
		},
		Child: woxwidget.Container{
			Height: 22, Radius: 5, Color: background,
			Padding: woxwidget.Insets{Left: 10, Top: 4, Right: 10, Bottom: 3},
			Child:   woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 6, Children: children},
		},
	}
}

func (a *App) refinementSummary(snapshot viewSnapshot) string {
	fallback := a.translate("i18n:ui_query_refinement_filters")
	if strings.HasPrefix(fallback, "ui query refinement") || fallback == "" {
		fallback = "Filters"
	}
	labels := make([]string, 0, 2)
	activeControls := 0
	for _, refinement := range snapshot.refinements {
		selected := normalizeRefinementValues(refinement, splitRefinementValues(snapshot.refinementValues[refinement.ID]))
		defaults := normalizeRefinementValues(refinement, nil)
		if sameStringSet(selected, defaults) {
			continue
		}
		activeControls++
		for _, value := range selected {
			for _, option := range refinement.Options {
				if option.Value == value {
					labels = append(labels, a.translate(option.Title))
					break
				}
			}
			if len(labels) == 2 {
				break
			}
		}
	}
	if len(labels) == 0 {
		return fallback
	}
	label := strings.Join(labels, ", ")
	if activeControls > len(labels) {
		label += fmt.Sprintf(" +%d", activeControls-len(labels))
	}
	return label
}

func sameStringSet(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for _, value := range left {
		if !slices.Contains(right, value) {
			return false
		}
	}
	return true
}
