package system

import (
	"context"
	"fmt"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util/filesearchservice"
)

// refreshServiceUpdateNotice retains upgrade state without probing SCM on every index event.
func (c *FileSearchPlugin) refreshServiceUpdateNotice(ctx context.Context, status filesearchservice.Status) {
	c.toolbarMsgStateMu.Lock()
	c.serviceUpdateStatus = status
	c.toolbarMsgStateMu.Unlock()
	if !status.HasUpdate() {
		return
	}
	// Attention preserves the read state for identical content under this version key.
	c.api.PushAttention(ctx, plugin.PushAttentionRequest{
		Key:         "file-index-service-update-" + status.EmbeddedVersion,
		Title:       "i18n:plugin_file_service_update_notice",
		Description: fmt.Sprintf(c.api.GetTranslation(ctx, "plugin_file_service_update_description"), status.InstalledVersion, status.EmbeddedVersion),
		Icon:        &fileIcon,
		Action:      &plugin.AttentionAction{Type: plugin.AttentionActionTypeOpenPluginSettings},
	})
}

// serviceUpdateToolbarMsg keeps pending upgrades visible while background indexing changes.
func (c *FileSearchPlugin) serviceUpdateToolbarMsg(ctx context.Context) (plugin.ToolbarMsg, bool) {
	c.toolbarMsgStateMu.Lock()
	status := c.serviceUpdateStatus
	c.toolbarMsgStateMu.Unlock()
	if !status.HasUpdate() {
		return plugin.ToolbarMsg{}, false
	}
	return plugin.ToolbarMsg{
		Id:    fileSearchToolbarMsgID,
		Title: fmt.Sprintf("%s (%s → %s)", c.api.GetTranslation(ctx, "plugin_file_service_update_notice"), status.InstalledVersion, status.EmbeddedVersion),
		Icon:  fileIcon,
		Actions: []plugin.ToolbarMsgAction{{
			Id: "open-service-settings", Name: "i18n:plugin_file_setting_service_update", Icon: icons.Get(icons.ActionSettings), IsDefault: true, PreventHideAfterAction: true,
			Action: func(ctx context.Context, _ plugin.ToolbarMsgActionContext) {
				plugin.GetPluginManager().GetUI().OpenSettingWindow(ctx, common.SettingWindowContext{Path: "/plugin/setting", Param: PluginID})
			},
		}},
	}, true
}
