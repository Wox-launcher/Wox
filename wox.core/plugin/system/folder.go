package system

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"
	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/setting/definition"
	"wox/setting/validator"
	"wox/util"
	"wox/util/shell"
)

const (
	folderResultScore int64 = 1000

	folderFavoritesSettingKey     = "favorites"
	folderFavoriteFormNameKey     = "name"
	folderFavoriteFormPathKey     = "path"
	folderFavoriteContextNameKey  = "favorite_name"
	folderFavoriteContextPathKey  = "favorite_path"
	folderFavoriteContextIndexKey = "favorite_index"
)

func init() {
	plugin.AllSystemPlugin = append(plugin.AllSystemPlugin, &FolderPlugin{})
}

type FolderPlugin struct {
	api             plugin.API
	showHiddenFiles atomic.Bool
}

type folderFavorite struct {
	Name string `json:"Name"`
	Path string `json:"Path"`
}

type folderFavoriteMatch struct {
	Name  string
	Path  string
	Index int
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
		SettingDefinitions: definition.PluginSettingDefinitions{
			{
				Type: definition.PluginSettingDefinitionTypeTable,
				Value: &definition.PluginSettingValueTable{
					Key:     folderFavoritesSettingKey,
					Title:   "i18n:plugin_folder_favorites",
					Tooltip: "i18n:plugin_folder_favorites_tooltip",
					Columns: []definition.PluginSettingValueTableColumn{
						{
							Key:   "Name",
							Label: "i18n:plugin_folder_favorite_name",
							Type:  definition.PluginSettingValueTableColumnTypeText,
							Width: 120,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
								{
									Type:  validator.PluginSettingValidatorTypeUnique,
									Value: &validator.PluginSettingValidatorUnique{},
								},
							},
						},
						{
							Key:   "Path",
							Label: "i18n:plugin_folder_favorite_path",
							Type:  definition.PluginSettingValueTableColumnTypeDirPath,
							Width: 220,
							Validators: []validator.PluginSettingValidator{
								{
									Type:  validator.PluginSettingValidatorTypeNotEmpty,
									Value: &validator.PluginSettingValidatorNotEmpty{},
								},
							},
						},
					},
				},
			},
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
		return plugin.NewQueryResponse(p.queryFavorites(ctx, query.Search))
	}

	info, statErr := os.Stat(folderPath)
	if statErr != nil || !info.IsDir() {
		return plugin.NewQueryResponse(p.queryFavorites(ctx, query.Search))
	}

	if shouldListChildren {
		return plugin.NewQueryResponse(p.queryChildren(ctx, folderPath))
	}

	favoriteMatch := p.findFavoriteByPath(ctx, folderPath, p.loadFavorites(ctx))
	return plugin.NewQueryResponse([]plugin.QueryResult{
		p.buildPathResult(folderPath, filepath.Base(folderPath), true, folderResultScore, favoriteMatch),
	})
}

// queryFavorites searches configured folder favorites by their saved name.
func (p *FolderPlugin) queryFavorites(ctx context.Context, search string) []plugin.QueryResult {
	search = strings.TrimSpace(search)
	if search == "" {
		return nil
	}

	favorites := p.loadFavorites(ctx)
	if len(favorites) == 0 {
		return nil
	}

	var results []plugin.QueryResult
	searchLower := strings.ToLower(search)
	for favoriteIndex, favorite := range favorites {
		name := strings.TrimSpace(favorite.Name)
		path := strings.TrimSpace(favorite.Path)
		if name == "" || path == "" || !strings.HasPrefix(strings.ToLower(name), searchLower) {
			continue
		}

		resolvedPath, ok := p.resolveFavoritePath(ctx, path, false)
		if !ok {
			continue
		}

		results = append(results, p.buildFavoriteResult(name, resolvedPath, favoriteIndex, int64(len(favorites)-favoriteIndex)))
	}

	return results
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
	favorites := p.loadFavorites(ctx)
	for _, entry := range entries {
		if !showHiddenFiles && isHiddenFolderEntry(entry) {
			continue
		}

		fullPath := filepath.Join(folderPath, entry.Name())
		var favoriteMatch *folderFavoriteMatch
		if entry.IsDir() {
			favoriteMatch = p.findFavoriteByPath(ctx, fullPath, favorites)
		}
		results = append(results, p.buildPathResult(fullPath, entry.Name(), entry.IsDir(), 0, favoriteMatch))
	}
	return results
}

