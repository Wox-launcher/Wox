package view

import (
	"testing"

	woxcomponent "wox/ui/launcher/component"
	previewview "wox/ui/launcher/view/preview"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestNoteAndChatTitleBarAlignment(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{R: 230, A: 255}, ResultSubtitle: woxui.Color{G: 160, A: 255}}
	for _, density := range []float32{0.85, 1, 1.15} {
		note := NotesToolbar(NotesToolbarProps{Width: 800, Platform: "windows", DensityScale: density, Theme: theme, OnSidebar: func() {}}).(woxwidget.Container).Child.(woxwidget.Stack)
		chatBar := ChatWindowTitleBar(ChatWindowTitleBarProps{Width: 800, Platform: "windows", Icon: &woxui.Image{}, Header: &previewview.ChatHeaderProps{DensityScale: density, Theme: theme}}).(woxwidget.Stack)
		chatHeader := chatBar.Children[len(chatBar.Children)-1]
		stateful := chatHeader.Child.(woxwidget.Stateful)
		chat := stateful.CreateState().Build(woxwidget.StateContext{}, stateful.Widget).(woxwidget.Container).Child.(woxwidget.Stack)

		noteTitle := note.Children[1].Child.(woxwidget.Align).Child.(woxwidget.TextBlock)
		chatTitle := chat.Children[2].Child.(woxwidget.Align).Child.(woxwidget.Gesture).Child.(woxwidget.Align).Child.(woxwidget.Text)
		if noteTitle.Style != chatTitle.Style || noteTitle.Color != chatTitle.Color || note.Children[1].Left != chatHeader.Left+chat.Children[2].Left {
			t.Fatalf("density %v: Note title and Chat title do not match", density)
		}
		noteSidebar := note.Children[len(note.Children)-1]
		chatSidebar := chat.Children[1]
		noteButton := noteSidebar.Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		chatButton := chatSidebar.Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		noteGlyph, chatGlyph := noteButton.Icon.(woxwidget.Image), chatButton.Icon.(woxwidget.Image)
		if noteSidebar.Left != chatHeader.Left+chatSidebar.Left || noteButton.Width != chatButton.Width || noteButton.Height != chatButton.Height || noteGlyph.Width != chatGlyph.Width || noteGlyph.Height != chatGlyph.Height {
			t.Fatalf("density %v: Note sidebar button and Chat sidebar button do not match", density)
		}
		icon := chatBar.Children[2]
		image := icon.Child.(woxwidget.Align).Child.(woxwidget.Image)
		if icon.Left != 12 || image.Source == nil || image.Width != 20 || image.Height != 20 {
			t.Fatal("Chat title bar must show its AI icon before the sidebar")
		}
	}
}

func TestNoteAndChatSidebarToggleShareStateIcon(t *testing.T) {
	theme := woxcomponent.Theme{PreviewText: woxui.Color{R: 230, A: 255}, ResultSubtitle: woxui.Color{G: 160, A: 255}}
	for _, open := range []bool{false, true} {
		note := NotesToolbar(NotesToolbarProps{Width: 800, Platform: "windows", SidebarOpen: open, Theme: theme, OnSidebar: func() {}}).(woxwidget.Container).Child.(woxwidget.Stack)
		noteButton := note.Children[len(note.Children)-1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		header := previewview.ChatHeader(previewview.ChatHeaderProps{Width: 500, Height: 40, SidebarOpen: open, Theme: theme}).(woxwidget.Stateful)
		chat := header.CreateState().Build(woxwidget.StateContext{}, header.Widget).(woxwidget.Container).Child.(woxwidget.Stack)
		chatButton := chat.Children[1].Child.(woxwidget.Align).Child.(woxwidget.Stateful).Widget.(woxcomponent.IconButtonProps)
		if noteButton.Icon.(woxwidget.Image).Source != chatButton.Icon.(woxwidget.Image).Source {
			t.Fatalf("Note and Chat use different sidebar icons when open=%v", open)
		}
	}
}
