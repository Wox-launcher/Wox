package system

import (
	"context"
	"net/url"
	"strings"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/util"
	"wox/util/clipboard"
)

func (c *ClipboardPlugin) clipboardQueryResponse(ctx context.Context, results []plugin.QueryResult, records []ClipboardRecord) plugin.QueryResponse {
	c.scheduleLinkFaviconPrefetch(ctx, records)
	return c.newClipboardQueryResponse(results)
}

// resolveTextRecordIcon uses a cached site favicon for link rows. Queries stay
// local-only; missing icons keep the source app or default text glyph until a
// background prefetch fills the cache.
func (c *ClipboardPlugin) resolveTextRecordIcon(ctx context.Context, record ClipboardRecord, normalizedLink string) common.WoxImage {
	icon := c.getDefaultTextIcon()
	if record.IconData != nil && *record.IconData != "" {
		if iconImage, err := common.ParseWoxImage(*record.IconData); err == nil {
			icon = iconImage
		}
	}
	if normalizedLink == "" {
		return icon
	}
	if cached, ok := system.GetWebsiteIconFromCacheOnly(ctx, normalizedLink); ok {
		return cached
	}
	return icon
}

func (c *ClipboardPlugin) scheduleLinkFaviconPrefetch(ctx context.Context, records []ClipboardRecord) {
	urls := c.collectMissingLinkFaviconURLs(ctx, records)
	if len(urls) == 0 {
		return
	}

	c.backgroundTasks.Add(1)
	util.Go(ctx, "clipboard link favicons", func() {
		defer c.backgroundTasks.Done()
		system.PrefetchWebsiteIcons(ctx, urls)
		for _, link := range urls {
			if _, ok := system.GetWebsiteIconFromCacheOnly(ctx, link); ok {
				c.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				return
			}
		}
	})
}

// collectMissingLinkFaviconURLs returns uncached link hosts that have not been
// requested this session. Each host is marked attempted immediately so a miss
// or failed download is not retried until Wox restarts.
func (c *ClipboardPlugin) collectMissingLinkFaviconURLs(ctx context.Context, records []ClipboardRecord) []string {
	if c.faviconFetchAttempted == nil {
		c.faviconFetchAttempted = util.NewHashMap[string, bool]()
	}

	var urls []string
	seen := map[string]struct{}{}
	for _, record := range records {
		if record.Type != string(clipboard.ClipboardTypeText) || !util.IsUrl(record.Content) {
			continue
		}
		link := util.NormalizeUrl(record.Content)
		if _, ok := system.GetWebsiteIconFromCacheOnly(ctx, link); ok {
			continue
		}
		host := clipboardLinkHostname(link)
		if host == "" {
			continue
		}
		if _, already := seen[host]; already {
			continue
		}
		if c.faviconFetchAttempted.Exist(host) {
			continue
		}
		c.faviconFetchAttempted.Store(host, true)
		seen[host] = struct{}{}
		urls = append(urls, link)
	}
	return urls
}

func clipboardLinkHostname(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.ToLower(parsed.Hostname())
}
