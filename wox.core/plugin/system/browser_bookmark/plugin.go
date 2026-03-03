package browserbookmark

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"
	"sync"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/plugin/system"
	"wox/setting/definition"
	"wox/util"
	"wox/util/browser"
	"wox/util/shell"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &BrowserBookmarkPlugin{})
}

var browserBookmarkIcon = common.PluginBookmarkIcon

const (
	browserBookmarkIndexBrowsersSettingKey = "indexBrowsers"
)

var browserBookmarkLoaders = map[string]func(*BrowserBookmarkPlugin, context.Context) []Bookmark{}

func registerBrowserBookmarkLoader(browserID string, loader func(*BrowserBookmarkPlugin, context.Context) []Bookmark) {
	if browserID == "" || loader == nil {
		return
	}
	browserBookmarkLoaders[browserID] = loader
}

type Bookmark struct {
	Name string
	Url  string
	Icon common.WoxImage
}

type BrowserBookmarkPlugin struct {
	api         plugin.API
	bookmarks   []Bookmark
	bookmarksMu sync.RWMutex
}

func (c *BrowserBookmarkPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "95d041d3-be7e-4b20-8517-88dda2db280b",
		Name:          "i18n:plugin_browser_bookmark_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_browser_bookmark_plugin_description",
		Icon:          browserBookmarkIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		Commands: []plugin.MetadataCommand{},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
		Features: []plugin.MetadataFeature{
			{
				Name: plugin.MetadataFeatureMRU,
			},
		},
		SettingDefinitions: c.getBrowserBookmarkSettingDefinitions(),
	}
}

func (c *BrowserBookmarkPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	c.api = initParams.API

	c.reloadBookmarks(ctx)

	c.api.OnSettingChanged(ctx, func(callbackCtx context.Context, key string, value string) {
		if key == browserBookmarkIndexBrowsersSettingKey {
			c.reloadBookmarks(callbackCtx)
		}
	})

	c.api.OnMRURestore(ctx, c.handleMRURestore)
}

func (c *BrowserBookmarkPlugin) Query(ctx context.Context, query plugin.Query) (results []plugin.QueryResult) {
	bookmarks := c.getBookmarksSnapshot()
	for _, b := range bookmarks {
		var bookmark = b
		var isMatch bool
		var matchScore int64

		var minMatchScore int64 = 50 // bookmark plugin has strict match score to avoid too many unrelated results

		isNameMatch, nameScore := plugin.IsStringMatchScore(ctx, bookmark.Name, query.Search)

		if isNameMatch && nameScore >= minMatchScore {
			isMatch = true
			matchScore = nameScore
		} else {
			//url match must be exact part match
			contains := strings.Contains(bookmark.Url, query.Search)
			if contains {
				isUrlMatch, urlScore := plugin.IsStringMatchScoreNoPinYin(ctx, bookmark.Url, query.Search)
				if isUrlMatch && urlScore >= minMatchScore {
					isMatch = true
					matchScore = urlScore
				}
			}
		}

		if isMatch {
			// default icon, use cached favicon if exists (no network)
			icon := browserBookmarkIcon
			cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(ctx, bookmark.Url)
			if ok {
				icon = cachedIcon
			}
			results = append(results, plugin.QueryResult{
				Title:    bookmark.Name,
				SubTitle: bookmark.Url,
				Score:    matchScore,
				Icon:     icon,
				Actions: []plugin.QueryResultAction{
					{
						Name: "i18n:plugin_browser_bookmark_open_in_browser",
						ContextData: common.ContextData{
							"name": bookmark.Name,
							"url":  bookmark.Url,
						},
						Action: func(ctx context.Context, actionContext plugin.ActionContext) {
							shell.Open(bookmark.Url)
						},
					},
				},
			})
		}
	}

	return
}

func (c *BrowserBookmarkPlugin) reloadBookmarks(ctx context.Context) {
	bookmarks := c.loadBookmarks(ctx)
	bookmarks = c.removeDuplicateBookmarks(bookmarks)

	c.bookmarksMu.Lock()
	c.bookmarks = bookmarks
	c.bookmarksMu.Unlock()

	c.api.Log(ctx, plugin.LogLevelInfo, fmt.Sprintf("loaded %d bookmarks", len(bookmarks)))

	// Prefetch all bookmark favicons in background without blocking
	urls := make([]string, 0, len(bookmarks))
	for _, b := range bookmarks {
		urls = append(urls, b.Url)
	}
	util.Go(ctx, "prefetch bookmark favicons", func() { c.prefetchWebsiteIcons(ctx, urls) })
}

