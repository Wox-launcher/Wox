package component

import (
	"context"
	"fmt"
	"time"

	"wox/i18n"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
	"wox/util"
)

const markdownCopyFeedbackDuration = 1200 * time.Millisecond

type markdownCodeCopyProps struct {
	id    string
	code  string
	size  float32
	theme ControlTheme
}

type markdownCodeCopyState struct {
	copied     bool
	generation uint64
	reset      *time.Timer
}

// markdownCodeCopyButton retains feedback independently for each code block.
func markdownCodeCopyButton(props markdownCodeCopyProps) woxwidget.Widget {
	return woxwidget.Stateful{
		Key: woxwidget.Key(props.id + "-feedback"), Type: (*markdownCodeCopyState)(nil), Widget: props,
		CreateState: func() woxwidget.State { return &markdownCodeCopyState{} },
	}
}

func (s *markdownCodeCopyState) InitState(_ woxwidget.StateContext, _ any) {}

func (s *markdownCodeCopyState) DidUpdateWidget(_ woxwidget.StateContext, oldWidget, newWidget any) {
	if oldWidget.(markdownCodeCopyProps).code != newWidget.(markdownCodeCopyProps).code {
		s.clearFeedback()
	}
}

func (s *markdownCodeCopyState) Build(stateContext woxwidget.StateContext, widget any) woxwidget.Widget {
	props := widget.(markdownCodeCopyProps)
	color := withAlpha(props.theme.BodyText, 180)
	iconSize := min(props.size, props.theme.Scaled(14))
	icon := CopyGlyph(iconSize, color)
	label := "selection_copy_to_clipboard"
	if s.copied {
		icon = CheckGlyph(iconSize, color)
		label = "toolbar_copied"
	}
	return WoxIconButton(IconButtonProps{
		ID: props.id, Label: i18n.GetI18nManager().TranslateWox(context.Background(), label),
		Icon: icon, Width: props.size, Height: props.size, Radius: props.theme.Scaled(5),
		HoverBackground: withAlpha(props.theme.BodyText, 30), FocusRingColor: props.theme.Focus,
		OnTap: func() {
			clipboard := currentClipboard()
			if clipboard == nil {
				stateContext.SetState(s.clearFeedback)
				return
			}
			if err := clipboard.WriteText(props.code); err != nil {
				util.GetLogger().Warn(context.Background(), fmt.Sprintf("copy Markdown code block: %v", err))
				stateContext.SetState(s.clearFeedback)
				return
			}
			s.clearFeedback()
			stateContext.SetState(func() { s.copied = true })
			generation := s.generation
			s.reset = time.AfterFunc(markdownCopyFeedbackDuration, func() {
				_ = woxui.Call(func() {
					// A stopped timer may already be queued; only the latest copy can clear feedback.
					if stateContext.Mounted() && s.generation == generation {
						stateContext.SetState(s.clearFeedback)
					}
				})
			})
		},
	})
}

func (s *markdownCodeCopyState) clearFeedback() {
	s.generation++
	s.copied = false
	if s.reset != nil {
		s.reset.Stop()
		s.reset = nil
	}
}

func (s *markdownCodeCopyState) Dispose() { s.clearFeedback() }
