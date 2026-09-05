package launcher

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	woxwidget "wox/ui/widget"
)

func TestDedicatedChatSurvivesLauncherPreviewReplacement(t *testing.T) {
	state := &chatPreviewState{key: "original", chat: chatData{ID: "original"}, active: true, editor: woxui.NewTextEditor("unsent draft")}
	app := New(false, nil)
	defer app.cancel()
	app.chatPreview = state
	app.chatView = &woxui.ManagedWindow{}
	app.visible = true
	app.query = newInputQuery("chat ")
	data, err := json.Marshal(chatPreviewData{ActiveChat: chatData{ID: "replacement"}})
	if err != nil {
		t.Fatal(err)
	}
	preview := queryPreview{PreviewType: "chat", PreviewData: string(data)}
	if err := app.activateChatPreview(queryResult{ID: "new-result", QueryID: "new-query"}, preview); err != nil {
		t.Fatal(err)
	}
	app.applyResults(app.query.QueryID, []queryResult{{ID: "replacement", Preview: preview}}, &queryLayout{}, nil, nil, 0, true)
	if app.chatFullscreen || !app.visible {
		t.Fatal("incoming chat results should not move focus away from the launcher")
	}
	if app.chatPreview != state || state.editor.State().Text != "unsent draft" || !state.active {
		t.Fatal("a launcher result replaced the pop-out conversation or its draft")
	}
	view := app.buildChatPreview(queryResult{}, preview, defaultPalette(), 400, 300, 1, true)
	if _, ok := view.(woxwidget.Stack); !ok {
		t.Fatal("launcher should mount the embedded chat while the dedicated window stays open")
	}
}

func TestChatQueryKeepsEmbeddedAndDedicatedEntrances(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.visible = true
	app.query = newInputQuery("chat ")
	app.chatView = &woxui.ManagedWindow{}
	state := &chatPreviewState{active: true, editor: woxui.NewTextEditor("unsent draft")}
	app.chatPreview = state
	results := []queryResult{{ID: "chat", Preview: queryPreview{PreviewType: "chat", PreviewData: `{"ActiveChat":{"Id":"chat"}}`}}}
	app.applyResults(app.query.QueryID, results, &queryLayout{ChatMode: true}, nil, nil, 0, true)
	if !app.chatFullscreen || !app.visible || app.chatPreview != state || state.editor.State().Text != "unsent draft" {
		t.Fatal("chat query must open embedded chat and preserve the shared draft")
	}
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEnter, Down: true, Modifiers: 0}) || state.error == "" {
		t.Fatal("embedded Enter should reach chat submission validation")
	}
	app.onChatWindowClosed()
	if !app.chatFullscreen || app.chatPreview != state || state.editor.State().Text != "unsent draft" {
		t.Fatal("closing the dedicated entrance discarded the embedded conversation")
	}
	if _, err := app.chatPreviewSnapshotFor(results[0], results[0].Preview); err != nil {
		t.Fatalf("embedded conversation stopped rendering after pop-out close: %v", err)
	}
	app.chatView = &woxui.ManagedWindow{}
	if !app.onChatPreviewKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || app.chatFullscreen || !state.active {
		t.Fatal("embedded Escape should return to the query without deactivating dedicated chat")
	}
	if !app.onDedicatedChatKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) || !app.chatWindowOpen() || !state.active {
		t.Fatal("dedicated Escape should leave the window and shared conversation active")
	}
}

func TestDedicatedChatDoesNotConsumeLauncherShortcuts(t *testing.T) {
	app := New(false, nil)
	defer app.cancel()
	app.chatView = &woxui.ManagedWindow{}
	app.chatPreview = &chatPreviewState{active: true}
	modifier := woxui.KeyModifierControl
	if primaryHotkey("b") == "command+b" {
		modifier = woxui.KeyModifierMeta
	}
	app.onKey(woxui.KeyEvent{Key: "b", Down: true, Modifiers: modifier})
	if app.chatPreview.panel != "" {
		t.Fatal("launcher shortcut changed the independent chat sidebar")
	}
	if app.onChatPreviewTextInput(woxui.TextInputEvent{Kind: woxui.TextInputCommit, Text: "query"}) {
		t.Fatal("dedicated chat intercepted launcher text input")
	}
}

