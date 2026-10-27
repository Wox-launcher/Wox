package app

import (
	"context"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"wox/common"
	"wox/plugin"
	"wox/util"
)

// TestIndexedIconUsesPluginCache covers warm reuse and recovery after plugin unload.
func TestIndexedIconUsesPluginCache(t *testing.T) {
	util.GetLogger()
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, root)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	sourcePath := filepath.Join(root, "icon.png")
	file, err := os.Create(sourcePath)
	if err != nil {
		t.Fatal(err)
	}
	img := image.NewNRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			img.SetNRGBA(x, y, color.NRGBA{R: 200, A: 255})
		}
	}
	if err := png.Encode(file, img); err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	appPath := filepath.Join(root, "example.exe")
	if err := os.WriteFile(appPath, []byte("app"), 0644); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(appPath)
	if err != nil {
		t.Fatal(err)
	}
	instance := &plugin.Instance{Metadata: plugin.Metadata{Id: common.AppPluginID}}
	a := &ApplicationPlugin{api: emptyAPIImpl{}, retriever: appRetriever, convertIcon: instance.ConvertIcon}
	source := common.NewWoxImageAbsolutePath(sourcePath)
	info := appInfo{Name: "Example", Identity: "example", Path: appPath, Icon: source, LastModifiedUnix: a.getAppModifiedUnix(appPath, stat)}
	a.prepareIndexedIcon(ctx, &info)
	if info.Icon == source || info.IconSource == nil || *info.IconSource != source {
		t.Fatalf("index did not preserve source and prepare list icon: %+v", info)
	}
	pluginCacheRoot := filepath.Join(util.GetLocation().GetImageCacheDirectory(), "plugins", common.AppPluginID)
	if rel, err := filepath.Rel(pluginCacheRoot, info.Icon.ImageData); err != nil || rel == "." || len(rel) < 2 || rel[:2] == ".." {
		t.Fatalf("indexed icon is outside its plugin cache: %s", info.Icon.ImageData)
	}
	if got := instance.ConvertIcon(ctx, info.Icon); got != info.Icon {
		t.Fatalf("query conversion produced a second icon: index=%+v query=%+v", info.Icon, got)
	}
	legacy := info
	legacy.Icon = common.ConvertIcon(ctx, source, "")
	legacy.IconSource = nil
	upgraded, ok := a.reuseAppFromCache(ctx, appPath, stat, map[string]appInfo{a.pathCacheKey(appPath): legacy})
	if !ok || upgraded.IconSource == nil || *upgraded.IconSource != legacy.Icon || upgraded.Icon == legacy.Icon {
		t.Fatalf("legacy cache was not upgraded: %+v, reused=%t", upgraded, ok)
	}
	if got := instance.ConvertIcon(ctx, upgraded.Icon); got != upgraded.Icon {
		t.Fatalf("upgraded icon was converted again: %+v", got)
	}
	a.apps = []appInfo{info}
	a.saveAppToCache(ctx)
	if err := common.ClearPluginImageCache(common.AppPluginID); err != nil {
		t.Fatal(err)
	}
	loaded, err := a.loadAppCache(ctx)
	if err != nil || len(loaded) != 1 || loaded[0].Icon != source {
		t.Fatalf("missing plugin cache did not restore source: %+v, %v", loaded, err)
	}
	restored, ok := a.reuseAppFromCache(ctx, appPath, stat, map[string]appInfo{a.pathCacheKey(appPath): loaded[0]})
	if !ok || restored.Icon == source || restored.Icon == info.Icon {
		t.Fatalf("index did not rebuild the icon in the new generation: %+v, reused=%t", restored, ok)
	}
	if got := instance.ConvertIcon(ctx, restored.Icon); got != restored.Icon {
		t.Fatalf("rebuilt icon was converted again: %+v", got)
	}
}
