package system

import (
	"context"
	"crypto/md5"
	"fmt"
	"net/url"
	"os"
	"path"
	"testing"
	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
	"wox/util/clipboard"
)

func TestResolveTextRecordIconPrefersCachedFaviconForLinks(t *testing.T) {
	initClipboardFaviconCache(t)
	link := "https://github.com/Wox-launcher/Wox"
	cachePath := writeClipboardWebsiteIconCache(t, link)
	sourceIcon := common.NewWoxImageAbsolutePath("/tmp/chrome.png")
	sourceIconData := sourceIcon.String()
	record := ClipboardRecord{
		Type:     string(clipboard.ClipboardTypeText),
		Content:  link,
		IconData: &sourceIconData,
	}

	clipboardPlugin := &ClipboardPlugin{api: &imagePasteFailureAPI{}}
	if icon := clipboardPlugin.resolveTextRecordIcon(context.Background(), record, link); icon.ImageData != cachePath {
		t.Fatalf("link icon = %+v, want cached favicon %q", icon, cachePath)
	}

	plain := ClipboardRecord{Type: string(clipboard.ClipboardTypeText), Content: "hello"}
	if icon := clipboardPlugin.resolveTextRecordIcon(context.Background(), plain, ""); icon != icons.Get(icons.ActionText) {
		t.Fatalf("plain text icon = %+v, want default text icon", icon)
	}

	uncached := ClipboardRecord{
		Type:     string(clipboard.ClipboardTypeText),
		Content:  "https://example.com/path",
		IconData: &sourceIconData,
	}
	if icon := clipboardPlugin.resolveTextRecordIcon(context.Background(), uncached, "https://example.com/path"); icon.ImageData != sourceIcon.ImageData {
		t.Fatalf("uncached link icon = %+v, want source app icon", icon)
	}
}

func TestConvertTextRecordUsesCachedFavicon(t *testing.T) {
	initClipboardFaviconCache(t)
	link := "https://github.com/Wox-launcher/Wox"
	cachePath := writeClipboardWebsiteIconCache(t, link)
	clipboardPlugin := &ClipboardPlugin{api: &imagePasteFailureAPI{}, imageCache: util.NewHashMap[string, *ImageCacheEntry]()}

	result := clipboardPlugin.convertTextRecord(context.Background(), ClipboardRecord{
		ID:      "link-1",
		Type:    string(clipboard.ClipboardTypeText),
		Content: link,
	}, plugin.Query{})
	if result.Icon.ImageData != cachePath {
		t.Fatalf("result icon = %+v, want cached favicon %q", result.Icon, cachePath)
	}
}

func TestCollectMissingLinkFaviconURLsIsOncePerHostAndSkipsCache(t *testing.T) {
	initClipboardFaviconCache(t)
	cached := "https://github.com/Wox-launcher/Wox"
	writeClipboardWebsiteIconCache(t, cached)
	missing := "https://example.com/path"
	clipboardPlugin := &ClipboardPlugin{faviconFetchAttempted: util.NewHashMap[string, bool]()}

	records := []ClipboardRecord{
		{Type: string(clipboard.ClipboardTypeText), Content: cached},
		{Type: string(clipboard.ClipboardTypeText), Content: missing},
		{Type: string(clipboard.ClipboardTypeText), Content: "https://example.com/other"},
		{Type: string(clipboard.ClipboardTypeText), Content: "plain text"},
	}
	got := clipboardPlugin.collectMissingLinkFaviconURLs(context.Background(), records)
	if len(got) != 1 || got[0] != missing {
		t.Fatalf("first collect = %v, want [%q]", got, missing)
	}
	if again := clipboardPlugin.collectMissingLinkFaviconURLs(context.Background(), records); len(again) != 0 {
		t.Fatalf("second collect = %v, want none after the session attempt", again)
	}
}

func initClipboardFaviconCache(t *testing.T) {
	t.Helper()
	t.Setenv(util.TestWoxDataDirEnv, t.TempDir())
	t.Setenv(util.TestUserDataDirEnv, t.TempDir())
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
}

func writeClipboardWebsiteIconCache(t *testing.T, websiteURL string) string {
	t.Helper()
	parsed, err := url.Parse(websiteURL)
	if err != nil {
		t.Fatalf("parse url: %v", err)
	}
	hostURL := parsed.Scheme + "://" + parsed.Host
	cachePath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("website_icon_%x.png", md5.Sum([]byte(hostURL))))
	if err := os.MkdirAll(path.Dir(cachePath), 0o755); err != nil {
		t.Fatalf("create cache dir: %v", err)
	}
	if err := os.WriteFile(cachePath, []byte("favicon"), 0o644); err != nil {
		t.Fatalf("write cache: %v", err)
	}
	return cachePath
}
