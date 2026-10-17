package system

import (
	"context"
	"strings"

	"wox/common"
	"wox/plugin"
	"wox/plugin/system/migrateapply"
)

const (
	favoriteImportImported = "imported"
	favoriteImportExists   = "exists"
	favoriteImportConflict = "conflict"
	favoriteImportFailed   = "failed"
)

// ImportedFavorite is one file or folder quicklink mapped onto a Folder favorite.
type ImportedFavorite struct {
	Name string
	Path string
}

// ImportFavorites appends file and folder favorites. An existing name is not overwritten.
func (p *FolderPlugin) ImportFavorites(ctx context.Context, items []ImportedFavorite) []migrateapply.Result {
	results := make([]migrateapply.Result, len(items))
	if p == nil || p.api == nil {
		for index := range results {
			results[index] = migrateapply.Result{Status: favoriteImportFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	favorites := p.loadFavorites(ctx)
	changed := false
	for index, item := range items {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			results[index] = migrateapply.Result{Status: favoriteImportFailed, ErrorCode: "failed"}
			continue
		}
		resolved, _, ok := p.resolveFavoritePath(ctx, item.Path, false)
		if !ok {
			results[index] = migrateapply.Result{Status: favoriteImportFailed, ErrorCode: "missing"}
			continue
		}
		if existing, found := folderFavoriteByName(favorites, name); found {
			if isSameFolderPath(existing.Path, resolved) {
				results[index] = migrateapply.Result{Status: favoriteImportExists}
			} else {
				results[index] = migrateapply.Result{Status: favoriteImportConflict, ErrorCode: "conflict"}
			}
			continue
		}
		favorites = append(favorites, folderFavorite{Name: name, Path: resolved})
		results[index] = migrateapply.Result{Status: favoriteImportImported}
		changed = true
	}
	if !changed {
		return results
	}
	if err := p.saveFavorites(ctx, favorites); err != nil {
		for index := range results {
			if results[index].Status == favoriteImportImported {
				results[index] = migrateapply.Result{Status: favoriteImportFailed, ErrorCode: "failed"}
			}
		}
	}
	return results
}

func folderFavoriteByName(favorites []folderFavorite, name string) (folderFavorite, bool) {
	for _, favorite := range favorites {
		if strings.EqualFold(strings.TrimSpace(favorite.Name), name) {
			return favorite, true
		}
	}
	return folderFavorite{}, false
}

func importMigratedFavorites(ctx context.Context, items []migrateapply.Favorite) []migrateapply.Result {
	results := make([]migrateapply.Result, len(items))
	var target *FolderPlugin
	if manager := plugin.GetPluginManager(); manager != nil {
		target, _ = manager.GetSystemPlugin(common.FolderPluginID).(*FolderPlugin)
	}
	if target == nil {
		for index := range results {
			results[index] = migrateapply.Result{Status: favoriteImportFailed, ErrorCode: "unavailable"}
		}
		return results
	}
	incoming := make([]ImportedFavorite, len(items))
	for index, item := range items {
		incoming[index] = ImportedFavorite{Name: item.Name, Path: item.Path}
	}
	return target.ImportFavorites(ctx, incoming)
}
