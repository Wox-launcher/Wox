package preview

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestChatMessageUsesContentWidthAndCenteredDisclosureIcon(t *testing.T) {
	action := func() {}
	actions, actionWidth := chatMessageActions(ChatMessageProps{Key: "user", OnCopy: action, OnEdit: action}, true, func(bool) {})
	if len(actions) != 2 || actionWidth != chatMessageActionWidth(ChatMessageProps{OnCopy: action, OnEdit: action}) {
		t.Fatalf("actions = %d, width = %.0f", len(actions), actionWidth)
	}

	user := chatMessageContent(ChatMessageProps{
		Key: "user", Role: "user", ContentWidth: 26, Text: "你好",
		TextLayout: woxwidget.TextBlockLayout{Size: woxui.Size{Width: 800, Height: 19}},
		Theme:      woxcomponent.Theme{SelectedBackground: woxui.Color{A: 255}},
	}, 1000, false, func(bool) {}).(woxwidget.Gesture)
	stack := user.Child.(woxwidget.Stack)
	card := stack.Children[0].Child.(woxwidget.Flex)
	body := card.Children[0].(woxwidget.Container)
	if body.Width != 50 || body.Color.A != 255 {
		t.Fatalf("user bubble = width %.0f, color %#v", body.Width, body.Color)
	}

	round := chatMessageContent(ChatMessageProps{Key: "round", Kind: "round", RoundLabel: "Worked for 0s"}, 1000, false, func(bool) {}).(woxwidget.Gesture)
	row := round.Child.(woxwidget.Container).Child.(woxwidget.Flex)
	if row.CrossAxisAlignment != woxwidget.CrossAxisCenter {
		t.Fatalf("round alignment = %v", row.CrossAxisAlignment)
	}
	if icon, ok := row.Children[0].(woxwidget.Image); !ok || icon.Source == nil {
		t.Fatalf("round icon = %#v", row.Children[0])
	}
}
