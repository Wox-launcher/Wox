package system

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"wox/common"
	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

const folderResultScore int64 = 1000

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &FolderPlugin{})
}

type FolderPlugin struct {
	api             plugin.API
	showHiddenFiles atomic.Bool
}

// GetMetadata returns the system plugin metadata for direct folder browsing.
func (p *FolderPlugin) GetMetadata() plugin.Metadata {
	return plugin.Metadata{
		Id:            "527ba64f-c8f5-4fc7-bb98-306f79d27f32",
		Name:          "i18n:plugin_folder_plugin_name",
		Author:        "Wox Launcher",
		Website:       "https://github.com/Wox-launcher/Wox",
		Version:       "1.0.0",
		MinWoxVersion: "2.0.0",
		Runtime:       "Go",
		Description:   "i18n:plugin_folder_plugin_description",
		Icon:          common.FolderIcon.String(),
		Entry:         "",
		TriggerKeywords: []string{
			"*",
		},
		SupportedOS: []string{
			"Windows",
			"Macos",
			"Linux",
		},
	}
}

func (p *FolderPlugin) Init(ctx context.Context, initParams plugin.InitParams) {
	p.api = initParams.API
}

// Query handles global input that points to an existing folder path.
func (p *FolderPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	if query.Type != plugin.QueryTypeInput || !query.IsGlobalQuery() {
		return plugin.QueryResponse{}
	}

	folderPath, shouldListChildren, ok := parseFolderQueryPath(query.Search)
	if !ok {
		return plugin.QueryResponse{}
	}

	info, statErr := os.Stat(folderPath)
	if statErr != nil || !info.IsDir() {
		return plugin.QueryResponse{}
	}

	if shouldListChildren {
		return plugin.NewQueryResponse(p.queryChildren(ctx, folderPath))
	}

	return plugin.NewQueryResponse([]plugin.QueryResult{
		p.buildPathResult(folderPath, filepath.Base(folderPath), true, folderResultScore),
	})
}

// queryChildren lists one folder level so path browsing stays local and predictable.
func (p *FolderPlugin) queryChildren(ctx context.Context, folderPath string) []plugin.QueryResult {
	entries, readErr := os.ReadDir(folderPath)
	if readErr != nil {
		if p.api != nil {
			p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to read folder: path=%q err=%s", folderPath, readErr.Error()))
		}
		return []plugin.QueryResult{}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}

		leftName := strings.ToLower(entries[i].Name())
		rightName := strings.ToLower(entries[j].Name())
		if leftName == rightName {
			return entries[i].Name() < entries[j].Name()
		}
		return leftName < rightName
	})

	results := make([]plugin.QueryResult, 0, len(entries))
	showHiddenFiles := p.showHiddenFiles.Load()
	for _, entry := range entries {
		if !showHiddenFiles && isHiddenFolderEntry(entry) {
			continue
		}

		fullPath := filepath.Join(folderPath, entry.Name())
		results = append(results, p.buildPathResult(fullPath, entry.Name(), entry.IsDir(), 0))
	}
	return results
}

// buildPathResult creates a draggable file or folder result with preview support.
func (p *FolderPlugin) buildPathResult(path string, title string, isDir bool, score int64) plugin.QueryResult {
	if title == "" || title == "." || title == string(os.PathSeparator) {
		title = path
	}

	return plugin.QueryResult{
		Title:    title,
		SubTitle: path,
		Icon:     getFolderPluginPathIcon(path, isDir),
		Score:    score,
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeFile,
			PreviewData: path,
		},
		Actions: p.buildPathActions(path, isDir),
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: []string{path},
		},
	}
}

// buildPathActions keeps Enter opening the path while primary+Enter enters folders.
func (p *FolderPlugin) buildPathActions(path string, isDir bool) []plugin.QueryResultAction {
	openIcon := common.ExecuteRunIcon
	if isDir {
		openIcon = common.FolderIcon
	}

	actions := []plugin.QueryResultAction{
		{
			Name:      "i18n:plugin_folder_open",
			Icon:      openIcon,
			IsDefault: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				_ = shell.Open(path)
			},
		},
	}

	if isDir {
		actions = append(actions, plugin.QueryResultAction{
			Name:                   "i18n:plugin_folder_enter",
			Icon:                   common.FolderIcon,
			Hotkey:                 util.PrimaryHotkey("enter"),
			PreventHideAfterAction: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				if p.api != nil {
					p.api.ChangeQuery(ctx, common.PlainQuery{
						QueryType: plugin.QueryTypeInput,
						QueryText: ensureFolderQueryTrailingSeparator(path),
					})
				}
			},
		})
	}

	actions = append(actions, p.buildToggleHiddenFilesAction())
	return actions
}

