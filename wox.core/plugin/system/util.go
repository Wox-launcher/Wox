package system

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"image/png"
	"io"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/keyboard"
	"wox/util/window"

	"github.com/disintegration/imaging"
	"github.com/mat/besticon/besticon"
)

type cacheResult struct {
	match bool
	score int64
}

var pinyinMatchCache = util.NewHashMap[string, cacheResult]()
var windowIconCache = util.NewHashMap[string, common.WoxImage]()

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int64) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.UsePinYin.Get() {
		key := term + subTerm
		if result, ok := pinyinMatchCache.Load(key); ok {
			return result.match, result.score
		}
	}

	match, score := util.IsStringMatchScore(term, subTerm, woxSetting.UsePinYin.Get())
	if woxSetting.UsePinYin.Get() {
		key := term + subTerm
		pinyinMatchCache.Store(key, cacheResult{match, score})
	}
	return match, score
}

func IsStringMatchScoreNoPinYin(ctx context.Context, term string, subTerm string) (bool, int64) {
	return util.IsStringMatchScore(term, subTerm, false)
}

func IsStringMatchNoPinYin(ctx context.Context, term string, subTerm string) bool {
	match, _ := util.IsStringMatchScore(term, subTerm, false)
	return match
}

func getWebsiteIconWithCache(ctx context.Context, websiteUrl string) (common.WoxImage, error) {
	parseUrl, err := url.Parse(websiteUrl)
	if err != nil {
		return webSearchIcon, fmt.Errorf("failed to parse url for %s: %s", websiteUrl, err.Error())
	}
	hostUrl := parseUrl.Scheme + "://" + parseUrl.Host

	// check if existed in cache
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(hostUrl)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", iconPathMd5))
	if _, statErr := os.Stat(iconCachePath); statErr == nil {
		return common.WoxImage{
			ImageType: common.WoxImageTypeAbsolutePath,
			ImageData: iconCachePath,
		}, nil
	}

	// 1) Try Google favicon service first (usually returns PNG)
	domain := parseUrl.Hostname()
	googleFaviconUrl := fmt.Sprintf("https://www.google.com/s2/favicons?sz=96&domain_url=%s", url.QueryEscape(domain))
	if downloadErr := util.HttpDownload(ctx, googleFaviconUrl, iconCachePath); downloadErr == nil {
		return common.NewWoxImageAbsolutePath(iconCachePath), nil
	}

	// 2) Fallback to besticon crawler
	option := besticon.WithLogger(besticon.NewDefaultLogger(io.Discard))
	iconFinder := besticon.New(option).NewIconFinder()
	icons, fetchErr := iconFinder.FetchIcons(hostUrl)
	if fetchErr != nil {
		return webSearchIcon, fmt.Errorf("failed to fetch icons for %s: %s", hostUrl, fetchErr.Error())
	}

	if len(icons) == 0 {
		return webSearchIcon, fmt.Errorf("no icons found for %s", hostUrl)
	}

	image, imageEr := icons[0].Image()
	if imageEr != nil {
		return webSearchIcon, fmt.Errorf("failed to get image for %s: %s", hostUrl, imageEr.Error())
	}

	woxImage, woxImageErr := common.NewWoxImage(*image)
	if woxImageErr != nil {
		return webSearchIcon, fmt.Errorf("failed to convert image for %s: %s", hostUrl, woxImageErr.Error())
	}

	// save to cache
	saveErr := imaging.Save(*image, iconCachePath)
	if saveErr != nil {
		return woxImage, fmt.Errorf("failed to save image for %s: %s", hostUrl, saveErr.Error())
	}

	return woxImage, nil
}

// getWebsiteIconFromCacheOnly checks if favicon exists in local cache without any network.
// return (icon, true) if exists; otherwise (zero, false)
func getWebsiteIconFromCacheOnly(ctx context.Context, websiteUrl string) (common.WoxImage, bool) {
	parseUrl, err := url.Parse(websiteUrl)
	if err != nil {
		return common.WoxImage{}, false
	}
	hostUrl := parseUrl.Scheme + "://" + parseUrl.Host
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(hostUrl)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", iconPathMd5))
	if _, statErr := os.Stat(iconCachePath); statErr == nil {
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

				// if both exist, skip
				if _, err1 := os.Stat(httpCache); err1 == nil {
					if _, err2 := os.Stat(httpsCache); err2 == nil {
						continue
					}
				}

				googleFaviconUrl := fmt.Sprintf("https://www.google.com/s2/favicons?sz=96&domain_url=%s", url.QueryEscape(domain))
				// ensure https cache
				if _, err := os.Stat(httpsCache); os.IsNotExist(err) {
					gctx, cancel := context.WithTimeout(ctx, 1500*time.Millisecond)
					_ = util.HttpDownload(gctx, googleFaviconUrl, httpsCache)
					cancel()
				}
				// ensure http cache (copy from https if available; otherwise download again)
				if _, err := os.Stat(httpCache); os.IsNotExist(err) {
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

func GetPasteToActiveWindowAction(ctx context.Context, api plugin.API, actionCallback func()) (plugin.QueryResultAction, error) {
	windowName := window.GetActiveWindowName()
	windowIcon, windowIconErr := window.GetActiveWindowIcon()
	if windowIconErr == nil && windowName != "" {
		windowIconImage, err := common.NewWoxImage(windowIcon)
		if err == nil {
			return plugin.QueryResultAction{
				Name:      "Paste to " + windowName,
				Icon:      windowIconImage,
				IsDefault: true,
				Action: func(ctx context.Context, actionContext plugin.ActionContext) {
					if actionCallback != nil {
						actionCallback()
					}
					util.Go(ctx, "ai command paste", func() {
						time.Sleep(time.Millisecond * 150)
						err := keyboard.SimulatePaste()
						if err != nil {
							api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("simulate paste clipboard failed, err=%s", err.Error()))
						} else {
							api.Log(ctx, plugin.LogLevelInfo, "simulate paste clipboard success")
						}
					})
				},
			}, nil
		}
	}

	return plugin.QueryResultAction{}, fmt.Errorf("no active window")
}

// processThinking parse the text to get the thinking and content
func processAIThinking(text string) (thinking string, content string) {
	const thinkStart = "<think>"
	const thinkEnd = "</think>"

	// Trim leading newlines for tag detection
	trimmedText := strings.TrimLeft(text, "\n")

	// Check if the text starts with the thinking tag after trimming newlines
	if len(trimmedText) >= len(thinkStart) && trimmedText[:len(thinkStart)] == thinkStart {
		// Calculate the offset to maintain original indices
		offset := len(text) - len(trimmedText)

		// Find the end tag in the original text
		endIndex := strings.Index(text, thinkEnd)
		if endIndex != -1 {
			// Extract thinking content (without the tags)
			thinking = text[offset+len(thinkStart) : endIndex]
			// Extract the remaining content after the thinking tag
			if endIndex+len(thinkEnd) < len(text) {
				content = text[endIndex+len(thinkEnd):]
			}
		} else {
			// If there's no end tag, the entire text is considered thinking
			thinking = text[offset+len(thinkStart):]
		}
	} else {
		// If there's no thinking tag at the beginning, the entire text is content
		content = text
	}

	return thinking, content
}

func convertAIThinkingToMarkdown(thinking string, content string) string {
	if thinking == "" {
		return content
	}

	// everyline in thinking should be prefixed with "> "
	thinkingLines := strings.Split(thinking, "\n")
	for i, line := range thinkingLines {
		thinkingLines[i] = "> " + line
	}
	thinking = strings.Join(thinkingLines, "\n")

	return thinking + "\n\n" + content
}
