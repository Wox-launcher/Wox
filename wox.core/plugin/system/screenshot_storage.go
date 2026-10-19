package system

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
	"wox/plugin"
	"wox/setting/definition"
	"wox/util"
	"wox/util/screenshotedit"

	"github.com/google/uuid"
	"github.com/mitchellh/go-homedir"
)

const screenshotSaveDirectorySettingKey = "save_directory"
const screenshotStorageStateSettingKey = "storage_state"

var screenshotOwnedFileName = regexp.MustCompile(`(?i)^\d{8}_\d{6}_wox_snapshots(?:_[a-z0-9_-]+)?\.(png|jpe?g)$`)

// The destination and pending source are one local setting: a crash cannot save the
// new directory without also recording where the unfinished transfer must resume.
type screenshotStorageState struct {
	Directory string                     `json:"directory"`
	Source    string                     `json:"source,omitempty"`
	Current   *screenshotStorageTransfer `json:"current,omitempty"`
}

// Current persists collision renames before any file moves, keeping companions
// attached to the same image when a transfer resumes after an interrupted copy.
type screenshotStorageTransfer struct {
	Name   string `json:"name"`
	Target string `json:"target"`
}

// initScreenshotStorage restores runtime state without rewriting older settings.
func (p *ScreenshotPlugin) initScreenshotStorage(ctx context.Context) {
	var state screenshotStorageState
	if raw := p.api.GetSetting(ctx, screenshotStorageStateSettingKey); raw != "" {
		if err := json.Unmarshal([]byte(raw), &state); err != nil {
			util.GetLogger().Error(ctx, "failed to read screenshot storage state: "+err.Error())
			return
		}
	}
	p.storageM.Lock()
	p.storage = state
	p.storageJobs = &sync.WaitGroup{}
	p.storageM.Unlock()
	if state.Source != "" {
		p.startScreenshotTransfer(nil)
	}
}

func (p *ScreenshotPlugin) screenshotDirectoryLocked() string {
	if p.storage.Directory != "" {
		return p.storage.Directory
	}
	return util.GetLocation().GetScreenshotDirectory()
}

// screenshotHistoryDirectories includes pending captures until the durable transfer is complete.
func (p *ScreenshotPlugin) screenshotHistoryDirectories() []string {
	p.storageM.Lock()
	defer p.storageM.Unlock()
	directories := []string{p.screenshotDirectoryLocked()}
	if p.storage.Source != "" {
		directories = append(directories, p.storage.Source)
	}
	return directories
}

// isScreenshotStorageImage preserves legacy history in the private default folder,
// while shared custom folders must never enroll unrelated images in retention or relocation.
func isScreenshotStorageImage(directory, name string) bool {
	if screenshotStoragePathsEqual(directory, util.GetLocation().GetScreenshotDirectory()) {
		return isScreenshotHistoryImage(name)
	}
	return screenshotOwnedFileName.MatchString(name)
}

// screenshotDirectorySetting exposes the effective path without syncing a machine-specific directory.
func (p *ScreenshotPlugin) screenshotDirectorySetting() definition.PluginSettingDefinitionItem {
	return definition.PluginSettingDefinitionItem{
		Type: definition.PluginSettingDefinitionTypeDirPath,
		Value: &definition.PluginSettingValueDirPath{
			Key: screenshotSaveDirectorySettingKey, DefaultValue: p.getScreenshotDirectory(),
			Label: "i18n:plugin_screenshot_save_directory", Tooltip: "i18n:plugin_screenshot_save_directory_tooltip",
		},
	}
}

// GetSettingOverride displays the directory from the same atomic state used by captures and transfers.
func (p *ScreenshotPlugin) GetSettingOverride(_ context.Context, key string) (string, bool) {
	if key != screenshotSaveDirectorySettingKey {
		return "", false
	}
	return p.getScreenshotDirectory(), true
}

