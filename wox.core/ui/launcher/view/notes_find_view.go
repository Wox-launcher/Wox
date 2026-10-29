package view

import (
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// NotesFindProps carries current-document search state without owning editor state.
type NotesFindProps struct {
	Size                                               woxui.Size
	Theme                                              woxcomponent.Theme
	Overlay                                            woxwidget.Widget
	Window                                             *woxui.Window
	Editor                                             *woxwidget.TextEditingController
	Focus                                              *woxwidget.FocusNode
	Label, PreviousLabel, NextLabel, CloseLabel, Count string
	Enabled                                            bool
	OnPrevious, OnNext, OnClose                        func()
	OnChanged                                          func(string)
	OnKey                                              func(woxui.KeyEvent) bool
}

// NotesFindBar places a nonmodal compact search row at the editor's upper-right corner.
func NotesFindBar(p NotesFindProps) woxwidget.Widget {
	scale := p.Theme.Controls.Scaled
	width := min(scale(400), p.Size.Width-scale(16))
	button := func(id, label string, glyph woxwidget.Widget, enabled bool, action func()) woxwidget.Widget {
		return woxcomponent.WoxIconButton(woxcomponent.IconButtonProps{ID: id, Label: label, Width: scale(28), Height: scale(28), Disabled: !enabled, OnTap: action, FocusRingColor: p.Theme.Controls.Focus, HoverBackground: p.Theme.Controls.TextSelectionBackground, Icon: glyph})
	}
	row := woxwidget.Flex{Axis: woxwidget.Horizontal, Gap: scale(4), CrossAxisAlignment: woxwidget.CrossAxisCenter, Children: []woxwidget.Widget{
		woxcomponent.WoxTextField(woxcomponent.TextFieldProps{
			ID: "notes.find", Label: p.Label, Hint: p.Label, Width: width - scale(164), Height: scale(28),
			// The compact field cannot use the taller form field's default vertical padding.
			Padding: woxwidget.Insets{Left: scale(8), Right: scale(8)},
			Style:   woxui.TextStyle{Size: scale(woxcomponent.SettingsControlFontSize)}, LineHeight: scale(28), TextAlignmentY: .5,
			Controller: p.Editor, Value: p.Editor.Text(), FocusNode: p.Focus, Focused: true, Autofocus: true,
			Window: p.Window, Theme: p.Theme.Controls, OnChanged: p.OnChanged, OnKey: p.OnKey,
		}),
		woxwidget.Container{Width: scale(48), Child: woxwidget.Text{Value: p.Count, Style: woxui.TextStyle{Size: scale(12)}, Color: p.Theme.Controls.TextSecondary}},
		button("notes.find.previous", p.PreviousLabel, woxcomponent.ChevronGlyph(scale(16), p.Theme.Controls.BodyText, true), p.Enabled, p.OnPrevious), button("notes.find.next", p.NextLabel, woxcomponent.ChevronGlyph(scale(16), p.Theme.Controls.BodyText, false), p.Enabled, p.OnNext), button("notes.find.close", p.CloseLabel, woxcomponent.CloseGlyph(scale(16), p.Theme.Controls.BodyText), true, p.OnClose),
	}}
	panel := woxwidget.Container{Width: width, Height: scale(44), Padding: woxwidget.UniformInsets(scale(8)), Radius: scale(6), Floating: true, Color: p.Theme.ActionBackground, BorderColor: p.Theme.PreviewSplit, BorderWidth: 1, Child: row}
	layers := []woxwidget.StackChild{}
	if p.Overlay != nil {
		layers = append(layers, woxwidget.StackChild{Child: p.Overlay})
	}
	layers = append(layers, woxwidget.StackChild{Left: p.Size.Width - width - scale(8), Top: NotesToolbarHeight, Child: panel})
	return woxwidget.Stack{Width: p.Size.Width, Height: p.Size.Height, Children: layers}
}
