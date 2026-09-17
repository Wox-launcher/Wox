package websiteicon

import (
	"context"
	"crypto/md5"
	"fmt"
	"github.com/disintegration/imaging"
	"github.com/mat/besticon/besticon"
	"io"
	"net/url"
	"os"
	"path"
	"wox/common"
	"wox/common/icons"
	"wox/util"
	"wox/util/imagecache"
)

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