// buildPathResult creates a draggable file or folder result with preview support.
func (p *FolderPlugin) buildPathResult(path string, title string, isDir bool, score int64, favoriteMatch *folderFavoriteMatch) plugin.QueryResult {
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
		Tails:   buildFolderFavoriteTails(favoriteMatch),
		Actions: p.buildPathActions(path, isDir, favoriteMatch),
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: []string{path},
		},
	}
}

// buildFavoriteResult creates a saved-folder result with actions for opening, navigating, and maintaining the favorite.
func (p *FolderPlugin) buildFavoriteResult(name string, path string, favoriteIndex int, scoreBoost int64) plugin.QueryResult {
	return plugin.QueryResult{
		Title:    name,
		SubTitle: path,
		Icon:     getFolderPluginPathIcon(path, true),
		Score:    folderResultScore + scoreBoost,
		Group:    "i18n:plugin_folder_favorites",
		Preview: plugin.WoxPreview{
			PreviewType: plugin.WoxPreviewTypeFile,
			PreviewData: path,
		},
		Actions: p.buildFavoriteActions(name, path, favoriteIndex),
		DragData: &plugin.QueryResultDragData{
			Type:  plugin.QueryResultDragDataTypeFiles,
			Files: []string{path},
		},
	}
}

// buildPathActions keeps Enter opening the path while primary+Enter enters folders.
func (p *FolderPlugin) buildPathActions(path string, isDir bool, favoriteMatch *folderFavoriteMatch) []plugin.QueryResultAction {
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
		if favoriteMatch != nil {
			actions = append(actions, p.buildEditFavoriteAction(favoriteMatch.Name, favoriteMatch.Path, favoriteMatch.Index), p.buildDeleteFavoriteAction(favoriteMatch.Name, favoriteMatch.Path, favoriteMatch.Index))
		} else {
			actions = append(actions, p.buildAddFavoriteAction(filepath.Base(path), path))
		}
	}

	actions = append(actions, p.buildToggleHiddenFilesAction())
	return actions
}

// buildFavoriteActions adds setting maintenance actions to saved folder results.
func (p *FolderPlugin) buildFavoriteActions(name string, path string, favoriteIndex int) []plugin.QueryResultAction {
	actions := []plugin.QueryResultAction{
		{
			Name:      "i18n:plugin_folder_open",
			Icon:      common.FolderIcon,
			IsDefault: true,
			Action: func(ctx context.Context, actionContext plugin.ActionContext) {
				_ = shell.Open(path)
			},
		},
		{
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
		},
		p.buildEditFavoriteAction(name, path, favoriteIndex),
		p.buildDeleteFavoriteAction(name, path, favoriteIndex),
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

// buildAddFavoriteAction persists the selected folder as a named favorite.
func (p *FolderPlugin) buildAddFavoriteAction(name string, path string) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "add_folder_favorite",
		Name:                   "i18n:plugin_folder_add_as_favorite",
		Icon:                   common.PluginBookmarkIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		ContextData:            p.buildFavoriteActionContextData(name, path, -1),
		Form:                   buildFolderFavoriteForm(name, path),
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			actionName, actionPath, _ := folderFavoriteDataFromActionContext(actionContext.ActionContext, name, path, -1)
			favorite := folderFavorite{
				Name: strings.TrimSpace(actionContext.Values[folderFavoriteFormNameKey]),
				Path: strings.TrimSpace(actionContext.Values[folderFavoriteFormPathKey]),
			}
			if favorite.Name == "" {
				favorite.Name = actionName
			}
			if favorite.Path == "" {
				favorite.Path = actionPath
			}
			if resolvedPath, ok := p.resolveFavoritePath(ctx, favorite.Path, true); ok {
				favorite.Path = resolvedPath
			} else {
				return
			}

			if err := p.addFavorite(ctx, favorite); err != nil {
				p.notifyFavoriteSettingError(ctx, "plugin_folder_favorite_add_failed", err)
				return
			}
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_added"))
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	}
}

