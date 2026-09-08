package plugin

import (
	"context"
	"fmt"
	"wox/common"
	"wox/util"
)

// ConvertIcon resolves a list icon using this plugin's cache ownership.
func (i *Instance) ConvertIcon(ctx context.Context, image common.WoxImage) common.WoxImage {
	return i.convertIcon(ctx, image, common.IconConversion{})
}

// convertIcon blocks stale instances from repopulating the next instance's cache.
func (i *Instance) convertIcon(ctx context.Context, image common.WoxImage, config common.IconConversion) common.WoxImage {
	if i == nil {
		converted, _ := common.ConvertPluginIcon(ctx, image, "", "", config)
		return converted
	}
	i.imageMu.RLock()
	defer i.imageMu.RUnlock()
	if i.imageCacheClosed {
		return common.ImageThumbnailPlaceholderIcon
	}
	converted, err := common.ConvertPluginIcon(ctx, image, i.Metadata.Id, i.PluginDirectory, config)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to convert plugin image: plugin=%s err=%v", i.Metadata.Id, err))
	}
	return converted
}

// closeImageCache waits for conversions and lazy publications before invalidating.
func (i *Instance) closeImageCache(ctx context.Context) {
	i.imageMu.Lock()
	defer i.imageMu.Unlock()
	if i.imageCacheClosed {
		return
	}
	i.imageCacheClosed = true
	if err := common.ClearPluginImageCache(i.Metadata.Id); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to clear plugin image cache: plugin=%s err=%v", i.Metadata.Id, err))
	}
}
