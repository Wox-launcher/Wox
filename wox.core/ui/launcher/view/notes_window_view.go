package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

const NotesToolbarHeight = woxcomponent.TitleBarHeight
const NotesFormatBarHeight = float32(36)
const NotesStatusHeight = float32(32)

// NotesWindowProps contains the prepared Notes toolbar, editor, and transient overlay.
type NotesWindowProps struct {
	Width, Height float32
	Label         string
	Toolbar       woxwidget.Widget
	Editor        woxwidget.Widget
	FormatBar     woxwidget.Widget
	Overlay       woxwidget.Widget
	Status        woxwidget.Widget
	Theme         woxcomponent.Theme
}

// NotesWindow builds the compact utility surface without capturing mutable controller state.
func NotesWindow(props NotesWindowProps) woxwidget.Widget {
	formatHeight := float32(0)
	if props.FormatBar != nil {
		formatHeight = NotesFormatBarHeight
	}
	statusHeight := float32(0)
	if props.Status != nil {
		statusHeight = NotesStatusHeight
	}
	editorHeight := max(float32(0), props.Height-NotesToolbarHeight-formatHeight-statusHeight)
	children := []woxwidget.Widget{props.Toolbar, woxwidget.Container{Width: props.Width, Height: editorHeight, Child: props.Editor}}
	if props.Status != nil {
		children = append(children, props.Status)
	}
	if props.FormatBar != nil {
		children = append(children, props.FormatBar)
	}
	// Paint AppBackgroundColor over native acrylic or vibrancy, matching the
	// launcher and WebView windows so editor text stays readable.
	body := woxwidget.Container{Width: props.Width, Height: props.Height, Color: props.Theme.Background, Child: woxwidget.Flex{Axis: woxwidget.Vertical, Children: children}}
	layers := []woxwidget.StackChild{{Child: body}}
	if props.Overlay != nil {
		layers = append(layers, woxwidget.StackChild{Child: props.Overlay})
	}
	return woxwidget.Semantics{
		Key: "notes-window", AutomationID: "notes.window", Role: woxui.AccessibilityRoleWindow, Label: props.Label,
		Child: woxwidget.Stack{Width: props.Width, Height: props.Height, Children: layers},
	}
}
