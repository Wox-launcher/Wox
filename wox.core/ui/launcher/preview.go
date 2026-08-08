package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"sort"
	"strings"
	"time"

	"wox/util"
)

type previewListData struct {
	Items []previewListItem `json:"items"`
}

type previewListItem struct {
	Icon     *woxImage    `json:"icon"`
	Title    string       `json:"title"`
	Subtitle string       `json:"subtitle"`
	Tails    []resultTail `json:"tails"`
}

// resolvePreview returns the cached remote preview while preserving unresolved remote state for blank rendering.
func (a *App) resolvePreview(preview queryPreview) queryPreview {
	if preview.PreviewType != "remote" {
		return normalizePreviewMetadata(preview)
	}
	key := strings.TrimSpace(preview.PreviewData)
	if key == "" {
		return queryPreview{PreviewType: "text", PreviewData: "Remote preview path is empty"}
	}
	if resolved, ok := a.remotePreviews[key]; ok {
		return normalizePreviewMetadata(resolved)
	}
	return preview
}

// prepareRemotePreview starts one deferred preview request before the next render.
func (a *App) prepareRemotePreview(preview queryPreview) {
	if preview.PreviewType != "remote" {
		return
	}
	key := strings.TrimSpace(preview.PreviewData)
	if key == "" {
		return
	}
	_, loaded := a.remotePreviews[key]
	requested := a.previewRequests[key]
	if !loaded && !requested {
		a.previewRequests[key] = true
		util.Go(a.lifecycleCtx, "load remote preview", func() {
			a.loadRemotePreview(key, preview)
		})
	}
}

func (a *App) loadRemotePreview(path string, fallback queryPreview) {
	resolved := queryPreview{PreviewType: "text", PreviewData: "Unable to load remote preview", PreviewTags: fallback.PreviewTags}
	if !strings.HasPrefix(path, "/") || strings.HasPrefix(path, "//") {
		resolved.PreviewData = "Core returned an invalid remote preview path"
	} else {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		parsed, parseErr := url.Parse(path)
		var err error
		if parseErr != nil || parsed.Path != "/preview" {
			err = fmt.Errorf("invalid remote preview path")
		} else {
			query := parsed.Query()
			loaded, loadErr := a.services.ResultPreview(ctx, a.sessionID, query.Get("sessionId"), query.Get("queryId"), query.Get("id"))
			err = loadErr
			if err == nil {
				tags := make([]previewTag, len(loaded.PreviewTags))
				for index, tag := range loaded.PreviewTags {
					tags[index] = previewTag{Label: tag.Label, Tooltip: tag.Tooltip}
				}
				resolved = queryPreview{
					PreviewType: loaded.PreviewType, PreviewData: loaded.PreviewData, PreviewOverlayData: loaded.PreviewOverlayData,
					PreviewTags: tags, PreviewProperties: loaded.PreviewProperties, ScrollPosition: loaded.ScrollPosition,
				}
			}
		}
		cancel()
		if err != nil {
			resolved = queryPreview{PreviewType: "text", PreviewData: fmt.Sprintf("Unable to load preview: %v", err), PreviewTags: fallback.PreviewTags}
		}
	}
	if err := a.runOnUI("apply remote preview", func() {
		if len(a.remotePreviews) >= 256 {
			// Query IDs make these entries short-lived; a bounded reset avoids an LRU on the frame path.
			a.remotePreviews = map[string]queryPreview{}
			a.previewRequests = map[string]bool{path: true}
		}
		a.remotePreviews[path] = resolved
		a.reconcileSelectedPreview()
		if a.window != nil {
			_ = a.window.Invalidate()
		}
	}); err != nil {
		log.Printf("dispatch remote preview result: %v", err)
	}
}

func normalizePreviewMetadata(preview queryPreview) queryPreview {
	if len(preview.PreviewTags) > 0 || len(preview.PreviewProperties) == 0 {
		return preview
	}
	keys := make([]string, 0, len(preview.PreviewProperties))
	for key := range preview.PreviewProperties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	preview.PreviewTags = make([]previewTag, 0, len(keys))
	for _, key := range keys {
		preview.PreviewTags = append(preview.PreviewTags, previewTag{Label: preview.PreviewProperties[key], Tooltip: key})
	}
	return preview
}

func parsePreviewImage(value string) (woxImage, bool) {
	imageType, imageData, ok := strings.Cut(value, ":")
	imageType = strings.TrimSpace(imageType)
	if !ok || imageType == "" || imageData == "" {
		return woxImage{}, false
	}
	return woxImage{ImageType: imageType, ImageData: imageData}, true
}

func decodePreviewList(value string) (previewListData, error) {
	var data previewListData
	if err := json.Unmarshal([]byte(value), &data); err != nil {
		return previewListData{}, err
	}
	return data, nil
}
