package manifest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"wox/database"
	"wox/i18n"
	"wox/plugin"
	"wox/setting"
	"wox/util"
	"wox/util/trash"

	"github.com/Masterminds/semver/v3"
)

const flowStoreDownloadTimeout = 2 * time.Minute

// flowStoreCatalogURLs are tried in order. The first body that parses as a JSON
// array wins, including an array whose plugins are all filtered out. The two
// lists are not merged, because they publish the same plugin IDs.
var flowStoreCatalogURLs = []string{
	"https://cdn.jsdelivr.net/gh/Flow-Launcher/Flow.Launcher.PluginsManifest@plugin_api_v2/plugins.json",
	"https://raw.githubusercontent.com/Flow-Launcher/Flow.Launcher.PluginsManifest/plugin_api_v2/plugins.json",
}

type flowStore struct{}

func (flowStore) Name() string { return "flow" }

func (flowStore) AppendManifests(ctx context.Context, manifests []plugin.StorePluginManifest) []plugin.StorePluginManifest {
	return appendFlowStoreManifests(ctx, manifests)
}

func (flowStore) Owns(manifest plugin.StorePluginManifest) bool {
	return strings.EqualFold(string(manifest.Runtime), string(RuntimeJSONRPC))
}

func (flowStore) Install(ctx context.Context, manifest plugin.StorePluginManifest, progress plugin.InstallProgressCallback) error {
	return installFlowStorePlugin(ctx, manifest, progress)
}

func (flowStore) OwnsInstance(instance *plugin.Instance) bool {
	return instance != nil && strings.EqualFold(instance.Metadata.Runtime, string(RuntimeJSONRPC))
}

func (flowStore) Uninstall(ctx context.Context, instance *plugin.Instance, skipCleanSetting bool, preserveCache bool, progress plugin.UninstallProgressCallback) (bool, error) {
	return uninstallFlowStorePlugin(ctx, instance, skipCleanSetting, preserveCache, progress)
}

// Store is the catalog implementation registered by the parent flow package.
func Store() plugin.ExternalStore { return flowStore{} }

var _ plugin.ExternalStore = flowStore{}

// appendFlowStoreManifests adds script plugins from the flow catalog.
// A fetch or parse failure keeps the Wox list that the caller already built.
func appendFlowStoreManifests(ctx context.Context, manifests []plugin.StorePluginManifest) []plugin.StorePluginManifest {
	fetched, err := fetchFlowStoreManifests(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow plugin store was not loaded: %s", err.Error()))
		return manifests
	}
	var installed []*plugin.Instance
	if manager := plugin.GetPluginManager(); manager != nil {
		installed = manager.GetPluginInstances()
	}
	return mergeFlowStoreManifests(manifests, fetched, installed, CollectionDirectory())
}

// fetchFlowStoreManifests returns the first catalog that is valid JSON.
func fetchFlowStoreManifests(ctx context.Context) ([]plugin.StorePluginManifest, error) {
	var lastErr error
	for _, catalogURL := range flowStoreCatalogURLs {
		body, err := util.HttpGet(ctx, catalogURL)
		if err != nil {
			lastErr = err
			util.GetLogger().Warn(ctx, fmt.Sprintf("flow plugin catalog %s: %s", catalogURL, err.Error()))
			continue
		}
		manifests, err := flowStoreManifestsFromJSON(body)
		if err != nil {
			lastErr = err
			util.GetLogger().Warn(ctx, fmt.Sprintf("flow plugin catalog %s: %s", catalogURL, err.Error()))
			continue
		}
		return manifests, nil
	}
	if lastErr == nil {
		lastErr = errors.New("flow plugin catalog was not found")
	}
	return nil, lastErr
}

// flowStoreManifestsFromJSON keeps Python, Node, and executable plugins.
// C# and F# rows are dropped until the dotnet host can start them.
// MinimumAppVersion belongs to the other app and is not copied onto MinWoxVersion.
func flowStoreManifestsFromJSON(raw []byte) ([]plugin.StorePluginManifest, error) {
	if len(raw) >= 3 && raw[0] == 0xEF && raw[1] == 0xBB && raw[2] == 0xBF {
		raw = raw[3:]
	}
	var documents []map[string]any
	if err := json.Unmarshal(raw, &documents); err != nil {
		return nil, err
	}

	manifests := make([]plugin.StorePluginManifest, 0, len(documents))
	indexByID := map[string]int{}
	for _, document := range documents {
		manifest, ok := flowStoreManifestFromDocument(document)
		if !ok {
			continue
		}
		key := flowStoreIDKey(manifest.Id)
		if previous, found := indexByID[key]; found {
			if flowStoreVersionGreater(manifest.Version, manifests[previous].Version) {
				manifests[previous] = manifest
			}
			continue
		}
		indexByID[key] = len(manifests)
		manifests = append(manifests, manifest)
	}
	return manifests, nil
}

