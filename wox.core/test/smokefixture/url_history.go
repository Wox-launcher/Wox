package smokefixture

import (
	"crypto/md5"
	"fmt"
	"path/filepath"
)

const (
	URLPluginID                       = "1af58721-6c97-4901-b291-620daf08d9c9"
	MissingFaviconURLHistoryURL       = "https://wox-smoke-missing-favicon.invalid/path"
	MissingFaviconURLHistoryQuery     = "wox-smoke-missing-favicon"
	missingFaviconURLHistoryCacheHost = "https://wox-smoke-missing-favicon.invalid"
)

// MissingFaviconURLHistoryIconPath returns the favicon path used by the URL smoke fixture.
func MissingFaviconURLHistoryIconPath(woxDataDirectory string) string {
	hash := fmt.Sprintf("%x", md5.Sum([]byte(missingFaviconURLHistoryCacheHost)))
	return filepath.Join(woxDataDirectory, "cache", "images", "website_icon_"+hash+".png")
}
