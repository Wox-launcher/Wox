package app

import (
	"context"
	"wox/plugin"
	"wox/setting"
	"wox/ui"
)

type CoreServices struct {
	UIManager      *ui.Manager
	SettingManager *setting.Manager
	PluginManager  *plugin.Manager
}

// StartCoreServices is the first bootstrap seam for the native launcher work.
// The initial slice only exposes the current singleton managers without
// changing startup semantics; later slices will move initialization into here.
func StartCoreServices(ctx context.Context, serverPort int) (*CoreServices, error) {
	_ = ctx
	_ = serverPort

	return &CoreServices{
		UIManager:      ui.GetUIManager(),
		SettingManager: nil,
		PluginManager:  plugin.GetPluginManager(),
	}, nil
}
