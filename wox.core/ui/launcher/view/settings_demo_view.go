package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const (
	settingsDemoWidth  = float32(680)
	settingsDemoHeight = float32(460)
	settingsDemoMargin = float32(12)
)

// SettingsDemoOverlay places one reusable animated demo beside its title trigger.
func SettingsDemoOverlay(props OnboardingProps, step OnboardingStep, anchor woxui.Rect, windowWidth, windowHeight float32, theme woxcomponent.Theme, onHover func(bool)) (woxwidget.Widget, float32, float32) {
	width := min(settingsDemoWidth, max(float32(160), windowWidth-settingsDemoMargin*2))
	height := min(settingsDemoHeight, max(float32(140), windowHeight-settingsDemoMargin*2))
	left := min(max(settingsDemoMargin, anchor.X+anchor.Width-width), max(settingsDemoMargin, windowWidth-width-settingsDemoMargin))
	top := anchor.Y + anchor.Height + settingsDemoMargin
	if top+height+settingsDemoMargin > windowHeight {
		top = max(settingsDemoMargin, anchor.Y-height-settingsDemoMargin)
	}
	preview := woxwidget.Container{
		Width: width, Height: height, Radius: 8, Color: theme.Background,
		BorderColor: settingsColorAlpha(theme.ResultTitle, 31), BorderWidth: 1,
		Child: woxwidget.Clip{Width: width, Height: height, Child: DemoPreview(props, step, width, height)},
	}
	overlay := woxwidget.Semantics{
		Key: woxwidget.Key("settings-demo-overlay-" + step.ID), AutomationID: "settings-demo-overlay-" + step.ID, Role: woxui.AccessibilityRoleGroup, Label: step.Title,
		Child: woxwidget.Gesture{ID: "settings-demo-overlay-hover-" + step.ID, OnHover: onHover, Child: preview},
	}
	return overlay, left, top
}
