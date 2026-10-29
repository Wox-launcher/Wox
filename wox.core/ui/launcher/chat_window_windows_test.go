//go:build windows

package launcher

import (
	"fmt"
	"testing"

	"wox/common"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

// TestDedicatedChatSidebarResizesWindow checks the real native minimum on open and close.
func TestDedicatedChatSidebarResizesWindow(t *testing.T) {
	err := woxui.Run(func() error {
		app := New(false, nil)
		defer app.cancel()
		defer app.windows.CloseAll()
		frames := 0
		managed, _, err := app.windows.Open(chatWindowID, woxui.WindowOptions{Title: "Sidebar resize test", Size: woxui.Size{Width: chatWindowMinimumWidth, Height: chatWindowMinimumHeight}, MinSize: chatWindowNativeMinSize(), Resizable: true, OnFrame: func(_ *woxui.DisplayList, _ woxui.FrameInfo) {
			frames++
			if app.chatWindow.chatView != nil && app.chatWindow.sidebarMinimumWidth != app.chatWindow.sidebarMinSize().Width {
				app.chatWindow.syncSidebarMinSize()
			}
		}})
		if err != nil {
			return err
		}
		app.chatWindow.chatView = managed
		app.chat.chatPreview = &chatPreviewState{sidebarOpen: true}
		if _, err := managed.Show(); err != nil {
			return err
		}
		app.chatWindow.syncSidebarMinSize()
		bounds, err := managed.Window().Bounds()
		if err != nil {
			return err
		}
		if bounds.Width < chatWindowMinimumWidth+chatHistorySidebarWidth {
			return fmt.Errorf("open sidebar width %g", bounds.Width)
		}
		if frames == 0 {
			return fmt.Errorf("native resize did not render a frame")
		}
		app.chat.chatPreview.sidebarOpen = false
		app.chatWindow.syncSidebarMinSize()
		bounds.Width = chatWindowMinimumWidth
		if err := managed.Window().SetBounds(bounds); err != nil {
			return err
		}
		bounds, err = managed.Window().Bounds()
		if err != nil {
			return err
		}
		if bounds.Width > chatWindowMinimumWidth {
			return fmt.Errorf("closed sidebar width %g", bounds.Width)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestDedicatedChatRendersWithoutLauncherMeasurer reproduces unavailable launcher text resources without a 30-second timer.
func TestDedicatedChatRendersWithoutLauncherMeasurer(t *testing.T) {
	err := woxui.Run(func() error {
		app := New(false, nil)
		defer app.cancel()
		defer app.windows.CloseAll()
		chat, _, err := app.windows.Open(chatWindowID, woxui.WindowOptions{Title: "Chat rendering test", Size: woxui.Size{Width: 900, Height: 680}})
		if err != nil {
			return err
		}
		app.chatWindow.chatView = chat
		// An uninitialized launcher deterministically rejects measurement, including
		// cache hits, just as a trimmed renderer rejects new native measurements.
		app.window = &woxui.Window{}
		app.chat.chatPreview = &chatPreviewState{key: "test", active: true, editor: woxui.NewTextEditor(""), chat: chatData{
			ID: "test", Conversations: []chatConversation{{ID: "answer", Role: "assistant", Text: "Answer\n\n| Column |\n| --- |\n| Value |"}},
		}}
		for _, scale := range []float32{1, 1.5, 2} {
			// Exercise the actual pop-out adapter, including its PrepareMessages callback.
			app.buildActiveChatPreview(900, 640, scale, false)
			if len(app.chat.chatMarkdown.entries) != 1 {
				return fmt.Errorf("expected one prepared Markdown reply, got %d", len(app.chat.chatMarkdown.entries))
			}
			for _, entry := range app.chat.chatMarkdown.entries {
				if entry.layout.window != chat.Window() {
					return fmt.Errorf("dedicated reply measured against the unavailable launcher at scale %g", scale)
				}
			}
			messages := app.chatMessagesProps(chat.Window(), snapshotChatPreviewLocked(app.chat.chatPreview), defaultPalette(), 800, 500, scale)
			message := messages.Messages[0]
			if message.Window != chat.Window() || message.Markdown == nil || message.Markdown.Window != chat.Window() {
				return fmt.Errorf("reply lost its dedicated host at scale %g", scale)
			}
			host := woxwidget.NewHost(func(woxui.FrameInfo) woxwidget.Widget { return woxcomponent.WoxMarkdown(*message.Markdown) })
			host.Attach(chat.Window())
			list := &woxui.DisplayList{RasterScale: scale}
			host.Frame(list, woxui.FrameInfo{Size: woxui.Size{Width: 800, Height: 500}, Scale: scale})
			host.Dispose()
			if list.TextDrawCount() != 3 {
				return fmt.Errorf("Markdown paragraph and table recorded %d text draws, want 3 at scale %g", list.TextDrawCount(), scale)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// TestChatMetadataAndDetailsUseHostWindow covers paths outside the Markdown body with an unavailable launcher.
func TestChatMetadataAndDetailsUseHostWindow(t *testing.T) {
	err := woxui.Run(func() error {
		app := New(false, nil)
		defer app.cancel()
		defer app.windows.CloseAll()
		managed, _, err := app.windows.Open(chatWindowID, woxui.WindowOptions{Title: "Chat layout test"})
		if err != nil {
			return err
		}
		window := managed.Window()
		app.window = &woxui.Window{}
		empty := app.chatMessagesProps(window, &chatPreviewSnapshot{}, defaultPalette(), 400, 300, 1)
		if empty.EmptyTextWidth <= 0 || empty.EmptyTextHeight <= 0 {
			return fmt.Errorf("empty-state text was measured against the launcher")
		}
		message := app.chatMessageProps(window, "test", 0, chatConversation{
			Role: "user", Text: "{skill:review} question", Timestamp: 1234,
			Attachments: []common.AIChatAttachment{{ID: "quote", Kind: common.AIChatAttachmentQuote, Text: "Quoted text"}},
		}, defaultPalette(), 400, true, false, 1)
		if message.TimestampWidth <= 0 || message.ContentWidth <= 0 || len(message.RichRuns) != 1 || len(message.Attachments) != 1 {
			return fmt.Errorf("message metadata or mention chip lost host measurements")
		}
		tool := chatConversation{ID: "tool", Role: "tool", ToolCallInfo: chatToolCallInfo{ID: "call", Name: "web_fetch", Status: "succeeded", Response: "Tool response", StartTimestamp: 1000, EndTimestamp: 1250}}
		activity := app.chatToolActivityProps(window, chatRenderItem{roundID: "round", roundExpanded: true, tools: []chatConversation{tool}}, map[string]bool{"tool": true}, defaultPalette(), 400)
		if activity.ToolSummaryWidth <= 0 || len(activity.Tools) != 1 {
			return fmt.Errorf("tool summary lost host measurements")
		}
		call := activity.Tools[0]
		if call.NameWidth <= 0 || call.DurationWidth <= 0 || len(call.Details) == 0 {
			return fmt.Errorf("tool badge or details lost host measurements")
		}
		app.chatMessageProps(window, "legacy-tool", 0, tool, defaultPalette(), 400, true, false, 1)
		for key, cached := range app.previews.previewLayouts {
			if cached.key.window != window {
				return fmt.Errorf("layout %q used another window", key)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