// flowStoreManifestFromDocument converts one catalog object. A missing required
// field or an unsupported language skips that object without failing the array.
func flowStoreManifestFromDocument(document map[string]any) (plugin.StorePluginManifest, bool) {
	if document == nil || LanguageKind(flowFieldString(document, "Language")) != KindScript {
		return plugin.StorePluginManifest{}, false
	}
	id := flowFieldString(document, "ID", "Id")
	downloadURL := flowFieldString(document, "UrlDownload")
	version := flowFieldString(document, "Version")
	if id == "" || downloadURL == "" || version == "" || !flowStoreIDSafe(id) {
		return plugin.StorePluginManifest{}, false
	}
	name := flowFieldString(document, "Name")
	if name == "" {
		name = id
	}
	website := flowFieldString(document, "Website")
	if website == "" {
		website = flowFieldString(document, "UrlSourceCode")
	}
	return plugin.StorePluginManifest{
		Id:            id,
		Name:          name,
		Author:        flowFieldString(document, "Author"),
		Version:       version,
		MinWoxVersion: minWoxVersion,
		Runtime:       RuntimeJSONRPC,
		Description:   flowFieldString(document, "Description"),
		IconUrl:       flowFieldString(document, "IcoPath"),
		Website:       website,
		DownloadUrl:   downloadURL,
		SupportedOS:   []string{"Windows", "Darwin", "Linux"},
		DateCreated:   flowFieldString(document, "DateCreated"),
		DateUpdated:   flowFieldString(document, "DateUpdated"),
	}, true
}

// mergeFlowStoreManifests drops a flow row whose ID is already a Wox store plugin
// or an installed plugin that does not live in the flow collection directory.
func mergeFlowStoreManifests(existing, fetched []plugin.StorePluginManifest, installed []*plugin.Instance, root string) []plugin.StorePluginManifest {
	blocked := map[string]struct{}{}
	for _, item := range existing {
		if key := flowStoreIDKey(item.Id); key != "" {
			blocked[key] = struct{}{}
		}
	}
	for _, instance := range installed {
		if instance == nil || flowPluginDirectoryIsChild(root, instance.PluginDirectory) {
			continue
		}
		if key := flowStoreIDKey(instance.Metadata.Id); key != "" {
			blocked[key] = struct{}{}
		}
	}
	for _, item := range fetched {
		key := flowStoreIDKey(item.Id)
		if key == "" {
			continue
		}
		if _, found := blocked[key]; found {
			continue
		}
		existing = append(existing, item)
		blocked[key] = struct{}{}
	}
	return existing
}

