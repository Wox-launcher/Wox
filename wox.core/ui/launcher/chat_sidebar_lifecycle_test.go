package launcher

import (
	"context"
	"testing"
	"time"

	woxui "wox/ui/runtime"
)

type delayedSidebarServices struct {
	chatHistoryTestServices
	writes  chan bool
	release chan struct{}
}

func (s delayedSidebarServices) SetChatSidebarOpen(_ context.Context, _ string, open bool) error {
	s.writes <- open
	<-s.release
	return nil
}

func TestChatSidebarRestoresSavedChoice(t *testing.T) {
	for _, open := range []bool{false, true} {
		app := New(false, nil)
		previewData := `{"ActiveChat":{"Id":"chat"},"SidebarOpen":false}`
		if open {
			previewData = `{"ActiveChat":{"Id":"chat"},"SidebarOpen":true}`
		}
		err := app.chat.activateChatPreview(queryResult{ID: "result", QueryID: "query"}, queryPreview{PreviewType: "chat", PreviewData: previewData})
		if err != nil {
			app.cancel()
			t.Fatal(err)
		}
		if got := chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen); got != open {
			app.cancel()
			t.Fatalf("restored sidebar = %v, want %v", got, open)
		}
		app.cancel()
	}
}

func TestChatSidebarChoiceSerializesWrites(t *testing.T) {
	service := delayedSidebarServices{writes: make(chan bool, 2), release: make(chan struct{}, 2)}
	dispatched := make(chan func(), 2)
	app := New(false, nil)
	defer app.cancel()
	preview := queryPreview{PreviewType: "chat", PreviewData: `{"ActiveChat":{"Id":"chat"},"SidebarOpen":false}`}
	result := queryResult{ID: "result", QueryID: "query"}
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	app.chat.services = service
	app.chat.runOnUI = func(_ string, fn func()) error { dispatched <- fn; return nil }
	app.chat.toggleChatPanel("history")
	if !<-service.writes {
		t.Fatal("first write did not open the sidebar")
	}
	app.chat.toggleChatPanel("history")
	app.chat.resetChatPreview()
	preview.PreviewData = `{"ActiveChat":{"Id":"chat"},"SidebarOpen":true}`
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	if chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen) {
		t.Fatal("stale preview overwrote the pending sidebar choice")
	}
	service.release <- struct{}{}
	(<-dispatched)()
	if <-service.writes {
		t.Fatal("last write did not close the sidebar")
	}
	service.release <- struct{}{}
	(<-dispatched)()
	if app.chat.chatHistoryChoiceSaving {
		t.Fatal("sidebar choice remained pending")
	}
}

func TestChatSidebarSurvivesLauncherBlur(t *testing.T) {
	hidden := make(chan struct{}, 1)
	app := New(false, chatWindowTestServices{hidden: hidden})
	defer app.cancel()
	app.uiCall = nil
	app.visible = true
	app.show.HideOnBlur = true
	preview := queryPreview{PreviewType: "chat", PreviewData: `{"ActiveChat":{"Id":"chat"}}`}
	result := queryResult{ID: "result", QueryID: "query"}
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	app.chat.toggleChatPanel("history")
	app.onFocus(woxui.FocusEvent{Active: false})
	select {
	case <-hidden:
	case <-time.After(5 * time.Second):
		t.Fatal("launcher blur did not finish hiding")
	}
	if err := app.chat.activateChatPreview(result, preview); err != nil {
		t.Fatal(err)
	}
	if !chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen) {
		t.Fatal("launcher blur closed the chat sidebar")
	}
}

func TestChatSidebarSurvivesPreviewResetAndReplacement(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	preview := queryPreview{PreviewType: "chat", PreviewData: `{"ActiveChat":{"Id":"chat"}}`}
	result := queryResult{ID: "result", QueryID: "query"}
	activate := func() {
		t.Helper()
		if err := app.chat.activateChatPreview(result, preview); err != nil {
			t.Fatal(err)
		}
	}
	activate()
	app.chat.toggleChatPanel("history")
	app.chat.resetChatPreview()
	app.chat.resetChatPreview()
	activate()
	if !chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen) {
		t.Fatal("hiding and reopening chat closed the sidebar")
	}

	// The history drawer remains open behind a floating catalog during replacement.
	app.chat.chatPreview.panel = "models"
	result.ID = "replacement"
	activate()
	if !chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen) {
		t.Fatal("replacing the preview closed the sidebar")
	}

	app.chat.toggleChatPanel("history")
	app.chat.resetChatPreview()
	activate()
	if chatHistoryVisible(app.chat.chatPreview.panel, app.chat.chatPreview.sidebarOpen) {
		t.Fatal("manually closed sidebar reopened after reset")
	}
}