// PrefetchWebsiteIcons downloads favicons for given URLs in background using Google's service.
// - Deduplicates by hostname
// - Skips if cache already exists
// - Uses short timeout per request
func (c *BrowserBookmarkPlugin) prefetchWebsiteIcons(ctx context.Context, urls []string) {
	// build unique hostnames
	domainSet := map[string]struct{}{}
	for _, raw := range urls {
		if u, err := url.Parse(raw); err == nil {
			if u.Hostname() != "" {
				domainSet[u.Hostname()] = struct{}{}
			}
		}
	}

	jobs := make(chan string, len(domainSet))
	workerCount := 8
	for i := 0; i < workerCount; i++ {
		util.Go(ctx, "prefetch favicon worker", func() {
			for domain := range jobs {
				// compute both http/https cache paths to keep key consistent with getWebsiteIconFromCacheOnly()
				httpKey := "http://" + domain
				httpsKey := "https://" + domain
				httpCache := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", fmt.Sprintf("%x", md5.Sum([]byte(httpKey)))))
				httpsCache := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", fmt.Sprintf("%x", md5.Sum([]byte(httpsKey)))))

				// if both exist, skip
				if _, err1 := os.Stat(httpCache); err1 == nil {
					if _, err2 := os.Stat(httpsCache); err2 == nil {
						continue
					}
				}

				googleFaviconURL := fmt.Sprintf("https://www.google.com/s2/favicons?sz=96&domain_url=%s", url.QueryEscape(domain))
				// ensure https cache
				if _, err := os.Stat(httpsCache); os.IsNotExist(err) {
					gctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
					_ = util.HttpDownload(gctx, googleFaviconURL, httpsCache)
					cancel()
				}
				// ensure http cache (copy from https if available; otherwise download again)
				if _, err := os.Stat(httpCache); os.IsNotExist(err) {
					if _, ok := os.Stat(httpsCache); ok == nil {
						if data, readErr := os.ReadFile(httpsCache); readErr == nil {
							_ = os.WriteFile(httpCache, data, os.ModePerm)
						}
					} else {
						gctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
						_ = util.HttpDownload(gctx, googleFaviconURL, httpCache)
						cancel()
					}
				}
			}
		})
	}
	for d := range domainSet {
		jobs <- d
	}
	close(jobs)
}

func (c *BrowserBookmarkPlugin) getBookmarksSnapshot() []Bookmark {
	c.bookmarksMu.RLock()
	defer c.bookmarksMu.RUnlock()

	return append([]Bookmark(nil), c.bookmarks...)
}

func (c *BrowserBookmarkPlugin) loadBookmarks(ctx context.Context) []Bookmark {
	selectedBrowsers := c.getSelectedBookmarkBrowsers(ctx)
	var bookmarks []Bookmark

	for _, browserID := range selectedBrowsers {
		loader, ok := browserBookmarkLoaders[browserID]
		if !ok {
			continue
		}
		bookmarks = append(bookmarks, loader(c, ctx)...)
	}

	return bookmarks
}

func (c *BrowserBookmarkPlugin) getSelectedBookmarkBrowsers(ctx context.Context) []string {
	installedBrowsers := c.getBookmarkIndexableInstalledBrowsers()
	settingValue := strings.TrimSpace(c.api.GetSetting(ctx, browserBookmarkIndexBrowsersSettingKey))
	return c.resolveSelectedBookmarkBrowsers(settingValue, installedBrowsers)
}

func (c *BrowserBookmarkPlugin) getBrowserBookmarkSettingDefinitions() []definition.PluginSettingDefinitionItem {
	indexableInstalledBrowsers := c.getBookmarkIndexableInstalledBrowsers()
	if len(indexableInstalledBrowsers) == 0 {
		return nil
	}

	settings := []definition.PluginSettingDefinitionItem{
		{
			Type: definition.PluginSettingDefinitionTypeSelect,
			Value: &definition.PluginSettingValueSelect{
				Key:          browserBookmarkIndexBrowsersSettingKey,
				Label:        "i18n:plugin_browser_bookmark_index_browsers",
				Tooltip:      "i18n:plugin_browser_bookmark_index_browsers_tooltip",
				DefaultValue: definition.PluginSettingValueSelectOptionValueSelectAll,
				IsMulti:      true,
				Options:      c.getBookmarkIndexBrowserOptions(indexableInstalledBrowsers),
			},
		},
	}

	return settings
}

