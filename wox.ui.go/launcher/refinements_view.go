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
	background := snapshot.palette.queryBackground
	foreground := snapshot.palette.resultSubtitle
	if snapshot.refinementOpen || label != a.translate("i18n:ui_query_refinement_filters") {
		background = snapshot.palette.selectionBackground
		foreground = snapshot.palette.queryText
	}
	return woxwidget.Gesture{
		ID:    "query-refinements-toggle",
		OnTap: func() { a.toggleRefinementBar() },
		Child: woxwidget.Container{
			Width: 128, Height: 30, Radius: 7, Color: background,
			Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 8, Bottom: 5},
			Child:   woxwidget.Text{Value: label, Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: foreground},
		},
	}
}

func (a *App) buildRefinementBar(snapshot viewSnapshot, width, height float32) woxwidget.Widget {
	controls := make([]woxwidget.Widget, 0)
	for _, refinement := range snapshot.refinements {
		refinement := refinement
		if len(snapshot.refinements) > 1 && refinement.Title != "" {
			controls = append(controls, woxwidget.Container{Height: 30, Padding: woxwidget.Insets{Top: 8, Right: 2}, Child: woxwidget.Text{
				Value: a.translate(refinement.Title), Style: woxui.TextStyle{Size: 12, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.resultSubtitle,
			}})
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
			option := option
			selected := slices.Contains(splitRefinementValues(snapshot.refinementValues[refinement.ID]), option.Value)
			controls = append(controls, a.buildRefinementOption(refinement, option, selected, snapshot.palette))
		}
	}
	return woxwidget.Container{
		Width: width, Height: height, Color: snapshot.palette.toolbarBackground,
		Padding: woxwidget.Insets{Left: 20, Top: 9, Right: 20, Bottom: 9},
		Child:   woxwidget.Wrap{Gap: 8, RunGap: 6, Children: controls},
	}
}

func (a *App) buildRefinementOption(refinement queryRefinement, option queryRefinementOption, selected bool, palette uiPalette) woxwidget.Widget {
	background := palette.queryBackground
	foreground := palette.resultSubtitle
	if selected {
		background = palette.selectedBackground
		foreground = palette.selectedTitle
	}
	label := a.translate(option.Title)
	if label == "" {
		label = option.Value
	}
	if option.Count != nil {
		label = fmt.Sprintf("%s %d", label, *option.Count)
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
			Height: 30, Radius: 7, Color: background,
			Padding: woxwidget.Insets{Left: 10, Top: 7, Right: 10, Bottom: 5},
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
