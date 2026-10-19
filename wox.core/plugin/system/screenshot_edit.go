package system

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"wox/common/icons"
	"wox/plugin"
	"wox/util"
	"wox/util/screenshotedit"
)

// saveScreenshotSceneInBackground lets the image reach history and clipboard before encoding the full desktop.
// Plugin unload already waits for these tasks, so a normal shutdown does not abandon an active archive write.
func (p *ScreenshotPlugin) saveScreenshotSceneInBackground(path string, save func() error, jobs *sync.WaitGroup) {
	if save == nil {
		return
	}
	p.runScreenshotStorageTask(jobs, "save screenshot scene", func(ctx context.Context) {
		started := time.Now()
		if err := save(); err != nil {
			util.GetLogger().Warn(ctx, "failed to save screenshot scene: "+err.Error())
			if ctx.Err() == nil {
				p.api.Notify(ctx, "i18n:plugin_screenshot_edit_save_failed")
			}
			return
		}
		util.GetLogger().Debug(ctx, "screenshot scene saved in background: path="+path+" duration="+time.Since(started).String())
		if ctx.Err() == nil {
			// This also updates an already-open clipboard/favorites list once its edit action becomes available.
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		}
	})
}

// indexScreenshotScenes rebuilds clipboard associations without decoding historical desktop images.
func (p *ScreenshotPlugin) indexScreenshotScenes(ctx context.Context) {
	for _, directory := range p.screenshotHistoryDirectories() {
		p.indexScreenshotScenesDirectory(ctx, directory)
	}
}

// indexScreenshotScenesDirectory keeps clipboard edit actions available in both folders during a transfer.
func (p *ScreenshotPlugin) indexScreenshotScenesDirectory(ctx context.Context, directory string) {
	entries, err := os.ReadDir(directory)
	if err != nil {
		if !os.IsNotExist(err) {
			util.GetLogger().Warn(ctx, "failed to index screenshot scenes: "+err.Error())
		}
		return
	}
	for _, entry := range entries {
		if ctx.Err() != nil {
			return
		}
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), screenshotedit.Suffix) {
			continue
		}
		if !isScreenshotStorageImage(directory, strings.TrimSuffix(entry.Name(), screenshotedit.Suffix)) {
			continue
		}
		path := filepath.Join(directory, strings.TrimSuffix(entry.Name(), screenshotedit.Suffix))
		if !screenshotedit.Available(path) {
			continue
		}
		metadata, err := screenshotedit.ReadMetadata(path)
		if err != nil {
			util.GetLogger().Warn(ctx, "failed to read screenshot scene metadata: "+err.Error())
			continue
		}
		screenshotedit.Register(path, metadata)
	}
}

// screenshotEditAction rechecks the scene after query rendering because retention may remove it meanwhile.
func (p *ScreenshotPlugin) screenshotEditAction(path string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Name: "i18n:plugin_screenshot_history_edit", Icon: icons.Get(icons.ActionEdit),
		Action: func(ctx context.Context, _ plugin.ActionContext) {
			if !screenshotedit.Available(path) {
				p.api.Notify(ctx, "i18n:plugin_screenshot_edit_failed")
				return
			}
			p.runScreenshot(ctx, path)
		},
	}
}

// ScreenshotEditActionForImage shares the screenshot plugin's workflow with clipboard history and favorites.
func ScreenshotEditActionForImage(imageHash string) (plugin.QueryResultAction, bool) {
	path := screenshotedit.Resolve(imageHash)
	if path == "" {
		return plugin.QueryResultAction{}, false
	}
	p, ok := plugin.GetPluginManager().GetSystemPlugin(screenshotPluginID).(*ScreenshotPlugin)
	if !ok || p.api == nil {
		return plugin.QueryResultAction{}, false
	}
	return p.screenshotEditAction(path), true
}