func (c *BrowserBookmarkPlugin) getBookmarkIndexBrowserOptions(installedBrowsers []browser.BrowserOption) []definition.PluginSettingValueSelectOption {
	options := []definition.PluginSettingValueSelectOption{
		{
			Label:       "i18n:plugin_browser_bookmark_index_browsers_all",
			Value:       definition.PluginSettingValueSelectOptionValueSelectAll,
			Icon:        common.PluginBrowserIcon,
			IsSelectAll: true,
		},
	}

	for _, localBrowser := range installedBrowsers {
		options = append(options, definition.PluginSettingValueSelectOption{
			Label: localBrowser.Label,
			Value: localBrowser.ID,
			Icon:  localBrowser.Icon,
		})
	}

	return options
}

func (c *BrowserBookmarkPlugin) getBookmarkIndexableInstalledBrowsers() []browser.BrowserOption {
	var browsers []browser.BrowserOption

	for _, localBrowser := range browser.GetInstalledBrowsers() {
		if !c.isBookmarkIndexableBrowser(localBrowser.ID) {
			continue
		}
		browsers = append(browsers, localBrowser)
	}

	return browsers
}

func (c *BrowserBookmarkPlugin) isBookmarkIndexableBrowser(browserID string) bool {
	_, ok := browserBookmarkLoaders[browserID]
	return ok
}

func (c *BrowserBookmarkPlugin) resolveSelectedBookmarkBrowsers(settingValue string, installedBrowsers []browser.BrowserOption) []string {
	if len(installedBrowsers) == 0 {
		return nil
	}

	selectedSet := map[string]struct{}{}
	for _, rawValue := range strings.Split(settingValue, ",") {
		normalized := browser.NormalizeBrowserID(rawValue)
		if normalized == "" {
			continue
		}
		if normalized == definition.PluginSettingValueSelectOptionValueSelectAll {
			return c.getBrowserIDs(installedBrowsers)
		}
		selectedSet[normalized] = struct{}{}
	}

	if len(selectedSet) == 0 {
		return c.getBrowserIDs(installedBrowsers)
	}

	var selected []string
	for _, localBrowser := range installedBrowsers {
		if _, ok := selectedSet[localBrowser.ID]; ok {
			selected = append(selected, localBrowser.ID)
		}
	}

	if len(selected) == 0 {
		return c.getBrowserIDs(installedBrowsers)
	}

	return selected
}

func (c *BrowserBookmarkPlugin) getBrowserIDs(installedBrowsers []browser.BrowserOption) []string {
	var browserIDs []string
	for _, localBrowser := range installedBrowsers {
		browserIDs = append(browserIDs, localBrowser.ID)
	}
	return browserIDs
}

// removeDuplicateBookmarks removes duplicate bookmarks based on name and url
func (c *BrowserBookmarkPlugin) removeDuplicateBookmarks(bookmarks []Bookmark) []Bookmark {
	seen := make(map[string]bool)
	var result []Bookmark

	for _, bookmark := range bookmarks {
		// Create a unique key based on name and url
		key := bookmark.Name + "|" + bookmark.Url

		if !seen[key] {
			seen[key] = true
			result = append(result, bookmark)
		}
	}

	return result
}

func (c *BrowserBookmarkPlugin) handleMRURestore(ctx context.Context, mruData plugin.MRUData) (*plugin.QueryResult, error) {
	name := mruData.ContextData["name"]
	url := mruData.ContextData["url"]
	if url == "" {
		return nil, fmt.Errorf("empty url in context data")
	}

	// Check if bookmark still exists in current bookmarks
	found := false
	for _, bookmark := range c.getBookmarksSnapshot() {
		if bookmark.Name == name && bookmark.Url == url {
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("bookmark no longer exists: %s", name)
	}

	if !mruData.Icon.IsValid() {
		// default icon, use cached favicon if exists (no network)
		icon := browserBookmarkIcon
		if cachedIcon, ok := system.GetWebsiteIconFromCacheOnly(context.Background(), url); ok {
			icon = cachedIcon
		}
		mruData.Icon = icon
	}

	result := &plugin.QueryResult{
		Title:    name,
		SubTitle: url,
		Icon:     mruData.Icon,
		Actions: []plugin.QueryResultAction{
			{
				Name:        "i18n:plugin_browser_bookmark_open_in_browser",
				ContextData: mruData.ContextData,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					shell.Open(url)
				},
			},
		},
	}

	return result, nil
}