// HandleSettingUpdate is an internal settings-service hook; switching storage must
// validate and persist its transfer before the ordinary form reports success.
func (p *ScreenshotPlugin) HandleSettingUpdate(ctx context.Context, key, value string) (bool, error) {
	if key != screenshotSaveDirectorySettingKey {
		return false, nil
	}
	p.storageM.Lock()
	defer p.storageM.Unlock()
	if p.storage.Source != "" {
		return true, errors.New(p.api.GetTranslation(ctx, "plugin_screenshot_directory_migration_busy"))
	}
	oldDirectory := p.screenshotDirectoryLocked()
	directory, err := homedir.Expand(strings.TrimSpace(value))
	if err != nil || directory == "" || !filepath.IsAbs(directory) {
		return true, errors.New(p.api.GetTranslation(ctx, "plugin_screenshot_directory_invalid"))
	}
	directory = filepath.Clean(directory)
	if screenshotStoragePathsEqual(oldDirectory, directory) {
		return true, nil
	}
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return true, fmt.Errorf("%s: %w", p.api.GetTranslation(ctx, "plugin_screenshot_directory_invalid"), err)
	}
	// Resolve aliases before comparing paths, including Windows junctions and symlinked cloud folders.
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return true, fmt.Errorf("%s: %w", p.api.GetTranslation(ctx, "plugin_screenshot_directory_invalid"), err)
	}
	resolvedOld, err := filepath.EvalSymlinks(oldDirectory)
	if os.IsNotExist(err) {
		resolvedOld = oldDirectory
	} else if err != nil {
		return true, err
	}
	if screenshotStoragePathsEqual(resolvedOld, resolvedDirectory) {
		return true, nil
	}
	probe, err := os.CreateTemp(directory, ".wox-screenshot-write-test-*")
	if err != nil {
		return true, fmt.Errorf("%s: %w", p.api.GetTranslation(ctx, "plugin_screenshot_directory_invalid"), err)
	}
	probe.Close()
	os.Remove(probe.Name())
	state := screenshotStorageState{Directory: directory, Source: oldDirectory}
	if err := p.persistScreenshotStorage(ctx, state); err != nil {
		return true, err
	}
	oldJobs := p.storageJobs
	p.storage = state
	p.storageJobs = &sync.WaitGroup{}
	p.startScreenshotTransfer(oldJobs)
	return true, nil
}

// persistScreenshotStorage keeps local filesystem paths and transfer progress out of Cloud Sync.
func (p *ScreenshotPlugin) persistScreenshotStorage(ctx context.Context, state screenshotStorageState) error {
	raw, err := json.Marshal(state)
	if err != nil {
		return err
	}
	result := p.api.SetSetting(ctx, plugin.SetSettingOption{Key: screenshotStorageStateSettingKey, Value: string(raw), IsLocal: true})
	if !result.Success {
		return errors.New(result.ErrMsg)
	}
	return nil
}

// beginScreenshotStorageUse leases the current generation before a capture or cleanup can start writing.
func (p *ScreenshotPlugin) beginScreenshotStorageUse() (string, *sync.WaitGroup) {
	p.storageM.Lock()
	defer p.storageM.Unlock()
	return p.beginScreenshotStorageUseLocked()
}

// beginScreenshotStorageUseLocked registers the lease before a directory switch can detach its generation.
func (p *ScreenshotPlugin) beginScreenshotStorageUseLocked() (string, *sync.WaitGroup) {
	if p.storageJobs == nil {
		p.storageJobs = &sync.WaitGroup{}
	}
	p.storageJobs.Add(1)
	return p.screenshotDirectoryLocked(), p.storageJobs
}

// runScreenshotStorageTask extends a capture's lease through its deferred scene/OCR writes.
func (p *ScreenshotPlugin) runScreenshotStorageTask(jobs *sync.WaitGroup, message string, job func(context.Context)) {
	if jobs == nil {
		p.runScreenshotBackgroundTask(message, job)
		return
	}
	jobs.Add(1)
	if !p.runScreenshotBackgroundTask(message, func(ctx context.Context) {
		defer jobs.Done()
		job(ctx)
	}) {
		jobs.Done()
	}
}

