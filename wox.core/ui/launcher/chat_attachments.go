package launcher

import (
	"context"
	"errors"
	"fmt"
	"image"
	"strings"

	"wox/common"
	woxui "wox/ui/runtime"
	"wox/util"
	"wox/util/clipboard"
)

type chatAttachmentImportTarget struct {
	sessionID        string
	previewKey       string
	chatID           string
	revision         uint64
	dedicated        bool
	windowGeneration uint64
}

type chatAttachmentImportJob struct {
	epoch  uint64
	target chatAttachmentImportTarget
	paths  []string
	image  *clipboard.ImageSnapshot
}

type chatPasteKind int

const (
	chatPasteText chatPasteKind = iota
	chatPasteFiles
	chatPasteImage
)

func cleanFileDropPaths(paths []string) []string {
	cleaned := make([]string, 0, len(paths))
	for _, path := range paths {
		if path = strings.TrimSpace(path); path != "" {
			cleaned = append(cleaned, path)
		}
	}
	return cleaned
}

func classifyChatPaste(value string, paths []string, img interface{ Bounds() image.Rectangle }) chatPasteKind {
	if len(cleanFileDropPaths(paths)) > 0 {
		return chatPasteFiles
	}
	if img != nil && !chatClipboardTextOutranksImage(value, img) {
		return chatPasteImage
	}
	return chatPasteText
}

// chatClipboardTextOutranksImage matches Notes: keep prose instead of a DIB preview.
func chatClipboardTextOutranksImage(value string, img interface{ Bounds() image.Rectangle }) bool {
	value = strings.TrimSpace(value)
	if value == "" || img == nil {
		return false
	}
	if strings.ContainsAny(value, " \t\n\r") {
		return true
	}
	bounds := img.Bounds()
	return bounds.Dx() < 64 || bounds.Dy() < 64
}

// launcherPresentsChatInput is true only when the launcher window itself is the chat composer.
func (a *App) launcherPresentsChatInput() bool {
	if a == nil || !a.visible || a.chatPreview == nil || !a.chatPreview.active {
		return false
	}
	if a.chatFullscreen {
		return true
	}
	return a.layout.ChatMode
}

func (a *App) captureChatAttachmentImportTarget(dedicated bool) (chatAttachmentImportTarget, bool) {
	state := a.chatPreview
	if state == nil {
		return chatAttachmentImportTarget{}, false
	}
	if dedicated {
		if !a.chatWindowOpen() {
			return chatAttachmentImportTarget{}, false
		}
	} else if !a.launcherPresentsChatInput() {
		return chatAttachmentImportTarget{}, false
	}
	return chatAttachmentImportTarget{
		sessionID:        a.sessionID,
		previewKey:       state.key,
		chatID:           state.chat.ID,
		revision:         state.revision,
		dedicated:        dedicated,
		windowGeneration: a.chatWindowGeneration,
	}, true
}

func (a *App) chatAttachmentImportTargetValid(target chatAttachmentImportTarget) bool {
	if a == nil || a.sessionID != target.sessionID {
		return false
	}
	state := a.chatPreview
	if state == nil || state.key != target.previewKey || state.chat.ID != target.chatID || state.revision != target.revision {
		return false
	}
	if target.dedicated {
		return a.chatWindowOpen() && a.chatWindowGeneration == target.windowGeneration
	}
	return a.launcherPresentsChatInput()
}

func (a *App) chatImportLifecycleCtx() context.Context {
	if a != nil && a.lifecycleCtx != nil {
		return a.lifecycleCtx
	}
	return context.Background()
}

// cancelChatAttachmentImports drops queued batches and invalidates the in-flight result.
func (a *App) cancelChatAttachmentImports() {
	a.chatImportEpoch++
	a.chatImportQueue = nil
	if state := a.chatPreview; state != nil {
		state.importing = false
	}
}

// attachChatComposerFile is the composer plus-button shortcut to the native file picker.
func (a *App) attachChatComposerFile() {
	a.prepareChatComposerFilePick(false)
	a.pickChatComposerAttachment(false)
}

// prepareChatComposerFilePick closes the @ overlay so the native dialog is not covered.
func (a *App) prepareChatComposerFilePick(clearAtToken bool) {
	state := a.chatPreview
	if state == nil {
		return
	}
	if chatOverlayPanel(state.panel) {
		if clearAtToken && state.panel == chatMentionPanel && state.editor != nil {
			replaceChatAtToken(state.editor, "")
		}
		restoreChatHistoryPanelLocked(state)
	}
	state.error = ""
	state.active = true
	a.updateChatTextInput(true)
	a.invalidateChatSurfaces()
}

// pickChatComposerAttachment opens the native file or folder picker and attaches the result.
func (a *App) pickChatComposerAttachment(directory bool) {
	if a == nil || a.chatPreview == nil {
		return
	}
	window := a.chatTextInputWindow()
	if window == nil {
		return
	}
	path, err := window.PickFile(woxui.FileDialogOptions{Directory: directory})
	if err != nil {
		if state := a.chatPreview; state != nil {
			state.error = a.chatAttachmentImportError(err)
			a.invalidateChatSurfaces()
		}
		util.GetLogger().Error(a.chatImportLifecycleCtx(), fmt.Sprintf("chat file picker: %v", err))
		return
	}
	if strings.TrimSpace(path) == "" {
		return
	}
	a.enqueueChatDraftAttachments([]string{path}, nil, a.chatWindowFocused)
}

