package plugin

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"wox/common"
	"wox/common/icons"
	"wox/util"

	"github.com/google/uuid"
)

type lazyResultIconEntry struct {
	SessionId       string
	QueryId         string
	ResultId        string
	OriginalIcon    common.WoxImage
	PluginDirectory string
	PluginInstance  *Instance
	CacheScope      string
	TargetSize      int
	CreatedAt       int64

	mu       sync.Mutex
	resolved bool
	icon     common.WoxImage
}

// clearLazyIconsExcept invalidates tokens without doing image I/O or invoking callbacks.
func (s *queryResultStore) clearLazyIconsExcept(sessionId string, keepQueryId string) {
	if sessionId == "" {
		return
	}

	// Lazy image tokens are scoped to the polished query result that created them.
	// When the same launcher session starts a newer query, old tokens should stop
	// resolving so a late UI image request cannot hydrate an icon for stale UI.
	var tokensToDelete []string
	s.lazyResultIcons.Range(func(token string, entry *lazyResultIconEntry) bool {
		if entry != nil && entry.SessionId == sessionId && entry.QueryId != keepQueryId {
			tokensToDelete = append(tokensToDelete, token)
		}
		return true
	})
	for _, token := range tokensToDelete {
		s.lazyResultIcons.Delete(token)
	}
}

// registerLazyIcon retains the source under a query token while keeping a reusable UI cache key.
func (s *queryResultStore) registerLazyIcon(ctx context.Context, pluginInstance *Instance, query Query, resultId string, normalized common.WoxImage, pluginDirectory string, size int, cacheScope string) common.WoxImage {
	if query.SessionId == "" || query.Id == "" || resultId == "" {
		return common.WoxImage{}
	}

	// Store the original normalized image, not a pre-decoded bitmap. The expensive
	// decode/crop/resize work is intentionally deferred until UI asks for this
	// token from a visible result image widget.
	token := uuid.NewString()
	s.lazyResultIcons.Store(token, &lazyResultIconEntry{
		SessionId:       query.SessionId,
		QueryId:         query.Id,
		ResultId:        resultId,
		OriginalIcon:    normalized,
		PluginDirectory: pluginDirectory,
		PluginInstance:  pluginInstance,
		CacheScope:      cacheScope,
		TargetSize:      size,
		CreatedAt:       util.GetSystemTimestamp(),
	})

	pluginName := "<unknown>"
	if pluginInstance != nil {
		pluginName = pluginInstance.GetName(ctx)
	}
	util.GetLogger().Debug(ctx, fmt.Sprintf("<%s> result(%s) icon deferred as lazyloadimage, size: %d", pluginName, resultId, size))
	// The authorization token is query-scoped, while the source hash is stable
	// across queries so UI can reuse an already decoded icon.
	cacheKey := fmt.Sprintf("%s-%s-%d", util.Md5([]byte(cacheScope)), normalized.Hash(), size)
	if normalized.ImageType == common.WoxImageTypeFileIcon {
		// Deferred file icons have not acquired the OS cache's versioned path yet.
		// Match its source-mtime invalidation so an app update cannot reuse old UI pixels.
		if info, err := os.Stat(normalized.ImageData); err == nil {
			cacheKey += fmt.Sprintf("-%d", info.ModTime().UnixNano())
		}
	}
	return common.NewWoxImageLazyLoad(token, cacheKey, icons.Get(icons.StatusImagePlaceholder), size)
}

// loadLazyIcon validates query ownership before resolving and publishing the image.
func (s *queryResultStore) loadLazyIcon(ctx context.Context, token string) (common.WoxImage, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return icons.Get(icons.StatusImagePlaceholder), fmt.Errorf("lazy image token is empty")
	}

	entry, found := s.lazyResultIcons.Load(token)
	if !found || entry == nil {
		return icons.Get(icons.StatusImagePlaceholder), fmt.Errorf("lazy image token not found")
	}

	// Hold the instance read lock through publication, not just conversion.
	if entry.PluginInstance != nil {
		entry.PluginInstance.imageMu.RLock()
		defer entry.PluginInstance.imageMu.RUnlock()
		if entry.PluginInstance.imageCacheClosed {
			return icons.Get(icons.StatusImagePlaceholder), fmt.Errorf("plugin is unloaded")
		}
	}
	resultCache, cacheFound := s.findInSession(entry.SessionId, entry.QueryId, entry.ResultId)
	if !cacheFound {
		s.lazyResultIcons.Delete(token)
		return icons.Get(icons.StatusImagePlaceholder), fmt.Errorf("lazy image result is no longer cached")
	}

	if resultCache.Result.Icon.ImageType != common.WoxImageTypeLazyLoad {
		s.lazyResultIcons.Delete(token)
		if !resultCache.Result.Icon.IsEmpty() {
			return resultCache.Result.Icon, nil
		}
		return icons.Get(icons.StatusImagePlaceholder), nil
	}
	payload, payloadErr := common.ParseWoxLazyLoadImagePayload(resultCache.Result.Icon)
	if payloadErr != nil || payload.Token != token {
		s.lazyResultIcons.Delete(token)
		return icons.Get(icons.StatusImagePlaceholder), fmt.Errorf("lazy image token is stale")
	}

	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.resolved {
		return entry.icon, nil
	}

	startedAt := util.GetSystemTimestamp()
	// Lazy load requests intentionally use the original synchronous converter
	// because this path runs after UI has built an image widget for the
	// result. That keeps the query response fast while still reusing the existing
	// crop/resize/cache behavior for the actual thumbnail artifact.
	pluginID := ""
	if entry.PluginInstance != nil {
		pluginID = entry.PluginInstance.Metadata.Id
	}
	converted, err := common.ConvertPluginIcon(ctx, entry.OriginalIcon, pluginID, entry.PluginDirectory, common.IconConversion{Size: entry.TargetSize, CacheScope: entry.CacheScope})
	if err != nil {
		return icons.Get(icons.StatusImagePlaceholder), err
	}
	// URL GIFs used to stay typed as url because conversion skipped resize.
	// UI cannot decode that type, so they collapsed to the placeholder. The
	// converter now downloads the file first; a remaining url means the fetch failed.
	if converted.IsEmpty() || converted.ImageType == common.WoxImageTypeLazyLoad || converted.ImageType == common.WoxImageTypeUrl {
		converted = icons.Get(icons.StatusImagePlaceholder)
	}

	entry.icon = converted
	entry.resolved = true
	resultCache.Result.Icon = converted
	util.GetLogger().Debug(ctx, fmt.Sprintf("lazy result icon hydrated, result: %s, elapsed: %dms", entry.ResultId, util.GetSystemTimestamp()-startedAt))
	return converted, nil
}
