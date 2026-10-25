package launcher

import (
	"context"

	"wox/common"
	woxui "wox/ui/runtime"
)

type chatServices interface {
	Chat(context.Context, string, common.AIChatData) error
	ChatByID(context.Context, string, string) (common.AIChatData, error)
	DefaultChatModel(context.Context, string) (common.Model, error)
	SetDefaultChatModel(context.Context, string, common.Model) error
	DeleteChat(context.Context, string, string) error
	StopChat(context.Context, string, string) (bool, error)
	AnswerAIQuestion(context.Context, string, string, string) error
}

// chatController owns the shared conversation and attachment import lifetime.
// The dedicated window and launcher present this same session.
type chatController struct {
	services                          chatServices
	sessionID                         string
	lifecycleCtx                      context.Context
	runOnUI                           func(string, func()) error
	window                            func() *woxui.Window
	language                          *languageCatalog
	aiSettings                        func() *aiSettingsController
	usePinYin                         func() bool
	isPrimary                         bool
	chatWindowOpen                    func() bool
	chatWindowFocused                 func() bool
	chatSurfaceVisible                func() bool
	chatTextInputWindow               func() *woxui.Window
	launcherChatInputActive           func() bool
	selectedChatVisible               func(string) bool
	captureChatAttachmentImportTarget func(bool) (chatAttachmentImportTarget, bool)
	chatAttachmentImportTargetValid   func(chatAttachmentImportTarget) bool
	invalidateChatSurfaces            func()
	prefetchChatCatalogs              func()
	loadAIModels                      func()
	loadAISkills                      func()
	loadChatPluginMentions            func()
	refreshAIMCPServerToolsLocked     func()
	exitChatMode                      func()
	closePreviewWindow                func()
	restoreQueryTextInput             func()
	chatPreview                       *chatPreviewState
	chatFullscreen                    bool
	chatImportQueue                   []chatAttachmentImportJob
	chatImportRunning                 bool
	chatImportEpoch                   uint64
	chatMarkdown                      chatMarkdownCache
}

// bindChat connects the shared session to its surfaces and shared AI catalogs.
func (a *App) bindChat() {
	c := &a.chat
	c.services = a.services
	c.sessionID = a.sessionID
	c.lifecycleCtx = a.lifecycleCtx
	c.runOnUI = a.runOnUI
	c.window = func() *woxui.Window { return a.window }
	c.language = &a.language
	c.aiSettings = func() *aiSettingsController { return a.aiSettings }
	c.usePinYin = func() bool { return a.generalSettings != nil && a.usePinYin() }
	c.isPrimary = a.isPrimary
	c.chatWindowOpen = a.chatWindow.chatWindowOpen
	c.chatWindowFocused = func() bool { return a.chatWindow.chatWindowFocused }
	c.chatSurfaceVisible = a.chatSurfaceVisible
	c.chatTextInputWindow = a.chatTextInputWindow
	c.launcherChatInputActive = a.launcherChatInputActive
	c.selectedChatVisible = func(resultID string) bool {
		index := a.resultList.selected
		return index >= 0 && index < len(a.resultList.results) && a.resultList.results[index].Preview.PreviewType == "chat" && a.resultList.results[index].ID == resultID
	}
	c.captureChatAttachmentImportTarget = a.captureChatAttachmentImportTarget
	c.chatAttachmentImportTargetValid = a.chatAttachmentImportTargetValid
	c.invalidateChatSurfaces = a.invalidateChatSurfaces
	c.prefetchChatCatalogs = a.prefetchChatCatalogs
	c.loadAIModels = a.loadAIModels
	c.loadAISkills = a.loadAISkills
	c.loadChatPluginMentions = a.loadChatPluginMentions
	c.refreshAIMCPServerToolsLocked = func() { a.aiSettings.refreshAIMCPServerToolsLocked() }
	c.exitChatMode = a.exitChatMode
	c.closePreviewWindow = a.closePreviewWindow
	c.restoreQueryTextInput = a.restoreQueryTextInput
}