func (a *App) enqueueChatDraftAttachments(paths []string, img *clipboard.ImageSnapshot, dedicated bool) {
	cleaned := cleanFileDropPaths(paths)
	if len(cleaned) == 0 && img == nil {
		return
	}
	target, ok := a.captureChatAttachmentImportTarget(dedicated)
	if !ok {
		return
	}
	state := a.chatPreview
	state.importing = true
	if state.error != "" {
		state.error = ""
	}
	a.chatImportQueue = append(a.chatImportQueue, chatAttachmentImportJob{
		epoch:  a.chatImportEpoch,
		target: target,
		paths:  cleaned,
		image:  img,
	})
	a.invalidateChatSurfaces()
	a.startNextChatImportIfIdle()
}

func (a *App) startNextChatImportIfIdle() {
	if a.chatImportRunning {
		return
	}
	if len(a.chatImportQueue) == 0 {
		a.chatImportQueue = nil
		if state := a.chatPreview; state != nil && state.importing {
			state.importing = false
			a.invalidateChatSurfaces()
		}
		return
	}
	job := a.chatImportQueue[0]
	// A sliced queue still retains consumed slots and their large clipboard payloads.
	a.chatImportQueue[0] = chatAttachmentImportJob{}
	a.chatImportQueue = a.chatImportQueue[1:]
	a.chatImportRunning = true
	ctx := a.chatImportLifecycleCtx()
	util.Go(ctx, "import chat attachments", func() {
		attachments, err := importChatAttachmentJob(job)
		if dispatchErr := a.runOnUI("apply chat attachment import", func() {
			a.chatImportRunning = false
			a.applyImportedChatAttachments(job, attachments, err)
			a.startNextChatImportIfIdle()
		}); dispatchErr != nil {
			common.RemoveImportedChatAttachments(attachments)
			util.GetLogger().Warn(ctx, fmt.Sprintf("apply chat attachment import: %v", dispatchErr))
		}
	})
}

func importChatAttachmentJob(job chatAttachmentImportJob) ([]common.AIChatAttachment, error) {
	if job.image != nil {
		img, err := job.image.Decode(40_000_000)
		if err != nil {
			kind := common.ChatAttachmentErrorInvalidImage
			if errors.Is(err, clipboard.ErrImageTooManyPixels) {
				kind = common.ChatAttachmentErrorTooManyPixels
			}
			return nil, &common.ChatAttachmentError{Name: "clipboard.png", Kind: kind, Err: err}
		}
		attachment, err := common.ImportChatImage(img)
		if err != nil {
			return nil, err
		}
		return []common.AIChatAttachment{attachment}, nil
	}
	return common.ImportChatAttachments(job.paths)
}

func (a *App) applyImportedChatAttachments(job chatAttachmentImportJob, attachments []common.AIChatAttachment, err error) {
	if job.epoch != a.chatImportEpoch || !a.chatAttachmentImportTargetValid(job.target) {
		common.RemoveImportedChatAttachments(attachments)
		return
	}
	state := a.chatPreview
	if err != nil {
		common.RemoveImportedChatAttachments(attachments)
		state.error = a.chatAttachmentImportError(err)
		util.GetLogger().Error(a.chatImportLifecycleCtx(), fmt.Sprintf("chat attachment import failed: %s", common.ChatAttachmentErrorName(err)))
		a.invalidateChatSurfaces()
		return
	}
	state.attachments = append(state.attachments, attachments...)
	state.error = ""
	a.invalidateChatSurfaces()
}

func (a *App) chatAttachmentImportError(err error) string {
	name := common.ChatAttachmentErrorName(err)
	if name == "" {
		name = "file"
	}
	reason := err.Error()
	if key := common.ChatAttachmentFailureReasonKey(err); key != "" {
		reason = a.translate("i18n:" + key)
	}
	message := a.translate("i18n:plugin_ai_chat_attachment_failed")
	if strings.Contains(message, "%s") {
		return fmt.Sprintf(message, name, reason)
	}
	return fmt.Sprintf("Could not attach %s: %s", name, reason)
}

// pasteChatComposer consumes file or bitmap clipboard payloads for the current draft.
func (a *App) pasteChatComposer(value string) bool {
	if copied, err := clipboard.ReadFilePaths(); err == nil {
		if paths := cleanFileDropPaths(copied); len(paths) > 0 {
			a.enqueueChatDraftAttachments(paths, nil, a.chatWindowFocused)
			return true
		}
	}
	img, imgErr := clipboard.ReadImageSnapshot()
	if imgErr != nil {
		util.GetLogger().Error(a.chatImportLifecycleCtx(), "chat paste: read image failed")
		if value != "" {
			return false
		}
		if state := a.chatPreview; state != nil {
			state.error = a.chatAttachmentImportError(&common.ChatAttachmentError{Name: "clipboard.png", Kind: common.ChatAttachmentErrorInvalidImage, Err: imgErr})
			a.invalidateChatSurfaces()
		}
		return true
	}
	if img != nil && classifyChatPaste(value, nil, img) == chatPasteImage {
		a.enqueueChatDraftAttachments(nil, img, a.chatWindowFocused)
		return true
	}
	return false
}
