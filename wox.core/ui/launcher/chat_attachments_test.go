package launcher

import (
	"context"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/clipboard"
)

func newChatAttachmentTestApp(t *testing.T) *App {
	t.Helper()
	return bindTestApp(&App{
		sessionID:    "session",
		lifecycleCtx: context.Background(),
		visible:      true,
		chat: chatController{
			chatFullscreen: true,
			chatPreview: &chatPreviewState{
				key:         "preview",
				chat:        chatData{ID: "chat-1", Model: aiModel{Name: "model"}},
				editor:      woxui.NewTextEditor("keep this draft"),
				attachments: []common.AIChatAttachment{{ID: "existing", Kind: common.AIChatAttachmentQuote, Text: "quoted"}},
				active:      true,
			},
		},
		resultList: resultController{layout: queryLayout{ChatMode: true}},
	})
}

func waitUntil(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out")
}

func TestClassifyChatPastePrefersFilesThenBitmapThenText(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 120, 80))
	if classifyChatPaste("C:\\tmp\\a.png", []string{`C:\tmp\a.png`}, img) != chatPasteFiles {
		t.Fatal("copied files must win over a thumbnail or path text")
	}
	if classifyChatPaste("", nil, img) != chatPasteImage {
		t.Fatal("a screenshot without files should import as an image")
	}
	if classifyChatPaste("hello from the browser", nil, img) != chatPasteText {
		t.Fatal("prose next to a bitmap must stay a text paste")
	}
	if classifyChatPaste("plain text", nil, nil) != chatPasteText {
		t.Fatal("ordinary text must keep the default insert")
	}
}

func TestApplyImportedChatAttachmentsPreservesDraft(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	target, ok := app.captureChatAttachmentImportTarget(false)
	if !ok {
		t.Fatal("expected a launcher chat target")
	}
	first := []common.AIChatAttachment{{ID: "one", Kind: common.AIChatAttachmentFile, Name: "a.txt", URL: "a.txt"}}
	app.chat.applyImportedChatAttachments(chatAttachmentImportJob{target: target}, first, nil)
	second := []common.AIChatAttachment{{ID: "two", Kind: common.AIChatAttachmentFile, Name: "b.txt", URL: "b.txt"}}
	app.chat.applyImportedChatAttachments(chatAttachmentImportJob{target: target}, second, nil)
	state := app.chat.chatPreview
	if state.chat.ID != "chat-1" || state.editor.State().Text != "keep this draft" {
		t.Fatal("import changed the conversation or composer text")
	}
	if len(state.attachments) != 3 || state.attachments[0].ID != "existing" || state.attachments[1].ID != "one" || state.attachments[2].ID != "two" {
		t.Fatalf("attachments = %+v", state.attachments)
	}
}

func TestFailedChatAttachmentBatchLeavesDraftUnchanged(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	target, _ := app.captureChatAttachmentImportTarget(false)
	app.chat.applyImportedChatAttachments(chatAttachmentImportJob{target: target}, []common.AIChatAttachment{{ID: "new"}}, os.ErrNotExist)
	if len(app.chat.chatPreview.attachments) != 1 || app.chat.chatPreview.attachments[0].ID != "existing" {
		t.Fatal("failed import mutated the existing draft")
	}
	if app.chat.chatPreview.error == "" {
		t.Fatal("failed import must surface an error")
	}
}

func TestChatAttachmentImportIgnoresStaleTarget(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	target, _ := app.captureChatAttachmentImportTarget(false)
	app.chat.startNewChat()
	app.chat.applyImportedChatAttachments(chatAttachmentImportJob{target: target}, []common.AIChatAttachment{{ID: "late"}}, nil)
	if len(app.chat.chatPreview.attachments) != 0 {
		t.Fatal("a finished import must not attach to a new chat")
	}
}

func TestSendChatMessageBlockedWhileImporting(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.chat.chatPreview.importing = true
	app.chat.sendChatMessage()
	if app.chat.chatPreview.sending || app.chat.chatPreview.chat.IsStreaming || len(app.chat.chatPreview.chat.Conversations) != 0 {
		t.Fatal("send must stay blocked while attachments are importing")
	}
	app.chat.chatPreview.importing = false
	app.chat.chatPreview.editor.SetText("", false)
	app.chat.sendChatMessage()
	if app.chat.chatPreview.error == "" {
		t.Fatal("send should run normal validation after import finishes")
	}
}

func TestRetryChatMessageBlockedWhileImporting(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.chat.chatPreview.importing = true
	app.chat.regenerateChatConversation("reply")
	if app.chat.chatPreview.revision != 0 || app.chat.chatPreview.error != "" || !app.chat.chatPreview.importing {
		t.Fatal("retry must leave an importing draft untouched")
	}
}

// TestChatImportQueueReleasesPayloads checks both consumed slots and the idle queue without GC timing.
func TestChatImportQueueReleasesPayloads(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	target, _ := app.captureChatAttachmentImportTarget(false)
	app.chat.chatPreview.importing = true
	app.chat.chatImportQueue = []chatAttachmentImportJob{{target: target, image: &clipboard.ImageSnapshot{}}}
	backing := app.chat.chatImportQueue
	callbacks := make(chan func())
	done := make(chan struct{})
	defer close(done)
	app.uiCall = func(fn func()) error {
		callbacks <- fn
		<-done
		return nil
	}
	app.chat.startNextChatImportIfIdle()
	if backing[0].image != nil {
		t.Error("consumed queue slot retained the clipboard snapshot")
	}
	select {
	case apply := <-callbacks:
		apply()
	case <-time.After(2 * time.Second):
		t.Fatal("import did not complete")
	}
	if app.chat.chatImportQueue != nil || app.chat.chatImportRunning || app.chat.chatPreview.importing {
		t.Fatal("idle queue must release its backing array and restore send")
	}
}

