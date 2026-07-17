package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// AboutLink contains one action shown on the About page.
type AboutLink struct {
	ID    string
	Label string
	Icon  *woxui.Image
	OnTap func()
}

// AboutSettingsProps contains the About branding, running version, and actions.
type AboutSettingsProps struct {
	Width       float32
	Height      float32
	AppIcon     *woxui.Image
	Version     string
	Description string
	Status      string
	Links       []AboutLink
	Theme       woxcomponent.Theme
}

// AboutSettingsView builds the About settings route.
func AboutSettingsView(props AboutSettingsProps) woxwidget.Widget {
	contentWidth := min(float32(600), max(float32(0), props.Width-48))
	links := make([]woxwidget.Widget, 0, len(props.Links))
	for _, link := range props.Links {
		links = append(links, aboutLinkButton(link, props.Theme))
	}

	var logo woxwidget.Widget = woxwidget.Container{
		Width: 100, Height: 100, Radius: 22, Color: woxui.Color{R: 255, G: 255, B: 255, A: 255},
		Child: woxwidget.Align{Width: 100, Height: 100, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: "W", Style: woxui.TextStyle{Size: 54, Weight: woxui.FontWeightSemibold}, Color: woxui.Color{A: 255},
		}},
	}
	if props.AppIcon != nil {
		logo = woxwidget.Image{Source: props.AppIcon, Width: 100, Height: 100}
	}

	children := []woxwidget.Widget{
		woxwidget.Container{Height: 40},
		woxwidget.Align{Width: contentWidth, Height: 100, Horizontal: 0.5, Child: logo},
		woxwidget.Container{Height: 30},
		woxwidget.Align{Width: contentWidth, Height: 28, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Container{
			Height: 26, Radius: 16, Color: props.Theme.ActionSelected, Padding: woxwidget.Insets{Left: 12, Top: 4, Right: 12, Bottom: 4},
			Child: woxwidget.Text{Value: props.Version, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ActionSelectedText},
		}},
		woxwidget.Container{Height: 30},
		woxwidget.Align{Width: contentWidth, Height: 24, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Text{
			Value: props.Description, Style: woxui.TextStyle{Size: 16}, Color: props.Theme.ResultTitle,
		}},
		woxwidget.Container{Height: 40},
		woxwidget.Align{Width: contentWidth, Height: 26, Horizontal: 0.5, Vertical: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 30, Children: links}},
	}
	if props.Status != "" {
		children = append(children,
			woxwidget.Container{Height: 18},
			woxwidget.Align{Width: contentWidth, Height: 18, Horizontal: 0.5, Child: woxwidget.Text{Value: props.Status, Style: woxui.TextStyle{Size: 12}, Color: props.Theme.ErrorText}},
		)
	}
	children = append(children, woxwidget.Container{Height: 40})

	return woxwidget.Container{Width: props.Width, Height: props.Height, Child: woxwidget.Align{
		Width: props.Width, Height: props.Height, Horizontal: 0.5, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children},
	}}
}

// aboutLinkButton keeps the About actions visually lightweight while preserving keyboard and accessibility behavior.
func aboutLinkButton(link AboutLink, theme woxcomponent.Theme) woxwidget.Widget {
	children := make([]woxwidget.Widget, 0, 2)
	if link.Icon != nil {
		children = append(children, woxwidget.Image{Source: link.Icon, Width: 18, Height: 18})
	}
	children = append(children, woxwidget.Text{Value: link.Label, Style: woxui.TextStyle{Size: 13}, Color: theme.ResultTitle})
	key := woxwidget.Key(link.ID)
	return woxwidget.Semantics{
		Key: key, AutomationID: string(key), Role: woxui.AccessibilityRoleButton, Label: link.Label, Actions: []woxui.AccessibilityAction{woxui.AccessibilityActionActivate},
		Child: woxwidget.Focusable{Key: key, OnKey: func(event woxui.KeyEvent) bool {
			if event.Key != woxui.KeyEnter && event.Key != woxui.KeySpace {
				return false
			}
			if event.Down && link.OnTap != nil {
				link.OnTap()
			}
			return true
		}, Child: woxwidget.Gesture{ID: string(key), OnTap: link.OnTap, Child: woxwidget.Container{
			Height: 26, Padding: woxwidget.Insets{Left: 6, Top: 4, Right: 6, Bottom: 4}, Child: woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: 8, Children: children},
		}}},
	}
}
