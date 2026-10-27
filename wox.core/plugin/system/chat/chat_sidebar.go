package chat

import (
	"context"
	"errors"

	"wox/util"

	"gorm.io/gorm"
)

const chatSidebarOpenKey = "sidebar_open"

// chatSidebarOpen defaults to the existing closed state for users without a saved choice.
func (r *AIChatPlugin) chatSidebarOpen(ctx context.Context) bool {
	if r.store == nil {
		return false
	}
	var open bool
	if err := r.store.Get(chatSidebarOpenKey, &open); err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		util.GetLogger().Warn(ctx, "AI: load chat sidebar preference: "+err.Error())
	}
	return open
}

// SetChatSidebarOpen persists the preference alongside the other chat choices.
func (r *AIChatPlugin) SetChatSidebarOpen(_ context.Context, open bool) error {
	if r.store == nil {
		return errors.New("AI chat storage is unavailable")
	}
	return r.store.SetWithSync(chatSidebarOpenKey, open, true)
}
