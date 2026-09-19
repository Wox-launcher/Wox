package plugin

import (
	"context"
	"strings"
)

// ResolvePluginDisplayName returns a localized plugin name from installed
// instances or the in-memory store catalog. Unknown IDs are returned unchanged.
func ResolvePluginDisplayName(ctx context.Context, pluginID string) string {
	pluginID = strings.TrimSpace(pluginID)
	if pluginID == "" {
		return ""
	}
	if instance := GetPluginManager().GetPluginInstanceById(pluginID); instance != nil {
		if name := strings.TrimSpace(instance.GetName(ctx)); name != "" {
			return name
		}
	}
	if manifest, err := GetStoreManager().GetStorePluginManifestById(ctx, pluginID); err == nil {
		if name := strings.TrimSpace(manifest.GetName(ctx)); name != "" {
			return name
		}
	}
	return pluginID
}