// buildToggleHiddenFilesAction flips hidden-child visibility and refreshes the current folder query.
func (p *FolderPlugin) buildToggleHiddenFilesAction() plugin.QueryResultAction {
	showHiddenFiles := p.showHiddenFiles.Load()
	actionName := "i18n:plugin_folder_show_hidden_files"
	if showHiddenFiles {
		actionName = "i18n:plugin_folder_hide_hidden_files"
	}

	return plugin.QueryResultAction{
		Name:                   actionName,
		Icon:                   common.FolderIcon,
		PreventHideAfterAction: true,
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			p.showHiddenFiles.Store(!showHiddenFiles)
			if p.api != nil {
				p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
			}
		},
	}
}

// getFolderPluginPathIcon mirrors Explorer behavior for folders and app bundles.
func getFolderPluginPathIcon(path string, isDir bool) common.WoxImage {
	if !isDir {
		return common.NewWoxImageFileIcon(path)
	}

	if util.IsMacOS() && strings.EqualFold(filepath.Ext(strings.TrimRight(path, `/\`)), ".app") {
		return common.NewWoxImageFileIcon(path)
	}
	return common.FolderIcon
}

// isHiddenFolderEntry follows the same dotfile convention as File Search hidden filtering.
func isHiddenFolderEntry(entry os.DirEntry) bool {
	name := entry.Name()
	return strings.HasPrefix(name, ".") && name != "." && name != ".."
}

// parseFolderQueryPath converts a typed folder path into the filesystem path to inspect.
func parseFolderQueryPath(input string) (path string, shouldListChildren bool, ok bool) {
	trimmedInput := strings.TrimSpace(input)
	if trimmedInput == "" || !isFolderQueryPathLike(trimmedInput) {
		return "", false, false
	}

	shouldListChildren = hasFolderQueryTrailingSeparator(trimmedInput)
	lookupPath := trimFolderQueryTrailingSeparators(trimmedInput)
	expandedPath, expandErr := expandFolderQueryHome(lookupPath)
	if expandErr != nil {
		return "", false, false
	}

	return filepath.Clean(expandedPath), shouldListChildren, true
}

// isFolderQueryPathLike filters global queries before any filesystem stat.
func isFolderQueryPathLike(input string) bool {
	if input == "~" || strings.HasPrefix(input, "~/") || strings.HasPrefix(input, `~\`) {
		return true
	}
	return filepath.IsAbs(input)
}

// hasFolderQueryTrailingSeparator detects the browsing mode requested by the user.
func hasFolderQueryTrailingSeparator(input string) bool {
	return strings.HasSuffix(input, "/") || strings.HasSuffix(input, `\`)
}

// trimFolderQueryTrailingSeparators removes browsing separators without breaking roots.
func trimFolderQueryTrailingSeparators(input string) string {
	for hasFolderQueryTrailingSeparator(input) {
		next := input[:len(input)-1]
		if next == "" || isWindowsVolumeRoot(next) {
			break
		}
		input = next
	}
	return input
}

// isWindowsVolumeRoot keeps inputs like C:\ from becoming C: during trimming.
func isWindowsVolumeRoot(path string) bool {
	volumeName := filepath.VolumeName(path)
	return volumeName != "" && strings.EqualFold(volumeName, path)
}

// expandFolderQueryHome resolves ~ paths before filesystem checks.
func expandFolderQueryHome(path string) (string, error) {
	if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
		return path, nil
	}

	homeDir, homeErr := os.UserHomeDir()
	if homeErr != nil {
		return "", homeErr
	}
	if path == "~" {
		return homeDir, nil
	}

	return filepath.Join(homeDir, path[2:]), nil
}

// ensureFolderQueryTrailingSeparator builds the follow-up query for entering a folder.
func ensureFolderQueryTrailingSeparator(path string) string {
	if hasFolderQueryTrailingSeparator(path) {
		return path
	}
	return path + string(os.PathSeparator)
}