// buildEditFavoriteAction updates an existing favorite from the result action panel.
func (p *FolderPlugin) buildEditFavoriteAction(name string, path string, favoriteIndex int) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "edit_folder_favorite",
		Name:                   "i18n:plugin_folder_edit_favorite",
		Icon:                   common.EditIcon,
		Type:                   plugin.QueryResultActionTypeForm,
		PreventHideAfterAction: true,
		ContextData:            p.buildFavoriteActionContextData(name, path, favoriteIndex),
		Form:                   buildFolderFavoriteForm(name, path),
		OnSubmit: func(ctx context.Context, actionContext plugin.FormActionContext) {
			_, _, currentIndex := folderFavoriteDataFromActionContext(actionContext.ActionContext, name, path, favoriteIndex)
			favorite := folderFavorite{
				Name: strings.TrimSpace(actionContext.Values[folderFavoriteFormNameKey]),
				Path: strings.TrimSpace(actionContext.Values[folderFavoriteFormPathKey]),
			}
			if resolvedPath, ok := p.resolveFavoritePath(ctx, favorite.Path, true); ok {
				favorite.Path = resolvedPath
			} else {
				return
			}

			if err := p.updateFavorite(ctx, currentIndex, favorite); err != nil {
				p.notifyFavoriteSettingError(ctx, "plugin_folder_favorite_update_failed", err)
				return
			}
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_updated"))
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
		},
	}
}

// buildDeleteFavoriteAction removes a saved folder favorite.
func (p *FolderPlugin) buildDeleteFavoriteAction(name string, path string, favoriteIndex int) plugin.QueryResultAction {
	return plugin.QueryResultAction{
		Id:                     "delete_folder_favorite",
		Name:                   "i18n:plugin_folder_delete_favorite",
		Icon:                   common.TrashIcon,
		PreventHideAfterAction: true,
		ContextData:            p.buildFavoriteActionContextData(name, path, favoriteIndex),
		Action: func(ctx context.Context, actionContext plugin.ActionContext) {
			_, _, currentIndex := folderFavoriteDataFromActionContext(actionContext, name, path, favoriteIndex)
			if err := p.deleteFavorite(ctx, currentIndex); err != nil {
				p.notifyFavoriteSettingError(ctx, "plugin_folder_favorite_delete_failed", err)
				return
			}
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_deleted"))
			p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: false})
		},
	}
}

// buildFolderFavoriteForm builds the action form used for adding and editing favorites.
func buildFolderFavoriteForm(name string, path string) definition.PluginSettingDefinitions {
	return definition.PluginSettingDefinitions{
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          folderFavoriteFormNameKey,
				Label:        "i18n:plugin_folder_favorite_name",
				DefaultValue: strings.TrimSpace(name),
				Validators: []validator.PluginSettingValidator{
					{
						Type:  validator.PluginSettingValidatorTypeNotEmpty,
						Value: &validator.PluginSettingValidatorNotEmpty{},
					},
				},
			},
		},
		{
			Type: definition.PluginSettingDefinitionTypeTextBox,
			Value: &definition.PluginSettingValueTextBox{
				Key:          folderFavoriteFormPathKey,
				Label:        "i18n:plugin_folder_favorite_path",
				DefaultValue: strings.TrimSpace(path),
				Validators: []validator.PluginSettingValidator{
					{
						Type:  validator.PluginSettingValidatorTypeNotEmpty,
						Value: &validator.PluginSettingValidatorNotEmpty{},
					},
				},
			},
		},
	}
}