// startScreenshotTransfer waits only for the previous generation; new captures use
// the new directory immediately and never wait for historical files to be copied.
func (p *ScreenshotPlugin) startScreenshotTransfer(oldJobs *sync.WaitGroup) {
	p.runScreenshotBackgroundTask("move screenshot history", func(ctx context.Context) {
		if oldJobs != nil {
			finished := make(chan struct{})
			go func() { oldJobs.Wait(); close(finished) }()
			select {
			case <-finished:
			case <-ctx.Done():
				return
			}
		}
		reported := false
		for ctx.Err() == nil {
			if err := p.transferScreenshotHistory(ctx); err == nil {
				p.indexScreenshotScenes(ctx)
				p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
				p.api.Notify(ctx, "i18n:plugin_screenshot_directory_migration_complete")
				return
			} else if ctx.Err() == nil {
				util.GetLogger().Warn(ctx, "failed to move screenshot history; will retry: "+err.Error())
				if !reported {
					p.api.Notify(ctx, "i18n:plugin_screenshot_directory_migration_failed")
					reported = true
				}
			}
			timer := time.NewTimer(30 * time.Second)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return
			}
		}
	})
}

// transferScreenshotHistory checkpoints each image bundle and leaves unrelated files in a user-selected folder alone.
func (p *ScreenshotPlugin) transferScreenshotHistory(ctx context.Context) error {
	p.storageM.Lock()
	state := p.storage
	p.storageM.Unlock()
	if state.Source == "" {
		return nil
	}
	if state.Current != nil {
		if err := transferScreenshotBundle(ctx, state); err != nil {
			return err
		}
		state.Current = nil
		if err := p.checkpointScreenshotStorage(ctx, state); err != nil {
			return err
		}
	}
	entries, err := os.ReadDir(state.Source)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	lastRefresh := time.Now()
	for _, entry := range entries {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if !entry.Type().IsRegular() || !isScreenshotStorageImage(state.Source, entry.Name()) {
			continue
		}
		target := entry.Name()
		// Legacy names moved out of the private default directory need the ownership
		// marker too, otherwise shared-folder retention could not distinguish them.
		if !screenshotOwnedFileName.MatchString(target) {
			info, err := entry.Info()
			if err != nil {
				return err
			}
			target = info.ModTime().Format("20060102_150405") + "_wox_snapshots_" + uuid.NewString() + filepath.Ext(target)
		}
		for _, suffix := range []string{"", ".ocr.json", screenshotedit.Suffix} {
			if _, err := os.Lstat(filepath.Join(state.Directory, target+suffix)); !os.IsNotExist(err) {
				ext := filepath.Ext(target)
				target = strings.TrimSuffix(target, ext) + "_" + uuid.NewString() + ext
				break
			}
		}
		state.Current = &screenshotStorageTransfer{Name: entry.Name(), Target: target}
		if err := p.checkpointScreenshotStorage(ctx, state); err != nil {
			return err
		}
		if err := transferScreenshotBundle(ctx, state); err != nil {
			return err
		}
		state.Current = nil
		if err := p.checkpointScreenshotStorage(ctx, state); err != nil {
			return err
		}
		if time.Since(lastRefresh) >= time.Second {
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			lastRefresh = time.Now()
		}
	}
	state.Source = ""
	return p.checkpointScreenshotStorage(ctx, state)
}

// checkpointScreenshotStorage publishes progress only after its durable local write succeeds.
func (p *ScreenshotPlugin) checkpointScreenshotStorage(ctx context.Context, state screenshotStorageState) error {
	p.storageM.Lock()
	defer p.storageM.Unlock()
	if err := p.persistScreenshotStorage(ctx, state); err != nil {
		return err
	}
	p.storage = state
	return nil
}

