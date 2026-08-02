package view

import (
	"fmt"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AISettingsTable ties keyboard selection to one shared table field.
type AISettingsTable struct {
	Index       int
	Field       FormTableFieldProps
	Highlighted bool
}

// AISettingsProps contains AI settings page presentation data.
type AISettingsProps struct {
	Width       float32
	Height      float32
	Theme       woxcomponent.Theme
	Available   bool
	Title       string
	Description string
	Error       string
	Tables      []AISettingsTable
	Selected    int
}

// AISettingsView builds the AI catalog tables and page scroll surface.
func AISettingsView(props AISettingsProps) woxwidget.Widget {
	contentWidth := SettingsPageContentWidth(props.Width)
	header := woxcomponent.WoxPageHeader(woxcomponent.PageHeaderProps{Title: props.Title, Description: props.Description, Width: contentWidth, Theme: props.Theme})
	if !props.Available {
		message := woxwidget.Container{Width: contentWidth, Height: 30, Child: woxwidget.Text{
			Value: "AI settings are unavailable.", Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
		}}
		return SettingsPage(SettingsPageProps{
			ID: "ai-settings-scroll", Width: props.Width, Height: props.Height, Children: []woxwidget.Widget{header, message}, ContentHeight: woxcomponent.PageHeaderHeight + 30,
		})
	}

	children := []woxwidget.Widget{header}
	contentHeight := woxcomponent.PageHeaderHeight
	var keepVisibleKey woxwidget.Key
	for _, table := range props.Tables {
		if table.Index == props.Selected {
			keepVisibleKey = woxwidget.Key(fmt.Sprintf("ai-settings-table-%d", table.Index))
		}
		target := woxcomponent.WoxSettingTarget(woxcomponent.SettingTargetProps{
			Width: table.Field.Width, Highlighted: table.Highlighted, Child: FormTableField(table.Field), Theme: props.Theme,
		})
		children = append(children, woxwidget.Keyed{Key: woxwidget.Key(fmt.Sprintf("ai-settings-table-%d", table.Index)), Child: target})
	}
	if props.Error != "" {
		children = append(children, woxwidget.Container{Width: contentWidth, Height: 30, Padding: woxwidget.Insets{Top: 8}, Child: woxwidget.TextBlock{
			Value: props.Error, Width: contentWidth, Height: 20, MaxLines: 1, Style: woxui.TextStyle{Size: 11}, Color: props.Theme.ErrorText,
		}})
		contentHeight += 30
	}
	return SettingsPage(SettingsPageProps{
		ID: "ai-settings-scroll", Width: props.Width, Height: props.Height, Children: children, ContentHeight: contentHeight, KeepVisibleKey: keepVisibleKey,
	})
}