// installFlowStorePlugin downloads one catalog zip into plugins/flow-jsonrpc/<id>.
// The caller already holds the store install lock.
func installFlowStorePlugin(ctx context.Context, manifest plugin.StorePluginManifest, progress plugin.InstallProgressCallback) error {
	var installed *plugin.Instance
	if manager := plugin.GetPluginManager(); manager != nil {
		installed = manager.GetPluginInstanceById(manifest.Id)
	}
	root := CollectionDirectory()
	if err := flowStoreInstallRejection(manifest, installed, root); err != nil {
		return err
	}

	tempDir, err := os.MkdirTemp("", "wox-flow-store-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)

	reportFlowProgress(ctx, progress, "plugin_install_progress_starting_download")
	zipPath := filepath.Join(tempDir, "plugin.zip")
	if err := downloadFlowPlugin(ctx, manifest.DownloadUrl, zipPath, progress); err != nil {
		return err
	}
	reportFlowProgress(ctx, progress, "plugin_install_progress_download_complete")

	extractDir := filepath.Join(tempDir, "extract")
	if err := os.MkdirAll(extractDir, os.ModePerm); err != nil {
		return err
	}
	reportFlowProgress(ctx, progress, "plugin_install_progress_extracting")
	if err := util.Unzip(zipPath, extractDir); err != nil {
		return fmt.Errorf("extract flow plugin %s: %w", manifest.Id, err)
	}
	reportFlowProgress(ctx, progress, "plugin_install_progress_extraction_complete")

	// Parse before replacing files so a bad archive does not unload the installed copy.
	pluginDir, err := locateFlowPluginDirectory(extractDir)
	if err != nil {
		return err
	}
	descriptor, err := Parse(pluginDir)
	if err != nil {
		return err
	}
	if err := validateFlowStoreDescriptor(descriptor, manifest.Id); err != nil {
		return err
	}

	oldDir := ""
	if installed != nil {
		oldDir = installed.PluginDirectory
		plugin.GetPluginManager().UnloadPlugin(ctx, installed)
	}
	reportFlowProgress(ctx, progress, "plugin_install_progress_loading")
	installedMetadata, err := stageFlowPlugin(extractDir, root, manifest.Id)
	if err != nil {
		restoreFlowPlugin(ctx, oldDir)
		return err
	}
	if err := plugin.GetPluginManager().LoadRuntimePlugin(ctx, installedMetadata); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to load flow plugin %s: %s", manifest.Id, err.Error()))
		if removeErr := os.RemoveAll(installedMetadata.Directory); removeErr != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove flow plugin directory %s: %s", installedMetadata.Directory, removeErr.Error()))
		}
		return fmt.Errorf("failed to load flow plugin %s: %w", manifest.Id, err)
	}
	reportFlowProgress(ctx, progress, "plugin_install_progress_complete")
	return nil
}

// flowStoreInstallRejection reports why this manifest must not replace an install.
// A newer installed semver is the only version that blocks replacement.
func flowStoreInstallRejection(manifest plugin.StorePluginManifest, installed *plugin.Instance, root string) error {
	if !strings.EqualFold(string(manifest.Runtime), string(RuntimeJSONRPC)) {
		return fmt.Errorf("flow plugin store cannot install runtime %s", manifest.Runtime)
	}
	if !flowStoreIDSafe(manifest.Id) {
		return fmt.Errorf("flow plugin id %q is not a safe directory name", manifest.Id)
	}
	if strings.TrimSpace(manifest.DownloadUrl) == "" {
		return fmt.Errorf("flow plugin %s is missing a download url", strings.TrimSpace(manifest.Id))
	}
	if installed == nil {
		return nil
	}
	if !flowPluginDirectoryIsChild(root, installed.PluginDirectory) {
		return fmt.Errorf("plugin %s is already installed and is not a flow plugin", strings.TrimSpace(manifest.Id))
	}
	if plugin.IsVersionUpgradable(manifest.Version, installed.Metadata.Version) {
		return fmt.Errorf("skip %s(%s), because it's already installed(%s)", manifest.Name, manifest.Version, installed.Metadata.Version)
	}
	return nil
}

// uninstallFlowStorePlugin removes one child of the flow collection directory.
// It returns false when the instance is not one of those children, so the caller
// can keep its own uninstall path. It does not stop the shared flow host.
func uninstallFlowStorePlugin(ctx context.Context, instance *plugin.Instance, skipCleanSetting bool, preserveCache bool, progress plugin.UninstallProgressCallback) (bool, error) {
	if instance == nil {
		return false, nil
	}
	root := CollectionDirectory()
	if !flowPluginDirectoryIsChild(root, instance.PluginDirectory) {
		return false, nil
	}

	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_preparing")
	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_unloading")
	plugin.GetPluginManager().UnloadPlugin(ctx, instance)

	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_removing")
	if err := removeFlowPluginDirectory(ctx, instance.PluginDirectory); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to remove flow plugin directory %s: %s", instance.PluginDirectory, err.Error()))
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_restoring_plugin", instance.Metadata.GetName(ctx))
		restoreFlowPlugin(ctx, instance.PluginDirectory)
		return true, err
	}

	if !skipCleanSetting {
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_cleaning_settings")
		if db := database.GetDB(); db != nil {
			if err := setting.NewPluginSettingStore(db, instance.Metadata.Id).DeleteAll(); err != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("failed to delete flow plugin settings %s: %s", instance.Metadata.Id, err.Error()))
			}
		}
	}
	if !preserveCache {
		reportFlowProgress(ctx, progress, "plugin_uninstall_progress_cleaning_cache")
		if err := util.GetLocation().RemovePluginCacheDirectory(instance.Metadata.Id); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to delete flow plugin cache %s: %s", instance.Metadata.Id, err.Error()))
		}
	}
	reportFlowProgress(ctx, progress, "plugin_uninstall_progress_complete")
	return true, nil
}

