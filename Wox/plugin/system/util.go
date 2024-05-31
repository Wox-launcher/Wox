package system

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/mat/besticon/besticon"
	"io"
	"net/url"
	"os"
	"path"
	"wox/plugin"
	"wox/plugin/llm"
	"wox/setting"
	"wox/util"
)

type cacheResult struct {
	match bool
	score int64
}

var pinyinMatchCache = util.NewHashMap[string, cacheResult]()

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
	chatStream func(ctx context.Context, conversations []llm.Conversation) (llm.ChatStream, error),
	conversations []llm.Conversation,
	shouldStartAnswering func() bool,
	onAnswering func(plugin.RefreshableResult, string) plugin.RefreshableResult,
	onAnswerErr func(plugin.RefreshableResult, error) plugin.RefreshableResult,
	onAnswerFinished func(plugin.RefreshableResult) plugin.RefreshableResult) func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {

	var stream llm.ChatStream
	var creatingStream bool
	return func(ctx context.Context, current plugin.RefreshableResult) plugin.RefreshableResult {
		if !shouldStartAnswering() {
			return current
		}

		if stream == nil {
			if creatingStream {
				util.GetLogger().Info(ctx, "Already creating stream, waiting create finish")
				return current
			}

			startTime := util.GetSystemTimestamp()
			util.GetLogger().Info(ctx, "creating stream")
			creatingStream = true
			createdStream, createErr := chatStream(ctx, conversations)
			creatingStream = false
			util.GetLogger().Info(ctx, fmt.Sprintf("created stream (cost %d ms)", util.GetSystemTimestamp()-startTime))
			if createErr != nil {
				if onAnswerErr != nil {
					current = onAnswerErr(current, createErr)
				}
				current.RefreshInterval = 0 // stop refreshing
				return current
			}
			stream = createdStream
		}

		util.GetLogger().Info(ctx, fmt.Sprintf("reading stream"))
		response, streamErr := stream.Receive(ctx)
		if errors.Is(streamErr, io.EOF) {
			util.GetLogger().Info(ctx, "read stream completed")
			if onAnswerFinished != nil {
				current = onAnswerFinished(current)
			}
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		if streamErr != nil {
			util.GetLogger().Info(ctx, fmt.Sprintf("failed to read stream: %s", streamErr.Error()))
			if onAnswerErr != nil {
				current = onAnswerErr(current, streamErr)
			}
			current.RefreshInterval = 0 // stop refreshing
			return current
		}

		if onAnswering != nil {
			util.GetLogger().Info(ctx, fmt.Sprintf("streamed %d text", len(response)))
			current = onAnswering(current, response)
		}

		return current
	}
}
