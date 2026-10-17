package websiteicon

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path"

	"github.com/disintegration/imaging"
	"github.com/mat/besticon/besticon"

	"wox/common"
	"wox/common/icons"
	"wox/util"
	"wox/util/imagecache"
)

// FetchEmbedded downloads a direct image or the site favicon and returns bytes that settings can store.
// A cache path is copied into the image so later cache cleanup does not drop the saved icon.
func FetchEmbedded(ctx context.Context, websiteURL string) (common.WoxImage, error) {
	parsed, err := url.Parse(websiteURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Hostname() == "" || parsed.User != nil {
		return common.WoxImage{}, errors.New("invalid URL")
	}
	icon, err := FetchDirectImage(ctx, websiteURL)
	if err == nil {
		return icon, nil
	}
	util.GetLogger().Debug(ctx, "fetch settings icon: direct image failed: "+err.Error())
	if !errors.Is(err, ErrNotAnImage) && IsDirectImageURL(websiteURL) {
		return common.WoxImage{}, err
	}
	icon, err = Fetch(ctx, websiteURL)
	if err != nil {
		return common.WoxImage{}, err
	}
	decoded, err := icon.ToImageWithContext(ctx)
	if err != nil {
		return common.WoxImage{}, err
	}
	return common.NewWoxImage(decoded)
}

// Fetch fetches a favicon or reuses the shared on-disk cache.
func Fetch(ctx context.Context, websiteUrl string) (common.WoxImage, error) {
	parseUrl, err := url.Parse(websiteUrl)
	if err != nil {
		return icons.Get(icons.PluginWebsearch), fmt.Errorf("failed to parse url for %s: %s", websiteUrl, err.Error())
	}
	hostUrl := parseUrl.Scheme + "://" + parseUrl.Host

	// check if existed in cache
	iconPathMd5 := fmt.Sprintf("%x", md5.Sum([]byte(hostUrl)))
	iconCachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%s.png", iconPathMd5))
	if info, statErr := os.Stat(iconCachePath); statErr == nil {
		imagecache.Touch(ctx, iconCachePath, info)
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
	siteIcons, fetchErr := iconFinder.FetchIcons(hostUrl)
	if fetchErr != nil {
		return icons.Get(icons.PluginWebsearch), fmt.Errorf("failed to fetch icons for %s: %s", hostUrl, fetchErr.Error())
	}

	if len(siteIcons) == 0 {
		return icons.Get(icons.PluginWebsearch), fmt.Errorf("no icons found for %s", hostUrl)
	}

	image, imageEr := siteIcons[0].Image()
	if imageEr != nil {
		return icons.Get(icons.PluginWebsearch), fmt.Errorf("failed to get image for %s: %s", hostUrl, imageEr.Error())
	}

	woxImage, woxImageErr := common.NewWoxImage(*image)
	if woxImageErr != nil {
		return icons.Get(icons.PluginWebsearch), fmt.Errorf("failed to convert image for %s: %s", hostUrl, woxImageErr.Error())
	}

	// save to cache
	saveErr := imaging.Save(*image, iconCachePath)
	if saveErr != nil {
		return woxImage, fmt.Errorf("failed to save image for %s: %s", hostUrl, saveErr.Error())
	}

	return woxImage, nil
}
