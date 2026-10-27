package launcher

import (
	"context"

	"wox/util"
)

type chatSidebarServices interface {
	SetChatSidebarOpen(context.Context, string, bool) error
}

// rememberChatSidebar serializes saves so rapid toggles persist the latest choice.
func (a *chatController) rememberChatSidebar(open bool) {
	a.chatHistoryOpen = open
	a.chatHistoryInitialized = true
	a.chatHistoryChoiceRevision++
	if a.chatHistoryChoiceSaving {
		return
	}
	a.chatHistoryChoiceSaving = true
	a.saveChatSidebarChoice()
}

func (a *chatController) saveChatSidebarChoice() {
	service, ok := a.services.(chatSidebarServices)
	if !ok {
		a.chatHistoryChoiceSaving = false
		return
	}
	open, revision := a.chatHistoryOpen, a.chatHistoryChoiceRevision
	util.Go(a.lifecycleCtx, "save chat sidebar preference", func() {
		if err := service.SetChatSidebarOpen(context.Background(), a.sessionID, open); err != nil {
			util.GetLogger().Error(context.Background(), "save chat sidebar preference: "+err.Error())
		}
		if err := a.runOnUI("finish saving chat sidebar preference", func() {
			if a.chatHistoryChoiceRevision != revision {
				a.saveChatSidebarChoice()
			} else {
				a.chatHistoryChoiceSaving = false
			}
		}); err != nil {
			util.GetLogger().Error(context.Background(), "dispatch chat sidebar preference: "+err.Error())
		}
	})
}