// stageFlowPlugin moves a parsed archive into collectionRoot/<plugin id>.
// plugin.json may sit at the archive root or one directory down.
func stageFlowPlugin(extractedDir, collectionRoot, expectedID string) (plugin.Metadata, error) {
	pluginDir, err := locateFlowPluginDirectory(extractedDir)
	if err != nil {
		return plugin.Metadata{}, err
	}
	descriptor, err := Parse(pluginDir)
	if err != nil {
		return plugin.Metadata{}, err
	}
	if err := validateFlowStoreDescriptor(descriptor, expectedID); err != nil {
		return plugin.Metadata{}, err
	}
	if err := util.GetLocation().EnsureDirectoryExist(collectionRoot); err != nil {
		return plugin.Metadata{}, err
	}
	dest := filepath.Join(collectionRoot, descriptor.Metadata.Id)
	if !flowPluginDirectoryIsChild(collectionRoot, dest) {
		return plugin.Metadata{}, fmt.Errorf("flow plugin destination escapes the collection directory")
	}
	if err := replaceDirectory(dest, pluginDir); err != nil {
		return plugin.Metadata{}, err
	}
	installed, err := Parse(dest)
	if err != nil {
		if removeErr := os.RemoveAll(dest); removeErr != nil {
			return plugin.Metadata{}, fmt.Errorf("parse installed flow plugin: %w (cleanup: %v)", err, removeErr)
		}
		return plugin.Metadata{}, err
	}
	return installed.Metadata, nil
}

func validateFlowStoreDescriptor(descriptor Descriptor, expectedID string) error {
	if !strings.EqualFold(descriptor.Metadata.Id, expectedID) {
		return fmt.Errorf("flow plugin id %s does not match %s", descriptor.Metadata.Id, expectedID)
	}
	if descriptor.Kind != KindScript {
		return fmt.Errorf("flow plugin %s language %s is not supported", descriptor.Metadata.Id, descriptor.Language)
	}
	if !flowStoreIDSafe(descriptor.Metadata.Id) {
		return fmt.Errorf("flow plugin id %q is not a safe directory name", descriptor.Metadata.Id)
	}
	return nil
}

// locateFlowPluginDirectory finds plugin.json at the extract root or in exactly
// one child directory. __MACOSX and dot directories are ignored.
func locateFlowPluginDirectory(extractedDir string) (string, error) {
	if _, err := os.Stat(filepath.Join(extractedDir, "plugin.json")); err == nil {
		return extractedDir, nil
	}
	entries, err := os.ReadDir(extractedDir)
	if err != nil {
		return "", fmt.Errorf("read flow plugin archive: %w", err)
	}
	var pluginDir string
	for _, entry := range entries {
		name := entry.Name()
		if !entry.IsDir() || strings.HasPrefix(name, ".") || strings.EqualFold(name, "__MACOSX") {
			continue
		}
		candidate := filepath.Join(extractedDir, name)
		if _, err := os.Stat(filepath.Join(candidate, "plugin.json")); err != nil {
			continue
		}
		if pluginDir != "" {
			return "", errors.New("flow plugin archive contains more than one plugin directory")
		}
		pluginDir = candidate
	}
	if pluginDir == "" {
		return "", errors.New("flow plugin archive does not contain plugin.json")
	}
	return pluginDir, nil
}

// flowPluginDirectoryIsChild reports whether directory is exactly one safe path
// element inside root. The collection root itself is not a plugin.
func flowPluginDirectoryIsChild(root, directory string) bool {
	root = filepath.Clean(filepath.FromSlash(strings.TrimSpace(root)))
	directory = filepath.Clean(filepath.FromSlash(strings.TrimSpace(directory)))
	if root == "" || root == "." || directory == "" || directory == "." {
		return false
	}
	compareRoot, compareDirectory := root, directory
	if runtime.GOOS == "windows" {
		compareRoot = strings.ToLower(root)
		compareDirectory = strings.ToLower(directory)
	}
	relative, err := filepath.Rel(compareRoot, compareDirectory)
	if err != nil || relative == "." || relative == "" {
		return false
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return false
	}
	if strings.ContainsRune(relative, filepath.Separator) {
		return false
	}
	return flowStoreIDSafe(relative)
}