func TestDedicatedChatEscapeDismissesPanelsWithoutClosingWindow(t *testing.T) {
	dispatched := make(chan struct{}, 1)
	app := &App{
		isPrimary: true, lifecycleCtx: context.Background(), chatView: &woxui.ManagedWindow{},
		chatPreview: &chatPreviewState{active: true, editor: woxui.NewTextEditor("unsent draft")}, editor: woxui.NewTextEditor("query"),
		uiCall: func(func()) error { dispatched <- struct{}{}; return nil },
	}
	for _, panel := range []string{"", "history", "models", "skills", chatCommandPanel, "debug"} {
		app.chatPreview.panel = panel
		if !app.onDedicatedChatKey(woxui.KeyEvent{Key: woxui.KeyEscape, Down: true}) {
			t.Fatalf("dedicated window did not handle Escape for panel %q", panel)
		}
		if app.chatPreview.panel != "" || !app.chatPreview.active || app.chatPreview.editor.State().Text != "unsent draft" {
			t.Fatalf("Escape must dismiss panel %q without changing composer state", panel)
		}
	}
	select {
	case <-dispatched:
		t.Fatal("Escape dispatched a window close")
	case <-time.After(100 * time.Millisecond):
	}
	if !app.chatPreview.active || app.editor.State().Selection.Anchor != app.editor.State().Selection.Focus {
		t.Fatal("dedicated Escape changed launcher input state")
	}
}

func TestDedicatedChatKeepsSecondarySessionAliveOnLauncherBlur(t *testing.T) {
	hidden := make(chan struct{}, 1)
	app := New(false, chatWindowTestServices{hidden: hidden})
	defer app.cancel()
	app.isPrimary = false
	app.visible = true
	app.show.HideOnBlur = true
	app.chatView = &woxui.ManagedWindow{}
	app.chatPreview = &chatPreviewState{active: true}
	app.onFocus(woxui.FocusEvent{Active: false})
	select {
	case <-hidden:
	case <-time.After(5 * time.Second):
		t.Fatal("launcher blur did not finish hiding its session")
	}
	if app.destroyed.Load() || app.visible || !app.chatPreview.active {
		t.Fatal("launcher blur must keep the pop-out's session and conversation alive")
	}
}

type chatWindowTestServices struct {
	contract.Services
	hidden chan struct{}
}

func (s chatWindowTestServices) Hidden(context.Context, string) error {
	s.hidden <- struct{}{}
	return nil
}

func TestPreviewTooltipWindowStaysOnLauncherUntilChatWindowIsFocused(t *testing.T) {
	launcher := &woxui.Window{}
	app := &App{window: launcher, visible: true, chatView: &woxui.ManagedWindow{}, chatWindowFocused: false}
	if app.previewTooltipWindow() != launcher {
		t.Fatal("unfocused dedicated chat must keep preview tooltips on the launcher")
	}
	app.chatWindowFocused = true
	if app.previewTooltipWindow() != launcher {
		t.Fatal("a focused chat window without a live native surface must fall back to the launcher")
	}
	app.chatWindowFocused = false
	app.visible = false
	if app.previewTooltipWindow() != launcher {
		t.Fatal("a hidden launcher without a live chat surface must fall back to the launcher window")
	}
}

func TestChatWindowOpenDetectsLiveInstance(t *testing.T) {
	app := &App{}
	if app.chatWindowOpen() || app.chatSurfaceVisible() {
		t.Fatal("empty app reported a visible chat surface")
	}
	app.chatView = &woxui.ManagedWindow{}
	if !app.chatWindowOpen() || !app.chatSurfaceVisible() {
		t.Fatal("created dedicated window should keep the chat surface visible")
	}
}

func TestOpenDedicatedChatWindowRequiresActiveChat(t *testing.T) {
	app := &App{}
	if err := app.openDedicatedChatWindow(); err == nil {
		t.Fatal("expected an error when no conversation is active")
	}
}

