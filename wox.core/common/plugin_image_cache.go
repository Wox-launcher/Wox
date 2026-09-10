package common

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"wox/util"
	"wox/util/imagecache"
	"wox/util/timetracking"
)

// Plugin image generations change output paths after invalidation, so every UI
// cache naturally misses without changing image identity or the wire protocol.
type pluginImageCache struct {
	mu         sync.Mutex
	generation uint64
}

var pluginImageCaches sync.Map

type imageCacheDirectoryKey struct{}

type IconConversion struct {
	Size        int
	AllowLazy   bool
	Diagnostics timetracking.IconConversionDiagnostics
	// CacheScope pins deferred work to its original generation. Empty means current.
	CacheScope string
}

// pluginImageCacheFor uses the validated plugin ID, never an installation path.
func pluginImageCacheFor(pluginID string) (string, *pluginImageCache, error) {
	directory, err := util.GetLocation().GetPluginCacheDirectory(pluginID)
	if err != nil {
		return "", nil, err
	}
	root := filepath.Join(util.GetLocation().GetImageCacheDirectory(), "plugins", filepath.Base(directory))
	state, found := pluginImageCaches.Load(root)
	if !found {
		state, _ = pluginImageCaches.LoadOrStore(root, &pluginImageCache{generation: 1})
	}
	return root, state.(*pluginImageCache), nil
}

// ConvertPluginIcon owns all file artifacts produced for one plugin. The lock
// keeps deletion from racing downloads and writes; different plugins run independently.
func ConvertPluginIcon(ctx context.Context, image WoxImage, pluginID, pluginDirectory string, config IconConversion) (WoxImage, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if pluginID == "" {
		return convertIconWithSize(ctx, image, pluginDirectory, config.Size, config.AllowLazy, config.Diagnostics), nil
	}
	root, state, err := pluginImageCacheFor(pluginID)
	if err != nil {
		return imageThumbnailPlaceholder, err
	}
	// ponytail: serialize conversions per plugin; use per-source locks if contention becomes measurable.
	state.mu.Lock()
	defer state.mu.Unlock()
	directory := filepath.Join(root, strconv.FormatUint(state.generation, 10))
	if config.CacheScope != "" && config.CacheScope != directory {
		return imageThumbnailPlaceholder, fmt.Errorf("plugin image request is stale")
	}
	ctx = context.WithValue(ctx, imageCacheDirectoryKey{}, directory)
	converted := convertIconWithSize(ctx, image, pluginDirectory, config.Size, config.AllowLazy, config.Diagnostics)
	// Plugins such as Quick Jump may already return a core lazy candidate.
	// Attach ownership here so both incoming and newly-created candidates agree.
	if converted.ImageType == WoxImageTypeLazyLoad {
		payload, err := ParseWoxLazyLoadImagePayload(converted)
		if err != nil {
			return imageThumbnailPlaceholder, err
		}
		payload.CacheScope = directory
		data, _ := json.Marshal(payload)
		converted.ImageData = string(data)
	}
	// SVG and GIF files bypass resize. Copy those results too, including images
	// returned from GetCacheFolder, so their UI keys have the same ownership.
	if converted.ImageType == WoxImageTypeAbsolutePath && filepath.Dir(converted.ImageData) != directory {
		converted, err = copyPluginImage(ctx, converted, directory)
	}
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to cache plugin image: plugin=%s err=%v", pluginID, err))
		return imageThumbnailPlaceholder, err
	}
	return converted, nil
}

// ClearPluginImageCache clears disk and memory together and retires deferred work.
func ClearPluginImageCache(pluginID string) error {
	root, state, err := pluginImageCacheFor(pluginID)
	if err != nil {
		return err
	}
	state.mu.Lock()
	defer state.mu.Unlock()
	state.generation++
	err = imagecache.RemoveDirectory(root)
	for _, key := range transparentPaddingBypassCache.Keys() {
		if strings.HasPrefix(key, root+string(filepath.Separator)) {
			transparentPaddingBypassCache.Delete(key)
		}
	}
	return err
}

func imageCacheDirectory(ctx context.Context) string {
	if directory, ok := ctx.Value(imageCacheDirectoryKey{}).(string); ok {
		return directory
	}
	return util.GetLocation().GetImageCacheDirectory()
}

// copyPluginImage preserves original SVG/GIF bytes instead of rasterizing them.
func copyPluginImage(ctx context.Context, source WoxImage, directory string) (WoxImage, error) {
	destination := filepath.Join(directory, "source_"+source.Hash()+filepath.Ext(source.ImageData))
	if info, err := os.Stat(destination); err == nil {
		imagecache.Touch(ctx, destination, info)
		return NewWoxImageAbsolutePath(destination), nil
	}
	input, err := os.Open(source.ImageData)
	if err != nil {
		return WoxImage{}, err
	}
	defer input.Close()
	if err := os.MkdirAll(directory, 0755); err != nil {
		return WoxImage{}, err
	}
	output, err := os.Create(destination)
	if err != nil {
		return WoxImage{}, err
	}
	_, copyErr := io.Copy(output, input)
	closeErr := output.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(destination)
		if copyErr != nil {
			return WoxImage{}, copyErr
		}
		return WoxImage{}, closeErr
	}
	return NewWoxImageAbsolutePath(destination), nil
}