// loadFavorites loads configured folder favorites from settings.
func (p *FolderPlugin) loadFavorites(ctx context.Context) []folderFavorite {
	if p.api == nil {
		return nil
	}

	raw := strings.TrimSpace(p.api.GetSetting(ctx, folderFavoritesSettingKey))
	if raw == "" {
		return nil
	}

	var favorites []folderFavorite
	if err := json.Unmarshal([]byte(raw), &favorites); err != nil {
		p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to unmarshal folder favorites: %s", err.Error()))
		return nil
	}

	return favorites
}

// findFavoriteByPath returns the saved favorite that points at the displayed folder path.
func (p *FolderPlugin) findFavoriteByPath(ctx context.Context, path string, favorites []folderFavorite) *folderFavoriteMatch {
	normalizedPath, normalizeErr := normalizeFolderFavoritePath(path)
	if normalizeErr != nil {
		return nil
	}

	for favoriteIndex, favorite := range favorites {
		name := strings.TrimSpace(favorite.Name)
		favoritePath := strings.TrimSpace(favorite.Path)
		if name == "" || favoritePath == "" {
			continue
		}

		normalizedFavoritePath, favoriteNormalizeErr := normalizeFolderFavoritePath(favoritePath)
		if favoriteNormalizeErr != nil {
			if p.api != nil {
				p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Skipping folder favorite with invalid path syntax: name=%q path=%q err=%s", name, favoritePath, favoriteNormalizeErr.Error()))
			}
			continue
		}

		if !isSameFolderPath(normalizedPath, normalizedFavoritePath) {
			continue
		}

		return &folderFavoriteMatch{
			Name:  name,
			Path:  normalizedFavoritePath,
			Index: favoriteIndex,
		}
	}

	return nil
}

func buildFolderFavoriteTails(favoriteMatch *folderFavoriteMatch) []plugin.QueryResultTail {
	if favoriteMatch == nil || strings.TrimSpace(favoriteMatch.Name) == "" {
		return nil
	}

	return []plugin.QueryResultTail{plugin.NewQueryResultTailText(favoriteMatch.Name)}
}

// addFavorite appends a new folder favorite to the setting table.
func (p *FolderPlugin) addFavorite(ctx context.Context, favorite folderFavorite) error {
	favorites := p.loadFavorites(ctx)
	favorites = append(favorites, favorite)
	return p.saveFavorites(ctx, favorites)
}

// updateFavorite updates one saved folder favorite by table index.
func (p *FolderPlugin) updateFavorite(ctx context.Context, favoriteIndex int, favorite folderFavorite) error {
	favorites := p.loadFavorites(ctx)
	if favoriteIndex < 0 || favoriteIndex >= len(favorites) {
		return fmt.Errorf("favorite index out of range: %d", favoriteIndex)
	}

	favorites[favoriteIndex] = favorite
	return p.saveFavorites(ctx, favorites)
}

// deleteFavorite removes one saved folder favorite by table index.
func (p *FolderPlugin) deleteFavorite(ctx context.Context, favoriteIndex int) error {
	favorites := p.loadFavorites(ctx)
	if favoriteIndex < 0 || favoriteIndex >= len(favorites) {
		return fmt.Errorf("favorite index out of range: %d", favoriteIndex)
	}

	favorites = append(favorites[:favoriteIndex], favorites[favoriteIndex+1:]...)
	return p.saveFavorites(ctx, favorites)
}

