package component

import (
	"strings"

	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// HotkeyRecorderProps describes the display state of a tappable hotkey recorder.
type HotkeyRecorderProps struct {
	ID            string
	Labels        []string
	Placeholder   string
	Focused       bool
	Error         bool
	Hold          bool
	HoldPrefix    string
	Window        *woxui.Window
	Theme         Theme
	OnFocusChange func(bool)
}

// WoxHotkeyRecorder matches Flutter's outlined recorder with platform-labelled keycaps.
func WoxHotkeyRecorder(props HotkeyRecorderProps) (woxwidget.Widget, float32) {
	border := withAlpha(props.Theme.ResultSubtitle, 140)
	if props.Error {
		border = props.Theme.ErrorText
	} else if props.Focused {
		border = props.Theme.Cursor
	}

	contentWidth := float32(80)
	var content woxwidget.Widget = woxwidget.Align{Width: contentWidth, Height: 22, Vertical: 0.5, Child: woxwidget.Text{
		Value: props.Placeholder, Style: woxui.TextStyle{Size: 13}, Color: props.Theme.ResultSubtitle,
	}}
	if props.Hold && len(props.Labels) > 0 {
		label := strings.TrimSpace(props.HoldPrefix + " " + strings.Join(props.Labels, " + "))
		contentWidth = float32(len([]rune(label)))*8 + 2
		if props.Window != nil {
			if metrics, err := props.Window.MeasureText(label, woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}); err == nil {
				contentWidth = metrics.Size.Width
			}
		}
		content = woxwidget.Align{Width: contentWidth, Height: 22, Vertical: 0.5, Child: woxwidget.Text{
			Value: label, Style: woxui.TextStyle{Size: 13, Weight: woxui.FontWeightSemibold}, Color: props.Theme.ActionText,
		}}
	} else if len(props.Labels) > 0 {
		content, contentWidth = WoxHotkey(HotkeyProps{
			// Flutter's recorder uses the app's default Material canvas rather than the launcher theme,
			// so key legends stay light and keyboard-like on both light and dark Wox surfaces.
			Labels: props.Labels, Foreground: woxui.Color{R: 33, G: 33, B: 33, A: 255}, Background: woxui.Color{R: 250, G: 250, B: 250, A: 255},
			Border: woxui.Color{R: 0, G: 0, B: 0, A: 31}, Compact: true, Window: props.Window,
		})
	}

	width := contentWidth + 16
	contentBox := woxwidget.Container{
		Width: width, Height: 30, Padding: woxwidget.Insets{Left: 8, Top: 4, Right: 8, Bottom: 4},
		BorderColor: border, BorderWidth: 1, Radius: 4, Child: content,
	}
	key := woxwidget.Key(props.ID)
	return woxwidget.Stateful{
		Key: key, Type: (*hotkeyRecorderFocusState)(nil), Widget: hotkeyRecorderFocusWidget{Props: props, Child: contentBox},
		CreateState: func() woxwidget.State { return &hotkeyRecorderFocusState{} },
	}, width
}

type hotkeyRecorderFocusWidget struct {
	Props HotkeyRecorderProps
	Child woxwidget.Widget
}

type hotkeyRecorderFocusState struct {
	focusNode  *woxwidget.FocusNode
	attachment *woxwidget.FocusAttachment
	key        woxwidget.Key
}

func (s *hotkeyRecorderFocusState) InitState(context woxwidget.StateContext, widget any) {
	s.focusNode = woxwidget.NewFocusNode()
	s.updateBinding(context, widget.(hotkeyRecorderFocusWidget).Props.ID)
	if widget.(hotkeyRecorderFocusWidget).Props.Focused {
		context.PostFrame(func() { s.focusNode.RequestFocus() })
	}
}

func (s *hotkeyRecorderFocusState) DidUpdateWidget(context woxwidget.StateContext, oldWidget, newWidget any) {
	oldProps := oldWidget.(hotkeyRecorderFocusWidget).Props
	props := newWidget.(hotkeyRecorderFocusWidget).Props
	s.updateBinding(context, props.ID)
	if oldProps.Focused == props.Focused {
		return
	}
	if props.Focused {
		context.PostFrame(func() { s.focusNode.RequestFocus() })
	} else {
		context.PostFrame(func() { s.focusNode.Unfocus() })
	}
}

func (s *hotkeyRecorderFocusState) Build(context woxwidget.StateContext, widget any) woxwidget.Widget {
	config := widget.(hotkeyRecorderFocusWidget)
	s.updateBinding(context, config.Props.ID)
	return woxwidget.Focusable{
		Key: s.key, UnfocusOnPointerOutside: true,
		// Keep recorder navigation local so Enter and Escape cannot fall through to page actions.
		OnKey: func(event woxui.KeyEvent) bool {
			if event.Key == woxui.KeyEscape {
				return true
			}
			if event.Down && !event.Composing && event.Key == woxui.KeyEnter && event.Modifiers == 0 {
				if !s.focusNode.MoveFocus(false) {
					s.focusNode.Unfocus()
				}
				return true
			}
			return false
		},
		OnFocusChange: func(focused bool) {
			s.focusNode.UpdateFocus(focused)
			if config.Props.OnFocusChange != nil {
				config.Props.OnFocusChange(focused)
			}
			context.Invalidate()
		},
		Child: woxwidget.Gesture{ID: config.Props.ID, OnTap: func() { s.focusNode.RequestFocus() }, Child: config.Child},
	}
}

func (s *hotkeyRecorderFocusState) Dispose() {
	if s.attachment != nil {
		s.attachment.Detach()
		s.attachment = nil
	}
}

// updateBinding keeps the recorder's stable FocusNode attached when its retained widget identity changes.
func (s *hotkeyRecorderFocusState) updateBinding(context woxwidget.StateContext, id string) {
	key := woxwidget.Key(id)
	if s.attachment != nil && s.key == key {
		return
	}
	if s.attachment != nil {
		s.attachment.Detach()
	}
	s.key = key
	s.attachment = context.BindFocusNode(s.focusNode, key)
}
