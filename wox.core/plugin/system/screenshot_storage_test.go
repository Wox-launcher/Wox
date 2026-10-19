package system

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"wox/common"
	"wox/plugin"
	"wox/util/screenshotedit"
)

type screenshotStorageTestAPI struct {
	plugin.API
	mu       sync.Mutex
	raw      string
	failSave bool
	local    bool
}

func (api *screenshotStorageTestAPI) GetTranslation(_ context.Context, key string) string {
	return key
}

func (api *screenshotStorageTestAPI) GetSetting(_ context.Context, key string) string {
	api.mu.Lock()
	defer api.mu.Unlock()
	if key == screenshotStorageStateSettingKey {
		return api.raw
	}
	return "false"
}

func (api *screenshotStorageTestAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	api.mu.Lock()
	defer api.mu.Unlock()
	if api.failSave {
		return plugin.SetSettingResult{ErrMsg: "disk full"}
	}
	api.raw, api.local = option.Value, option.IsLocal
	return plugin.SetSettingResult{Success: true}
}

func (*screenshotStorageTestAPI) Notify(context.Context, string)                         {}
func (*screenshotStorageTestAPI) RefreshQuery(context.Context, plugin.RefreshQueryParam) {}
func (*screenshotStorageTestAPI) Log(context.Context, plugin.LogLevel, string)           {}

// waitScreenshotStorageTasks fails promptly instead of hanging in the retry loop after a transfer regression.
func waitScreenshotStorageTasks(t *testing.T, p *ScreenshotPlugin) {
	t.Helper()
	done := make(chan struct{})
	go func() { p.backgroundWG.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("screenshot transfer did not finish")
	}
}

// newScreenshotStorageTestPlugin uses isolated folders and a cancellable lifecycle without native capture or OCR.
func newScreenshotStorageTestPlugin(t *testing.T) (*ScreenshotPlugin, *screenshotStorageTestAPI, string, string) {
	t.Helper()
	root := t.TempDir()
	src, dst := filepath.Join(root, "old"), filepath.Join(root, "new")
	for _, directory := range []string{src, dst} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	api := &screenshotStorageTestAPI{}
	p := &ScreenshotPlugin{api: api, storage: screenshotStorageState{Directory: src}, backgroundCtx: ctx, backgroundCancel: cancel}
	t.Cleanup(p.stopScreenshotBackgroundTasks)
	return p, api, src, dst
}

type screenshotDirectoryTestUI struct {
	common.UI
	directory string
}

func (ui *screenshotDirectoryTestUI) CaptureScreenshot(_ context.Context, request common.CaptureScreenshotRequest) (common.CaptureScreenshotResult, error) {
	ui.directory = request.ExportDirectory
	return common.CaptureScreenshotResult{Status: common.CaptureScreenshotStatusCancelled}, nil
}

