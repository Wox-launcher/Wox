package launcher

import (
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// buildAISettingsPage renders the three core-backed AI tables through the shared form surface.
func (a *App) buildAISettingsPage(snapshot settingsSnapshot, width, height float32) woxwidget.Widget {
	contentWidth := max(float32(0), width-72)
	if snapshot.aiForm == nil {
		return woxwidget.Container{Width: width, Height: height, Padding: woxwidget.Insets{Left: 36, Top: 36, Right: 36}, Child: woxwidget.Text{
			Value: "AI settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle,
		}}
	}
	callbacks := formFieldCallbacks{idPrefix: "ai-settings", focus: a.selectAISettingsTable, openTable: a.openAISettingsTable}
	rows := make([]woxwidget.Widget, 0, len(snapshot.aiForm.definitions))
	for index, definition := range snapshot.aiForm.definitions {
		rows = append(rows, a.buildFormTableField(*snapshot.aiForm, callbacks, snapshot.palette, index, definition, contentWidth, 142))
	}
	note := snapshot.note
	if snapshot.aiProvidersLoading {
		note = "Loading the provider catalog…"
	} else if snapshot.aiProvidersError != "" {
		note = "Provider catalog unavailable: " + snapshot.aiProvidersError
	} else if note == "" {
		note = "Providers and MCP servers save immediately · Skills currently add local directories"
	}
	return woxwidget.Container{
		Width: width, Height: height,
		Padding: woxwidget.Insets{Left: 36, Top: 30, Right: 36, Bottom: 22},
		Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 10, Children: []woxwidget.Widget{
			woxwidget.Container{Width: contentWidth, Height: 62, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 7, Children: []woxwidget.Widget{
				woxwidget.Text{Value: "AI", Style: woxui.TextStyle{Size: 24, Weight: woxui.FontWeightSemibold}, Color: snapshot.palette.queryText},
				woxwidget.Text{Value: "Configure model providers, MCP tools, and reusable skills", Style: woxui.TextStyle{Size: 13}, Color: snapshot.palette.resultSubtitle},
			}}},
			woxwidget.Flex{Axis: woxwidget.Vertical, Gap: 8, Children: rows},
			woxwidget.Container{Width: contentWidth, Height: 32, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
				Value: note, Width: contentWidth, Height: 24, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: snapshot.palette.resultSubtitle,
			}},
		}},
	}
}