// removeFlowPluginDirectory deletes one plugin directory. Trash is preferred,
// and a failed trash move falls back to deleting the directory in place.
func removeFlowPluginDirectory(ctx context.Context, directory string) error {
	if err := trash.MoveToTrash(directory); err != nil {
		// In-place deletion covers a trash backend that cannot see this directory.
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to move flow plugin directory %s to trash: %s", directory, err.Error()))
	} else {
		return nil
	}

	delays := []time.Duration{0, 100 * time.Millisecond, 300 * time.Millisecond}
	var removeErr error
	for attempt, delay := range delays {
		if delay > 0 {
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		removeErr = os.RemoveAll(directory)
		if removeErr == nil || os.IsNotExist(removeErr) {
			return nil
		}
		util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove flow plugin directory %s on attempt %d: %s", directory, attempt+1, removeErr.Error()))
	}
	return removeErr
}

// replaceDirectory moves src onto dest. A cross-volume rename is copied instead.
func replaceDirectory(dest, src string) error {
	if _, err := os.Stat(dest); err == nil {
		if err := os.RemoveAll(dest); err != nil {
			return fmt.Errorf("remove existing flow plugin directory: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.Rename(src, dest); err == nil {
		return nil
	}
	if err := copyDirectory(src, dest); err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return nil
}

func copyDirectory(src, dest string) error {
	return filepath.WalkDir(src, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, os.ModePerm)
		}
		if err := os.MkdirAll(filepath.Dir(target), os.ModePerm); err != nil {
			return err
		}
		return copyFile(path, target)
	})
}

func copyFile(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	info, err := in.Stat()
	if err != nil {
		return err
	}
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

func downloadFlowPlugin(ctx context.Context, downloadURL, dest string, progress plugin.InstallProgressCallback) error {
	downloadCtx, cancel := context.WithTimeout(ctx, flowStoreDownloadTimeout)
	defer cancel()
	err := util.HttpDownloadWithProgress(downloadCtx, downloadURL, dest, func(downloaded int64, total int64) {
		if total > 0 {
			percentage := float64(downloaded) / float64(total) * 100
			reportFlowProgress(ctx, progress, "plugin_install_progress_downloading", percentage)
			return
		}
		reportFlowProgress(ctx, progress, "plugin_install_progress_downloaded_bytes", downloaded)
	})
	if err != nil {
		if removeErr := os.Remove(dest); removeErr != nil && !os.IsNotExist(removeErr) {
			util.GetLogger().Warn(ctx, fmt.Sprintf("failed to remove incomplete flow plugin download %s: %s", dest, removeErr.Error()))
		}
	}
	if errors.Is(err, context.DeadlineExceeded) && ctx.Err() == nil {
		return fmt.Errorf("flow plugin download timed out after %s: %w", flowStoreDownloadTimeout, err)
	}
	return err
}

// restoreFlowPlugin loads the directory again after a failed replace or delete.
func restoreFlowPlugin(ctx context.Context, directory string) {
	if strings.TrimSpace(directory) == "" {
		return
	}
	descriptor, err := Parse(directory)
	if err != nil || descriptor.Kind != KindScript {
		return
	}
	if err := plugin.GetPluginManager().LoadRuntimePlugin(ctx, descriptor.Metadata); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to restore flow plugin %s: %s", directory, err.Error()))
	}
}

func reportFlowProgress(ctx context.Context, progress func(string), key string, args ...any) {
	if progress == nil {
		return
	}
	message := i18n.GetI18nManager().TranslateWox(ctx, "i18n:"+key)
	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}
	progress(message)
}

func flowStoreVersionGreater(candidate, current string) bool {
	next, nextErr := semver.NewVersion(candidate)
	prev, prevErr := semver.NewVersion(current)
	if nextErr != nil || prevErr != nil || next == nil || prev == nil {
		return false
	}
	return next.GreaterThan(prev)
}

func flowStoreIDKey(id string) string {
	return strings.ToLower(strings.TrimSpace(id))
}

// flowStoreIDSafe rejects ids that would escape plugins/flow-jsonrpc/<id>.
func flowStoreIDSafe(id string) bool {
	id = strings.TrimSpace(id)
	if id == "" || id == "." || id == ".." {
		return false
	}
	if strings.ContainsAny(id, `/\`) || strings.Contains(id, "..") {
		return false
	}
	return filepath.Base(id) == id
}
