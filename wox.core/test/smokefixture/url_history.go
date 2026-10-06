package smokefixture

import (
	"crypto/md5"
	"fmt"
	"path/filepath"

	"wox/common"
)

const (
	URLPluginID                       = common.URLPluginID
	MissingFaviconURLHistoryURL       = "https://wox-smoke-missing-favicon.invalid/path"
	MissingFaviconURLHistoryQuery     = "wox-smoke-missing-favicon"
	missingFaviconURLHistoryCacheHost = "https://wox-smoke-missing-favicon.invalid"
)

// MissingFaviconURLHistoryIconPath returns the favicon path used by the URL smoke fixture.
func MissingFaviconURLHistoryIconPath(woxDataDirectory string) string {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(missingFaviconURLHistoryCacheHost)))
	return filepath.Join(woxDataDirectory, "cache", "images", "website_icon_"+hash+".png")
}