// TestScreenshotDirectorySwitchWaitsForOldWrites verifies immediate new captures and rejection of overlapping transfers.
func TestScreenshotDirectorySwitchWaitsForOldWrites(t *testing.T) {
	p, api, src, dst := newScreenshotStorageTestPlugin(t)
	image := filepath.Join(src, "20261018_120000_wox_snapshots.png")
	if err := os.WriteFile(image, []byte("image"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, oldJobs := p.beginScreenshotStorageUse()
	release := make(chan struct{})
	var releaseOnce sync.Once
	defer releaseOnce.Do(func() { close(release) })
	p.runScreenshotStorageTask(oldJobs, "finish old scene", func(context.Context) {
		<-release
		if err := os.WriteFile(image+".ocr.json", []byte("ocr"), 0o644); err != nil {
			t.Error(err)
		}
	})
	oldJobs.Done()
	if handled, err := p.HandleSettingUpdate(context.Background(), screenshotSaveDirectorySettingKey, dst); !handled || err != nil {
		t.Fatalf("switch: handled=%t err=%v", handled, err)
	}
	if got, handled := p.GetSettingOverride(context.Background(), screenshotSaveDirectorySettingKey); !handled || got != dst {
		t.Fatalf("displayed directory = %q, handled=%t", got, handled)
	}
	if got := plugin.NewAPI(&plugin.Instance{Plugin: p}).GetSetting(context.Background(), screenshotSaveDirectorySettingKey); got != dst {
		t.Fatalf("settings API displayed %q instead of the effective directory", got)
	}
	ui := &screenshotDirectoryTestUI{}
	p.runScreenshotRequest(context.Background(), "", common.DefaultCaptureScreenshotRequest(), ui)
	if ui.directory != dst {
		t.Fatalf("new capture directory = %q, want %q", ui.directory, dst)
	}
	if _, err := p.HandleSettingUpdate(context.Background(), screenshotSaveDirectorySettingKey, filepath.Join(t.TempDir(), "third")); err == nil || !strings.Contains(err.Error(), "migration_busy") {
		t.Fatalf("expected pending-transfer error, got %v", err)
	}
	if _, err := os.Stat(image); err != nil {
		t.Fatalf("source moved before old writes finished: %v", err)
	}
	items, err := p.listScreenshotHistory()
	if err != nil || len(items) != 1 || items[0].path != image {
		t.Fatalf("history disappeared while transfer waited: items=%v err=%v", items, err)
	}
	releaseOnce.Do(func() { close(release) })
	waitScreenshotStorageTasks(t, p)
	if _, err := p.HandleSettingUpdate(context.Background(), screenshotSaveDirectorySettingKey, dst); err != nil {
		t.Fatalf("completed transfer still blocks directory settings: %v", err)
	}
	for _, suffix := range []string{"", ".ocr.json"} {
		if _, err := os.Stat(filepath.Join(dst, "20261018_120000_wox_snapshots.png"+suffix)); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(image + suffix); !os.IsNotExist(err) {
			t.Fatalf("old file still present: %v", err)
		}
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	var saved screenshotStorageState
	if err := json.Unmarshal([]byte(api.raw), &saved); err != nil {
		t.Fatal(err)
	}
	if !api.local || saved.Directory != dst || saved.Source != "" || saved.Current != nil {
		t.Fatalf("incorrect durable state: local=%t state=%+v", api.local, saved)
	}
}

// TestScreenshotTransferCollisionPreservesBundle checks collision renaming and mtime-sensitive OCR compatibility.
func TestScreenshotTransferCollisionPreservesBundle(t *testing.T) {
	p, _, src, dst := newScreenshotStorageTestPlugin(t)
	modified := time.Now().Add(-time.Hour).Truncate(time.Millisecond)
	for _, suffix := range []string{"", ".ocr.json", screenshotedit.Suffix} {
		path := filepath.Join(src, "20261018_120000_wox_snapshots.png"+suffix)
		if err := os.WriteFile(path, []byte("old"+suffix), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dst, "20261018_120000_wox_snapshots.png"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	p.storage = screenshotStorageState{Directory: dst, Source: src}
	if err := p.transferScreenshotHistory(context.Background()); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dst)
	if err != nil {
		t.Fatal(err)
	}
	var renamed string
	for _, entry := range entries {
		if isScreenshotHistoryImage(entry.Name()) && entry.Name() != "20261018_120000_wox_snapshots.png" {
			renamed = entry.Name()
		}
	}
	if renamed == "" {
		t.Fatal("colliding capture was not preserved")
	}
	for _, suffix := range []string{"", ".ocr.json", screenshotedit.Suffix} {
		path := filepath.Join(dst, renamed+suffix)
		data, err := os.ReadFile(path)
		if err != nil || string(data) != "old"+suffix {
			t.Fatalf("companion mismatch: %s data=%q err=%v", path, data, err)
		}
		info, err := os.Stat(path)
		if err != nil || !info.ModTime().Equal(modified) {
			t.Fatalf("mtime changed: %s err=%v", path, err)
		}
	}
	data, _ := os.ReadFile(filepath.Join(dst, "20261018_120000_wox_snapshots.png"))
	if string(data) != "existing" {
		t.Fatal("destination capture was overwritten")
	}
}

// TestScreenshotTransferResumesPublishedCompanions models a restart after copying a companion but before deleting originals.
func TestScreenshotTransferResumesPublishedCompanions(t *testing.T) {
	p, api, src, dst := newScreenshotStorageTestPlugin(t)
	state := screenshotStorageState{Directory: dst, Source: src, Current: &screenshotStorageTransfer{Name: "20261018_120000_wox_snapshots.png", Target: "20261018_120000_wox_snapshots_renamed.png"}}
	for _, suffix := range []string{"", ".ocr.json"} {
		if err := os.WriteFile(filepath.Join(src, "20261018_120000_wox_snapshots.png"+suffix), []byte("saved"+suffix), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := copyScreenshotStorageFile(context.Background(), filepath.Join(src, "20261018_120000_wox_snapshots.png.ocr.json"), filepath.Join(dst, "20261018_120000_wox_snapshots_renamed.png.ocr.json")); err != nil {
		t.Fatal(err)
	}
	if err := p.persistScreenshotStorage(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	p.initScreenshotStorage(context.Background())
	waitScreenshotStorageTasks(t, p)
	data, err := os.ReadFile(filepath.Join(dst, "20261018_120000_wox_snapshots_renamed.png"))
	if err != nil || string(data) != "saved" {
		t.Fatalf("resume failed: data=%q err=%v", data, err)
	}
	if entries, err := os.ReadDir(src); err != nil || len(entries) != 0 {
		t.Fatalf("resume left originals: entries=%v err=%v", entries, err)
	}
	api.mu.Lock()
	defer api.mu.Unlock()
	if strings.Contains(api.raw, "source") || strings.Contains(api.raw, "current") {
		t.Fatalf("resume did not clear pending state: %s", api.raw)
	}
}

// TestScreenshotDirectoryRejectsInvalidAndUnpersistedChanges keeps validation failures from switching runtime storage.
func TestScreenshotDirectoryRejectsInvalidAndUnpersistedChanges(t *testing.T) {
	p, api, src, dst := newScreenshotStorageTestPlugin(t)
	if err := os.WriteFile(filepath.Join(src, "not-a-folder"), []byte("file"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"", "relative", filepath.Join(src, "not-a-folder")} {
		if _, err := p.HandleSettingUpdate(context.Background(), screenshotSaveDirectorySettingKey, value); err == nil {
			t.Fatalf("accepted invalid directory %q", value)
		}
	}
	api.failSave = true
	if _, err := p.HandleSettingUpdate(context.Background(), screenshotSaveDirectorySettingKey, dst); err == nil {
		t.Fatal("accepted a directory whose state could not be persisted")
	}
	if p.getScreenshotDirectory() != src || p.storage.Source != "" {
		t.Fatal("failed setting changed the runtime directory")
	}
}

// TestScreenshotStorageCopyCancellationAndConflict never deletes or replaces a source when publication fails.
func TestScreenshotStorageCopyCancellationAndConflict(t *testing.T) {
	_, _, src, dst := newScreenshotStorageTestPlugin(t)
	left, right := filepath.Join(src, "20261018_120000_wox_snapshots.png"), filepath.Join(dst, "20261018_120000_wox_snapshots.png")
	if err := os.WriteFile(left, []byte("source"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyScreenshotStorageFile(ctx, left, right); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if _, err := os.Stat(right); !os.IsNotExist(err) {
		t.Fatalf("cancelled copy published a destination: %v", err)
	}
	if err := os.WriteFile(right, []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := copyScreenshotStorageFile(context.Background(), left, right); err == nil {
		t.Fatal("copy accepted a different destination file")
	}
	if err := publishScreenshotStorageFile(left, right); err == nil {
		t.Fatal("atomic publication replaced a file created by another writer")
	}
	if err := copyScreenshotStorageFile(context.Background(), left, left); err == nil {
		t.Fatal("copy accepted a destination that aliases its source")
	}
	data, _ := os.ReadFile(right)
	if string(data) != "existing" {
		t.Fatal("copy replaced an existing file")
	}
	if _, err := os.Stat(left); err != nil {
		t.Fatal("copy removed its source")
	}
}

// TestScreenshotCustomFolderLeavesUnrelatedImagesAlone protects other images in shared cloud folders.
func TestScreenshotCustomFolderLeavesUnrelatedImagesAlone(t *testing.T) {
	p, _, src, dst := newScreenshotStorageTestPlugin(t)
	owned, unrelated := "20261018_120000_wox_snapshots.png", "family.png"
	for _, name := range []string{owned, unrelated} {
		if err := os.WriteFile(filepath.Join(src, name), []byte(name), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	items, err := p.listScreenshotHistory()
	if err != nil || len(items) != 1 || items[0].fileName != owned {
		t.Fatalf("custom history included unrelated images: items=%v err=%v", items, err)
	}
	p.storage = screenshotStorageState{Directory: dst, Source: src}
	if err := p.transferScreenshotHistory(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(src, unrelated)); err != nil {
		t.Fatal("migration moved an unrelated image")
	}
	// Retention must use the same ownership rule even for images older than its cutoff.
	modified := time.Now().AddDate(0, 0, -40)
	for _, name := range []string{owned, unrelated} {
		path := filepath.Join(dst, name)
		if name == unrelated {
			if err := os.WriteFile(path, []byte(name), 0o644); err != nil {
				t.Fatal(err)
			}
		}
		if err := os.Chtimes(path, modified, modified); err != nil {
			t.Fatal(err)
		}
	}
	p.cleanupExpiredScreenshots(context.Background())
	if _, err := os.Stat(filepath.Join(dst, unrelated)); err != nil {
		t.Fatal("retention removed an unrelated image")
	}
	if _, err := os.Stat(filepath.Join(dst, owned)); !os.IsNotExist(err) {
		t.Fatalf("retention did not remove an expired Wox screenshot: %v", err)
	}
}
