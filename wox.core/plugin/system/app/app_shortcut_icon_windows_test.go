package app

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"wox/common"
	"wox/util"

	"github.com/disintegration/imaging"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// TestShortcutArtworkSurvivesLaunchpadAndCache exercises a non-default DLL
// resource through native parsing, list preprocessing, persistence and grid lookup.
func TestShortcutArtworkSurvivesLaunchpadAndCache(t *testing.T) {
	util.GetLogger()
	root := t.TempDir()
	t.Setenv(util.TestWoxDataDirEnv, root)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(root, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	link := filepath.Join(root, "custom.lnk")
	resource := filepath.Join(os.Getenv("SystemRoot"), "System32", "shell32.dll")
	writeAppIconShortcut(t, link, resource+",-4")
	retriever := &WindowsRetriever{}
	info, err := retriever.ParseAppInfo(ctx, link)
	if err != nil {
		t.Fatal(err)
	}
	if info.ResolvedIconPath == "" || info.IconSourcePath != resource || !info.HasDedicatedIcon {
		t.Fatalf("shortcut lost its resolved resource artwork: %+v", info)
	}
	fullSize, err := imaging.Open(info.ResolvedIconPath)
	if err != nil {
		t.Fatal(err)
	}
	if fullSize.Bounds().Dx() <= util.ResultListIconSize {
		t.Fatalf("shortcut lost its full-size artwork: %v", fullSize.Bounds())
	}
	app := &ApplicationPlugin{retriever: retriever, api: emptyAPIImpl{}}
	stat, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}
	app.populateAppMetadata(ctx, link, &info, stat)
	info.Icon = common.ConvertIcon(ctx, info.Icon, "")
	if info.Icon.ImageData == info.ResolvedIconPath {
		t.Fatal("test did not exercise list preprocessing")
	}
	data, err := json.Marshal(appCacheFile{Version: appCacheVersion, Apps: []appInfo{info}})
	if err != nil {
		t.Fatal(err)
	}
	loaded, err := parseAppCacheContent(data)
	if err != nil || len(loaded) != 1 {
		t.Fatalf("reload app cache: entries=%d err=%v", len(loaded), err)
	}
	info = loaded[0]
	populateLaunchpadVisibility(ctx, &info, appDirectory{Path: root})
	if !app.shouldShowInLaunchpad(info) {
		t.Fatal("resolved custom artwork was hidden from Launchpad")
	}
	if got := app.getQueryResultIcon(info, true); got != common.NewWoxImageAbsolutePath(info.ResolvedIconPath) {
		t.Fatalf("Launchpad reopened the DLL or reused the small list icon: %+v", got)
	}
	if got := app.getQueryResultIcon(info, false); got != info.Icon {
		t.Fatalf("list stopped using its preprocessed icon: %+v", got)
	}
	cache := map[string]appInfo{app.pathCacheKey(link): info}
	if _, reused := app.reuseAppFromCache(ctx, link, stat, cache); !reused {
		t.Fatal("fresh shortcut cache was not reused")
	}
	if err := os.Remove(info.ResolvedIconPath); err != nil {
		t.Fatal(err)
	}
	if _, reused := app.reuseAppFromCache(ctx, link, stat, cache); reused {
		t.Fatal("missing full-size artwork did not invalidate the app entry")
	}
	legacy, err := json.Marshal(appCacheFile{Version: 16, Apps: []appInfo{info}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := parseAppCacheContent(legacy); err == nil {
		t.Fatal("app cache from before shortcut icon resolution was accepted")
	}
}

// writeAppIconShortcut creates a real shortcut without launching its target.
func writeAppIconShortcut(t *testing.T, link, location string) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); !ok || oleErr.Code() != oleSFalse {
			t.Fatal(err)
		}
	}
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		t.Fatal(err)
	}
	defer unknown.Release()
	dispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		t.Fatal(err)
	}
	defer dispatch.Release()
	shortcut, err := oleutil.CallMethod(dispatch, "CreateShortcut", link)
	if err != nil {
		t.Fatal(err)
	}
	defer shortcut.Clear()
	for name, value := range map[string]string{
		"TargetPath":   filepath.Join(os.Getenv("SystemRoot"), "System32", "notepad.exe"),
		"IconLocation": location,
	} {
		variant, err := oleutil.PutProperty(shortcut.ToIDispatch(), name, value)
		if variant != nil {
			variant.Clear()
		}
		if err != nil {
			t.Fatal(err)
		}
	}
	saved, err := oleutil.CallMethod(shortcut.ToIDispatch(), "Save")
	if saved != nil {
		saved.Clear()
	}
	if err != nil {
		t.Fatal(err)
	}
}