func TestHandleFileDropAppendsToVisibleChatInsteadOfQuery(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	directory := t.TempDir()
	path := filepath.Join(directory, "note.txt")
	if err := os.WriteFile(path, []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	app.handleFileDrop([]string{path})
	waitUntil(t, func() bool { return !app.chat.chatPreview.importing && len(app.chat.chatPreview.attachments) == 2 })
	if app.chat.chatPreview.chat.ID != "chat-1" || app.chat.chatPreview.editor.State().Text != "keep this draft" {
		t.Fatal("chat drop started a new conversation or replaced the draft")
	}
	if app.chat.chatPreview.attachments[1].Kind != common.AIChatAttachmentFile || app.chat.chatPreview.attachments[1].URL != path {
		t.Fatalf("attached file = %+v", app.chat.chatPreview.attachments[1])
	}
}

func TestHiddenChatIsNotALauncherDropTarget(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.visible = false
	if app.launcherPresentsChatInput() {
		t.Fatal("a hidden launcher must not treat background chat as the drop target")
	}
	app.visible = true
	app.chat.chatFullscreen = false
	app.resultList.layout.ChatMode = false
	if app.launcherPresentsChatInput() {
		t.Fatal("a search result list must not inherit a background chat draft")
	}
}

func TestFailedChatAttachmentImportRestoresSend(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.handleFileDrop([]string{filepath.Join(t.TempDir(), "missing.bin")})
	waitUntil(t, func() bool { return !app.chat.chatPreview.importing && app.chat.chatPreview.error != "" })
	if len(app.chat.chatPreview.attachments) != 1 || app.chat.chatPreview.attachments[0].ID != "existing" {
		t.Fatal("a failed import must leave the existing draft attachments unchanged")
	}
	app.chat.chatPreview.editor.SetText("", false)
	app.chat.sendChatMessage()
	if app.chat.chatPreview.sending || app.chat.chatPreview.importing {
		t.Fatal("send must run after a failed import instead of staying blocked")
	}
	if app.chat.chatPreview.error == "" {
		t.Fatal("send should run normal validation after import failure")
	}
}

func TestClosingDedicatedChatWindowDiscardsImport(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.chatWindow.chatView = &woxui.ManagedWindow{}
	app.chatWindow.chatWindowGeneration = 3
	target, ok := app.captureChatAttachmentImportTarget(true)
	if !ok {
		t.Fatal("expected a dedicated chat import target")
	}
	app.onChatWindowClosed()
	app.chat.applyImportedChatAttachments(chatAttachmentImportJob{target: target}, []common.AIChatAttachment{{ID: "late"}}, nil)
	if len(app.chat.chatPreview.attachments) != 1 || app.chat.chatPreview.attachments[0].ID != "existing" {
		t.Fatal("closing the dedicated window must discard the in-flight import")
	}
}

func TestDedicatedChatDropDoesNotUseLauncherQuery(t *testing.T) {
	app := newChatAttachmentTestApp(t)
	app.visible = false
	app.chat.chatFullscreen = false
	app.resultList.layout.ChatMode = false
	app.chatWindow.chatView = &woxui.ManagedWindow{}
	app.chatWindow.chatWindowGeneration = 3
	directory := t.TempDir()
	path := filepath.Join(directory, "clip.bin")
	if err := os.WriteFile(path, []byte("bin"), 0600); err != nil {
		t.Fatal(err)
	}
	app.handleChatWindowFileDrop([]string{path})
	waitUntil(t, func() bool { return !app.chat.chatPreview.importing && len(app.chat.chatPreview.attachments) == 2 })
	if app.chat.chatPreview.attachments[1].URL != path {
		t.Fatalf("dedicated drop = %+v", app.chat.chatPreview.attachments[1])
	}
}

func TestImportChatAttachmentsRollsBackFailedBatch(t *testing.T) {
	directory := t.TempDir()
	previous := util.GetLocation().GetUserDataDirectory()
	util.GetLocation().UpdateUserDataDirectory(directory)
	t.Cleanup(func() { util.GetLocation().UpdateUserDataDirectory(previous) })
	imagePath := filepath.Join(directory, "ok.png")
	file, err := os.Create(imagePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeTestPNG(file, 2, 2); err != nil {
		t.Fatal(err)
	}
	_ = file.Close()
	_, err = common.ImportChatAttachments([]string{imagePath, filepath.Join(directory, "missing.bin")})
	if err == nil {
		t.Fatal("expected the missing file to fail the batch")
	}
	entries, _ := os.ReadDir(filepath.Join(directory, "chat", "attachments"))
	if len(entries) != 0 {
		t.Fatalf("failed batch left managed files: %v", entries)
	}
}

func writeTestPNG(file *os.File, width, height int) error {
	return png.Encode(file, image.NewRGBA(image.Rect(0, 0, width, height)))
}