func TestResetChatPreviewKeepsStateWhileDedicatedWindowOpen(t *testing.T) {
	app := &App{
		chatPreview:    &chatPreviewState{key: "chat", active: true, chat: chatData{ID: "1", Title: "Hello"}},
		chatFullscreen: true,
		chatView:       &woxui.ManagedWindow{},
	}
	app.resetChatPreview()
	if app.chatPreview == nil || app.chatPreview.chat.ID != "1" {
		t.Fatal("reset discarded the conversation while the dedicated window is open")
	}
	if app.chatFullscreen {
		t.Fatal("launcher fullscreen lingered after reset behind the dedicated window")
	}
}

func TestDeactivateChatPreviewKeepsActiveConversationInDedicatedWindow(t *testing.T) {
	app := &App{
		chatPreview:    &chatPreviewState{key: "chat", active: true},
		chatFullscreen: true,
		chatView:       &woxui.ManagedWindow{},
	}
	app.deactivateChatPreview()
	if app.chatPreview == nil || !app.chatPreview.active {
		t.Fatal("dedicated window conversation was deactivated")
	}
	if app.chatFullscreen {
		t.Fatal("launcher fullscreen lingered after deactivate")
	}
}

func TestChatWindowTitleBarIncludesCaptionControls(t *testing.T) {
	app := &App{palette: defaultPalette(), chatPreview: &chatPreviewState{key: "chat", chat: chatData{Title: "Hello"}}}
	bar := app.buildChatWindowTitleBar(800, true, app.palette.componentTheme()).(woxwidget.Stack)
	var chrome woxcomponent.WindowCloseChromeProps
	found := false
	for _, child := range bar.Children {
		stateful, ok := child.Child.(woxwidget.Stateful)
		if !ok {
			continue
		}
		props, ok := stateful.Widget.(woxcomponent.WindowCloseChromeProps)
		if !ok {
			continue
		}
		chrome = props
		found = true
	}
	if !found {
		t.Fatal("dedicated chat window is missing platform caption controls")
	}
	if chrome.OnMinimize == nil || chrome.OnMaximize == nil || chrome.OnClose == nil {
		t.Fatalf("chat window chrome = %+v, want minimize, maximize, and close", chrome)
	}
	var headerDrag woxwidget.Gesture
	foundDrag := false
	for _, child := range bar.Children {
		if gesture, ok := child.Child.(woxwidget.Gesture); ok && gesture.ID == "chat-window-title-drag" {
			headerDrag = gesture
			foundDrag = true
			break
		}
	}
	if !foundDrag || headerDrag.OnDragStart == nil || headerDrag.OnDoubleTap == nil {
		t.Fatal("dedicated chat title bar is missing a window drag and double-click maximize gesture")
	}
	header, ok := findChatWindowHeader(bar)
	if !ok {
		t.Fatal("dedicated chat title bar is missing the conversation header")
	}
	background := header.Child.(woxwidget.Stack).Children[0].Child.(woxwidget.Gesture)
	if background.ID != "chat-titlebar-drag-chat" || background.OnDragStart == nil {
		t.Fatal("dedicated chat header does not drag the chat window")
	}
}

func findChatWindowHeader(bar woxwidget.Stack) (woxwidget.Container, bool) {
	for _, child := range bar.Children {
		container, ok := child.Child.(woxwidget.Container)
		if !ok {
			continue
		}
		stack, ok := container.Child.(woxwidget.Stack)
		if !ok || len(stack.Children) == 0 {
			continue
		}
		gesture, ok := stack.Children[0].Child.(woxwidget.Gesture)
		if ok && gesture.ID == "chat-titlebar-drag-chat" {
			return container, true
		}
	}
	return woxwidget.Container{}, false
}

func TestHideLauncherAfterChatPopOutLeavesConversation(t *testing.T) {
	app := &App{
		visible:        true,
		chatFullscreen: true,
		chatPreview:    &chatPreviewState{key: "chat", active: true, chat: chatData{ID: "1"}},
	}
	app.hideLauncherAfterChatPopOut()
	if app.visible || app.chatFullscreen {
		t.Fatal("launcher should hide after popping chat into its own window")
	}
	if app.chatPreview == nil || !app.chatPreview.active {
		t.Fatal("pop-out hid the conversation instead of continuing it")
	}
}