// transferScreenshotBundle copies companions first and removes originals only after every copy succeeds.
func transferScreenshotBundle(ctx context.Context, state screenshotStorageState) error {
	suffixes := []string{".ocr.json", screenshotedit.Suffix, ""}
	for _, suffix := range suffixes {
		if err := copyScreenshotStorageFile(ctx, filepath.Join(state.Source, state.Current.Name+suffix), filepath.Join(state.Directory, state.Current.Target+suffix)); err != nil {
			return err
		}
	}
	for _, suffix := range suffixes {
		var err error
		if suffix == screenshotedit.Suffix {
			err = screenshotedit.Remove(filepath.Join(state.Source, state.Current.Name))
		} else {
			err = os.Remove(filepath.Join(state.Source, state.Current.Name+suffix))
		}
		if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	// Clipboard edit actions resolve by fingerprint, so publish each relocated scene before the next bundle.
	imagePath := filepath.Join(state.Directory, state.Current.Target)
	if screenshotedit.Available(imagePath) {
		if metadata, err := screenshotedit.ReadMetadata(imagePath); err == nil {
			screenshotedit.Register(imagePath, metadata)
		}
	}
	return nil
}

// copyScreenshotStorageFile supports cross-volume/cloud folders, cancellation,
// and resume without replacing an existing file. Mtime preserves history and OCR validity.
func copyScreenshotStorageFile(ctx context.Context, src, dst string) error {
	info, err := os.Lstat(src)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("screenshot is not a regular file: %s", src)
	}
	if destinationInfo, err := os.Lstat(dst); err == nil {
		if os.SameFile(info, destinationInfo) {
			return fmt.Errorf("screenshot destination aliases its source: %s", dst)
		}
		same, err := screenshotStorageFilesEqual(ctx, src, dst)
		if err != nil {
			return err
		}
		if !same {
			return fmt.Errorf("screenshot destination already exists: %s", dst)
		}
		return os.Chtimes(dst, info.ModTime(), info.ModTime())
	} else if !os.IsNotExist(err) {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	temp, err := os.CreateTemp(filepath.Dir(dst), ".wox-screenshot-transfer-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())
	err = copyScreenshotStorageData(ctx, temp, in)
	if err == nil {
		err = temp.Chmod(info.Mode().Perm())
	}
	if err == nil {
		err = temp.Sync()
	}
	closeErr := temp.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Chtimes(temp.Name(), info.ModTime(), info.ModTime()); err != nil {
		return err
	}
	// Publish without replacing another capture, and never expose an incomplete history image.
	return publishScreenshotStorageFile(temp.Name(), dst)
}

// copyScreenshotStorageData bounds cancellation latency even for large editable desktop archives.
func copyScreenshotStorageData(ctx context.Context, dst io.Writer, src io.Reader) error {
	buffer := make([]byte, 1024*1024)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		n, readErr := src.Read(buffer)
		if n > 0 {
			if _, err := dst.Write(buffer[:n]); err != nil {
				return err
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

// screenshotStorageFilesEqual verifies a previously published copy before deleting its source on resume.
func screenshotStorageFilesEqual(ctx context.Context, left, right string) (bool, error) {
	var hashes [2][sha256.Size]byte
	for i, path := range []string{left, right} {
		file, err := os.Open(path)
		if err != nil {
			return false, err
		}
		hash := sha256.New()
		err = copyScreenshotStorageData(ctx, hash, file)
		file.Close()
		if err != nil {
			return false, err
		}
		copy(hashes[i][:], hash.Sum(nil))
	}
	return hashes[0] == hashes[1], nil
}

func screenshotStoragePathsEqual(left, right string) bool {
	if runtime.GOOS == "windows" {
		return strings.EqualFold(filepath.Clean(left), filepath.Clean(right))
	}
	return filepath.Clean(left) == filepath.Clean(right)
}