// saveFavorites writes the favorite table while keeping name lookup deterministic.
func (p *FolderPlugin) saveFavorites(ctx context.Context, favorites []folderFavorite) error {
	if err := validateFolderFavorites(ctx, favorites); err != nil {
		return err
	}

	data, err := json.Marshal(favorites)
	if err != nil {
		return err
	}

	p.api.SaveSetting(ctx, folderFavoritesSettingKey, string(data), false)
	return nil
}

// validateFolderFavorites prevents ambiguous case-insensitive name matches.
func validateFolderFavorites(ctx context.Context, favorites []folderFavorite) error {
	seen := make(map[string]struct{}, len(favorites))
	for _, favorite := range favorites {
		name := strings.TrimSpace(favorite.Name)
		if name == "" {
			return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_name_required"))
		}
		if strings.TrimSpace(favorite.Path) == "" {
			return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_path_required"))
		}

		key := strings.ToLower(name)
		if _, exists := seen[key]; exists {
			return errors.New(i18n.GetI18nManager().TranslateWox(ctx, "ui_validator_value_must_be_unique"))
		}
		seen[key] = struct{}{}
	}

	return nil
}

func (p *FolderPlugin) resolveFavoritePath(ctx context.Context, path string, notify bool) (string, bool) {
	cleanedPath, err := normalizeFolderFavoritePath(path)
	if err != nil {
		if notify && p.api != nil {
			p.api.Notify(ctx, i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_path_required"))
		}
		return "", false
	}

	info, statErr := os.Stat(cleanedPath)
	if statErr != nil || !info.IsDir() {
		if notify && p.api != nil {
			p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, "plugin_folder_favorite_path_invalid"), cleanedPath))
		} else if p.api != nil {
			p.api.Log(ctx, plugin.LogLevelWarning, fmt.Sprintf("Skipping invalid folder favorite path: path=%q err=%v", cleanedPath, statErr))
		}
		return "", false
	}

	return cleanedPath, true
}

func normalizeFolderFavoritePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", errors.New("path is empty")
	}

	expandedPath, expandErr := expandFolderQueryHome(path)
	if expandErr != nil {
		return "", expandErr
	}
	return filepath.Clean(expandedPath), nil
}

func isSameFolderPath(left string, right string) bool {
	if util.IsWindows() || util.IsMacOS() {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func (p *FolderPlugin) notifyFavoriteSettingError(ctx context.Context, messageKey string, err error) {
	if p.api == nil {
		return
	}
	p.api.Log(ctx, plugin.LogLevelError, fmt.Sprintf("Failed to update folder favorites: %s", err.Error()))
	p.api.Notify(ctx, fmt.Sprintf(i18n.GetI18nManager().TranslateWox(ctx, messageKey), err.Error()))
}

func (p *FolderPlugin) buildFavoriteActionContextData(name string, path string, favoriteIndex int) map[string]string {
	contextData := map[string]string{
		folderFavoriteContextNameKey:  strings.TrimSpace(name),
		folderFavoriteContextPathKey:  strings.TrimSpace(path),
		folderFavoriteContextIndexKey: strconv.Itoa(favoriteIndex),
	}
	return contextData
}

func folderFavoriteDataFromActionContext(actionContext plugin.ActionContext, fallbackName string, fallbackPath string, fallbackIndex int) (string, string, int) {
	name := strings.TrimSpace(actionContext.ContextData[folderFavoriteContextNameKey])
	if name == "" {
		name = strings.TrimSpace(fallbackName)
	}

	path := strings.TrimSpace(actionContext.ContextData[folderFavoriteContextPathKey])
	if path == "" {
		path = strings.TrimSpace(fallbackPath)
	}

	favoriteIndex := fallbackIndex
	if rawIndex := strings.TrimSpace(actionContext.ContextData[folderFavoriteContextIndexKey]); rawIndex != "" {
		if parsedIndex, err := strconv.Atoi(rawIndex); err == nil {
			favoriteIndex = parsedIndex
		}
	}

	return name, path, favoriteIndex
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
