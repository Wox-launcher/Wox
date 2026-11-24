package system

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image/png"
	"net/url"
	"os"
	"path"
	"time"
	"wox/common"
	"wox/common/icons"
	"wox/i18n"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
	"wox/util/imagecache"
	"wox/util/keyboard"
	"wox/util/websiteicon"
	"wox/util/window"
)

var windowIconCache = util.NewHashMap[string, common.WoxImage]()

// GetWebsiteIconWithCache fetches a favicon or reuses the shared on-disk cache.
func GetWebsiteIconWithCache(ctx context.Context, websiteUrl string) (common.WoxImage, error) {
	return websiteicon.Fetch(ctx, websiteUrl)
}

// GetWebsiteIconFromCacheOnly checks if favicon exists in local cache without any network.
// return (icon, true) if exists; otherwise (zero, false)
func GetWebsiteIconFromCacheOnly(ctx context.Context, websiteUrl string) (common.WoxImage, bool) {
	parseUrl, err := url.Parse(websiteUrl)
	if err != nil {
		return common.WoxImage{}, false
	}
	hostUrl := parseUrl.Scheme + "://" + parseUrl.Host
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(hostUrl)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", iconPathMd5))
	if info, statErr := os.Stat(iconCachePath); statErr == nil {
		imagecache.Touch(ctx, iconCachePath, info)
		return common.NewWoxImageAbsolutePath(iconCachePath), true
	}
	return common.WoxImage{}, false
}

// PrefetchWebsiteIcons downloads favicons for given URLs in background using Google's service.
// - Deduplicates by hostname
// - Skips if cache already exists
// - Uses short timeout per request
func PrefetchWebsiteIcons(ctx context.Context, urls []string) {
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
				// compute both http/https cache paths to keep key consistent with getWebsiteIcon*()
				httpKey := "http://" + domain
				httpsKey := "https://" + domain
				httpCache := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", fmt.Sprintf("%x", md5.Sum([]byte(httpKey)))))
				httpsCache := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", fmt.Sprintf("%x", md5.Sum([]byte(httpsKey)))))

				httpInfo, httpErr := os.Stat(httpCache)
				httpsInfo, httpsErr := os.Stat(httpsCache)
				if httpErr == nil {
					imagecache.Touch(ctx, httpCache, httpInfo)
				}
				if httpsErr == nil {
					imagecache.Touch(ctx, httpsCache, httpsInfo)
				}
				if httpErr == nil && httpsErr == nil {
					continue
				}

				googleFaviconUrl := fmt.Sprintf("https://www.google.com/s2/favicons?sz=96&domain_url=%s", url.QueryEscape(domain))
				// ensure https cache
				if os.IsNotExist(httpsErr) {
					gctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
					_ = util.HttpDownload(gctx, googleFaviconUrl, httpsCache)
					cancel()
				}
				// ensure http cache (copy from https if available; otherwise download again)
				if os.IsNotExist(httpErr) {
					if _, ok := os.Stat(httpsCache); ok == nil {
						if data, rErr := os.ReadFile(httpsCache); rErr == nil {
							_ = os.WriteFile(httpCache, data, os.ModePerm)
						}
					} else {
						gctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
						_ = util.HttpDownload(gctx, googleFaviconUrl, httpCache)
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

func GetActiveWindowIcon(ctx context.Context) (common.WoxImage, error) {
	cacheKey := fmt.Sprintf("%s-%d", window.GetActiveWindowName(), window.GetActiveWindowPid())
	if icon, ok := windowIconCache.Load(cacheKey); ok {
		return icon, nil
	}

	icon, err := window.GetActiveWindowIcon()
	if err != nil {
		return common.WoxImage{}, err
	}

	var buf bytes.Buffer
	encodeErr := png.Encode(&buf, icon)
	if encodeErr != nil {
		return common.WoxImage{}, encodeErr
	}

	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	woxIcon := common.NewWoxImageBase64(fmt.Sprintf("data:image/png;base64,%s", base64Str))
	windowIconCache.Store(cacheKey, woxIcon)
	return woxIcon, nil
}

// GetPasteToActiveWindowAction builds a paste action for the captured foreground
// window. A PID is enough when the window title has not been resolved yet.
func GetPasteToActiveWindowAction(ctx context.Context, api plugin.API, target plugin.QueryEnv, prepareClipboard func(context.Context) error) (plugin.QueryResultAction, error) {
	windowName, windowPid := target.ActiveWindowTitle, target.ActiveWindowPid
	if windowName == "" {
		windowName = window.GetWindowNameByPid(windowPid)
	}
	if windowName == "" && windowPid <= 0 {
		return plugin.QueryResultAction{}, fmt.Errorf("no active window")
	}

	actionName := i18n.GetI18nManager().TranslateWox(ctx, "plugin_ai_command_paste")
	if windowName != "" {
		actionName = fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_paste_to_window"), window.CompactTitle(windowName, window.ActionTitleMaxRunes))
	}

	action := plugin.QueryResultAction{
		Name:                   actionName,
		Icon:                   icons.Get(icons.ActionPaste),
		IsDefault:              true,
		PreventHideAfterAction: util.IsKDEWayland(),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			var err error
			if prepareClipboard != nil {
				err = prepareClipboard(ctx)
			}
			// KDE images are published by the focused launcher. Hide explicitly
			// after publication so activation cannot race the usual async hide.
			if util.IsKDEWayland() {
				api.HideApp(ctx)
			}
			if err != nil {
				api.Log(ctx, plugin.LogLevelError, err.Error())
				api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: err.Error()})
				return
			}
			util.Go(ctx, "paste to active window", func() {
				if err := pasteToActiveWindow(ctx, api, target); err != nil {
					api.Log(ctx, plugin.LogLevelError, err.Error())
					api.ShowToastMsg(ctx, plugin.ShowToastMsgOption{Message: err.Error()})
				}
			})
		},
	}

	if !target.ActiveWindowIcon.IsEmpty() {
		action.TailIcon = target.ActiveWindowIcon
	}

	return action, nil
}

func pasteTextToActiveWindow(ctx context.Context, api plugin.API, target plugin.QueryEnv, text string) error {
	if target.ActiveWindowTitle == "" && target.ActiveWindowPid <= 0 {
		return fmt.Errorf("no active window")
	}
	if text == "" {
		return fmt.Errorf("paste text is empty")
	}
	if err := clipboard.WriteText(text); err != nil {
		return fmt.Errorf("write ai command answer to clipboard failed: %w", err)
	}
	return pasteToActiveWindow(ctx, api, target)
}

// pasteToActiveWindow sends the clipboard contents after the caller has published them.
func pasteToActiveWindow(ctx context.Context, api plugin.API, target plugin.QueryEnv) error {
	if util.IsKDEWayland() {
		return pasteToKDEWindow(ctx, api, target)
	}
	if pid := target.ActiveWindowPid; pid > 0 && !window.ActivateWindowByPid(pid) {
		api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("activate window failed, pid=%d", pid))
	}
	time.Sleep(150 * time.Millisecond)
	if err := keyboard.SimulatePaste(); err != nil {
		return fmt.Errorf("simulate paste clipboard failed, err=%s", err.Error())
	}

	api.Log(ctx, plugin.LogLevelInfo, "simulate paste clipboard success")
	return nil
}
