package system

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/mat/besticon/besticon"
	"image/png"
	"io"
	"net/url"
	"os"
	"path"
	"sync"
	"wox/ai"
	"wox/plugin"
	"wox/setting"
	"wox/share"
	"wox/util"
	"wox/util/window"
)

type cacheResult struct {
	match bool
	score int64
}

var pinyinMatchCache = util.NewHashMap[string, cacheResult]()
var windowIconCache = util.NewHashMap[string, plugin.WoxImage]()

func IsStringMatchScore(ctx context.Context, term string, subTerm string) (bool, int64) {
	woxSetting := setting.GetSettingManager().GetWoxSetting(ctx)
	if woxSetting.UsePinYin {
		key := term + subTerm
		if result, ok := pinyinMatchCache.Load(key); ok {
			return result.match, result.score
		}
	}

	match, score := util.IsStringMatchScore(term, subTerm, woxSetting.UsePinYin)
	if woxSetting.UsePinYin {
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

func getWebsiteIconWithCache(ctx context.Context, websiteUrl string) (plugin.WoxImage, error) {
	parseUrl, err := url.Parse(websiteUrl)
	if err != nil {
		return webSearchIcon, fmt.Errorf("failed to parse url for %s: %s", websiteUrl, err.Error())
	}
	hostUrl := parseUrl.Scheme + "://" + parseUrl.Host

	// check if existed in cache
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(hostUrl)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", iconPathMd5))
	if _, statErr := os.Stat(iconCachePath); statErr == nil {
		return plugin.WoxImage{
			ImageType: plugin.WoxImageTypeAbsolutePath,
			ImageData: iconCachePath,
		}, nil
	}

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

	woxImage, woxImageErr := plugin.NewWoxImage(*image)
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

func createLLMOnRefreshHandler(ctx context.Context,
	chatStreamAPI func(ctx context.Context, model ai.Model, conversations []ai.Conversation, callback ai.ChatStreamFunc) error,
	model ai.Model,
	conversations []ai.Conversation,
	shouldStartAnswering func() bool,
	onPreparing func(plugin.RefreshableResult) plugin.RefreshableResult,
	onAnswering func(plugin.RefreshableResult, string, bool) plugin.RefreshableResult,
	onAnswerErr func(plugin.RefreshableResult, error) plugin.RefreshableResult) func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {

	var isStreamCreated bool
	var locker sync.Locker = &sync.Mutex{}
	var chatStreamDataTypeBuffer ai.ChatStreamDataType
	var responseBuffer string
	return func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
		if !shouldStartAnswering() {
			return current
		}

		if !isStreamCreated {
			isStreamCreated = true
			util.GetLogger().Info(ctx, "creating stream")
			if onPreparing != nil {
				current = onPreparing(current)
			}
			err := chatStreamAPI(ctx, model, conversations, func(chatStreamDataType ai.ChatStreamDataType, response string) {
				locker.Lock()
				chatStreamDataTypeBuffer = chatStreamDataType
				if chatStreamDataType == ai.ChatStreamTypeStreaming || chatStreamDataTypeBuffer == ai.ChatStreamTypeFinished {
					responseBuffer += response
				}
				if chatStreamDataType == ai.ChatStreamTypeError {
					responseBuffer = response
				}
				util.GetLogger().Info(ctx, fmt.Sprintf("stream buffered: %s", responseBuffer))
				locker.Unlock()
			})
			if err != nil {
				util.GetLogger().Info(ctx, fmt.Sprintf("failed to create stream: %s", err.Error()))
				return onAnswerErr(current, err)
			}
		}

		if chatStreamDataTypeBuffer == ai.ChatStreamTypeFinished {
			util.GetLogger().Info(ctx, "stream finished")
			locker.Lock()
			buf := responseBuffer
			responseBuffer = ""
			locker.Unlock()
			return onAnswering(current, buf, true)
		}
		if chatStreamDataTypeBuffer == ai.ChatStreamTypeError {
			util.GetLogger().Info(ctx, fmt.Sprintf("stream error: %s", responseBuffer))
			locker.Lock()
			err := fmt.Errorf(responseBuffer)
			responseBuffer = ""
			locker.Unlock()
			return onAnswerErr(current, err)
		}
		if chatStreamDataTypeBuffer == ai.ChatStreamTypeStreaming {
			util.GetLogger().Info(ctx, fmt.Sprintf("streaming: %s", responseBuffer))
			locker.Lock()
			buf := responseBuffer
			responseBuffer = ""
			locker.Unlock()
			return onAnswering(current, buf, false)
		}

		return current
	}
}

func getActiveWindowIcon(ctx context.Context) (plugin.WoxImage, error) {
	cacheKey := fmt.Sprintf("%s-%d", window.GetActiveWindowName(), window.GetActiveWindowPid())
	if icon, ok := windowIconCache.Load(cacheKey); ok {
		return icon, nil
	}

	icon, err := window.GetActiveWindowIcon()
	if err != nil {
		return plugin.WoxImage{}, err
	}

	var buf bytes.Buffer
	encodeErr := png.Encode(&buf, icon)
	if encodeErr != nil {
		return plugin.WoxImage{}, encodeErr
	}

	base64Str := base64.StdEncoding.EncodeToString(buf.Bytes())
	woxIcon := plugin.NewWoxImageBase64(fmt.Sprintf("data:image/png;base64,%s", base64Str))
	windowIconCache.Store(cacheKey, woxIcon)
	return woxIcon, nil
}

func refreshQuery(ctx context.Context, api plugin.API, query plugin.Query) {
	if query.Type == plugin.QueryTypeSelection {
		return
	}

	api.ChangeQuery(ctx, share.PlainQuery{
		QueryType: query.Type,
		QueryText: query.RawQuery,
	})
}
