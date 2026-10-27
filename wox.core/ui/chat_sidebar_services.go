package ui

import (
	"context"
	"errors"

	"wox/common"
	"wox/plugin"
)

// SetChatSidebarOpen saves the sidebar choice through the active chat feature.
func (s *CoreServices) SetChatSidebarOpen(ctx context.Context, sessionID string, open bool) error {
	ctx = uiServiceContext(ctx, sessionID)
	service, ok := plugin.GetPluginManager().GetAIChatPluginChater(ctx).(common.AIChatSidebarSettings)
	if !ok {
		return errors.New("AI chat sidebar settings are unavailable")
	}
	return service.SetChatSidebarOpen(ctx, open)
}
