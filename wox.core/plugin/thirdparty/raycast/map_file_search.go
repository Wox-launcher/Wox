package raycast

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"wox/common"
	"wox/plugin/system/file_search/indexpolicy"
	"wox/plugin/thirdparty/migrate"
)

const raycastFileSearchExtensionID = "e:r:file-search"

// fileSearchMeta is the subset of Raycast File Search preferences that Wox stores.
// Scanner engines, thread counts, OCR limits, and volume policies have no Wox setting.
type fileSearchMeta struct {
	Include                            []string `json:"include"`
	Ignore                             []string `json:"ignore"`
	ContentSearch                      *bool    `json:"contentSearch"`
	IncludeHiddenFiles                 *bool    `json:"includeHiddenFiles"`
	SemanticIndexingPaths              []string `json:"semanticIndexingPaths"`
	SemanticIndexingUseFileSearchScope *bool    `json:"semanticIndexingUseFileSearchScope"`
	ViewState                          *struct {
		ShowDetails *bool `json:"showDetails"`
	} `json:"viewState"`
}

// mapFileSearchRows maps overlapping File Search preferences onto the Wox file-search plugin.
// Directory and ignore rows are only the values Wox does not already have. Import merges those
// rows into the current tables so Wox defaults and extra directories stay in place.
func mapFileSearchRows(payload Payload, current woxSnapshot) (rows []mappedSetting, roots, ignores, contentRoots []string) {
	meta, ok := fileSearchMetaFrom(payload)
	if !ok {
		return nil, nil, nil, nil
	}
	if current.FileRootsKnown {
		roots = append([]string(nil), current.FileRoots...)
	}
	if current.FileIgnoreKnown {
		ignores = append([]string(nil), current.FileIgnorePatterns...)
	}
	if current.ContentRootsKnown {
		contentRoots = append([]string(nil), current.ContentRoots...)
	}

	seenRoots := append([]string(nil), roots...)
	for index, path := range meta.Include {
		cleaned := cleanImportedPath(path)
		if cleaned == "" || containsImportedPath(seenRoots, cleaned) {
			continue
		}
		seenRoots = append(seenRoots, cleaned)
		rows = append(rows, fileSearchPathRow("setting:file_root:"+strconv.Itoa(index), "file_directory", "roots", "roots", cleaned))
	}
	seenIgnore := append([]string(nil), ignores...)
	for index, pattern := range meta.Ignore {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || containsIgnorePattern(seenIgnore, pattern) {
			continue
		}
		seenIgnore = append(seenIgnore, pattern)
		rows = append(rows, fileSearchPathRow("setting:file_ignore:"+strconv.Itoa(index), "file_ignore_pattern", "ignorePatterns", "ignore", pattern))
	}
	if meta.IncludeHiddenFiles != nil {
		row := boolMappedSetting("setting:file_skip_hidden", "file_skip_hidden", "skipHiddenFiles", !*meta.IncludeHiddenFiles, current.SkipHiddenKnown, current.SkipHidden)
		row.pluginID = common.FileSearchPluginID
		rows = append(rows, row)
	}
	if meta.ViewState != nil && meta.ViewState.ShowDetails != nil {
		row := boolMappedSetting("setting:file_preview", "file_preview", "showPreview", *meta.ViewState.ShowDetails, current.ShowPreviewKnown, current.ShowPreview)
		row.pluginID = common.FileSearchPluginID
		rows = append(rows, row)
	}
	if meta.ContentSearch != nil {
		row := boolMappedSetting("setting:file_content_search", "file_content_search", "contentSearchEnabled", *meta.ContentSearch, current.ContentSearchKnown, current.ContentSearch)
		row.pluginID = common.FileSearchPluginID
		rows = append(rows, row)
	}

	seenContent := append([]string(nil), contentRoots...)
	for index, path := range fileSearchContentPaths(meta) {
		cleaned := cleanImportedPath(path)
		if cleaned == "" || containsImportedPath(seenContent, cleaned) {
			continue
		}
		seenContent = append(seenContent, cleaned)
		rows = append(rows, fileSearchPathRow("setting:file_content_root:"+strconv.Itoa(index), "file_content_directory", "contentRoots", "content_roots", cleaned))
	}
	return rows, roots, ignores, contentRoots
}

// fileSearchContentPaths returns content directories that are independent of the filename roots.
// An empty semantic list follows the filename directories only while content search is enabled.
func fileSearchContentPaths(meta fileSearchMeta) []string {
	paths := append([]string(nil), meta.SemanticIndexingPaths...)
	useScope := meta.SemanticIndexingUseFileSearchScope != nil && *meta.SemanticIndexingUseFileSearchScope
	if len(paths) == 0 && useScope && meta.ContentSearch != nil && *meta.ContentSearch {
		paths = append(paths, meta.Include...)
	}
	return paths
}

func fileSearchMetaFrom(payload Payload) (fileSearchMeta, bool) {
	for _, extension := range payload.Settings.InternalExtensions {
		if extension.ID != raycastFileSearchExtensionID || len(extension.Meta) == 0 {
			continue
		}
		var meta fileSearchMeta
		if json.Unmarshal(extension.Meta, &meta) != nil {
			return fileSearchMeta{}, false
		}
		return meta, true
	}
	return fileSearchMeta{}, false
}

func fileSearchPathRow(id, titleKey, key, merge, value string) mappedSetting {
	return mappedSetting{
		id: id, titleKey: titleKey, key: key, value: value, detail: value,
		pluginID: common.FileSearchPluginID, merge: merge, selectable: true,
	}
}

// fileSearchSettingWrites merges the selected directory and ignore rows into the current tables.
// The stored value is the full table, because an empty Wox setting means "use defaults" and a
// partial write would drop those defaults.
func fileSearchSettingWrites(mapped mappedExport, selected map[string]struct{}) []migrate.SettingWrite {
	var writes []migrate.SettingWrite
	var rootIDs, ignoreIDs, contentIDs []string
	var roots, ignores, contents []string
	for _, row := range mapped.fileSearch {
		if _, ok := selected[row.id]; !ok || !row.selectable {
			continue
		}
		switch row.merge {
		case "roots":
			rootIDs = append(rootIDs, row.id)
			roots = append(roots, row.value)
		case "ignore":
			ignoreIDs = append(ignoreIDs, row.id)
			ignores = append(ignores, row.value)
		case "content_roots":
			contentIDs = append(contentIDs, row.id)
			contents = append(contents, row.value)
		default:
			writes = append(writes, migrate.SettingWrite{
				ItemIDs: []string{row.id}, Key: row.key, Value: row.value, PluginID: row.pluginID,
			})
		}
	}
	if len(roots) > 0 {
		if raw, err := json.Marshal(pathTable(mergeImportedPaths(mapped.fileRoots, roots))); err == nil {
			writes = append(writes, migrate.SettingWrite{
				ItemIDs: rootIDs, Key: "roots", Value: string(raw), PluginID: common.FileSearchPluginID,
			})
		}
	}
	if len(ignores) > 0 {
		merged := withMandatoryIgnore(mergeImportedPatterns(mapped.fileIgnore, ignores))
		if raw, err := json.Marshal(patternTable(merged)); err == nil {
			writes = append(writes, migrate.SettingWrite{
				ItemIDs: ignoreIDs, Key: "ignorePatterns", Value: string(raw), PluginID: common.FileSearchPluginID,
			})
		}
	}
	if len(contents) > 0 {
		if raw, err := json.Marshal(pathTable(mergeImportedPaths(mapped.contentRoots, contents))); err == nil {
			writes = append(writes, migrate.SettingWrite{
				ItemIDs: contentIDs, Key: "contentRoots", Value: string(raw), PluginID: common.FileSearchPluginID,
			})
		}
	}
	return writes
}

func pathTable(paths []string) []struct {
	Path string `json:"Path"`
} {
	rows := make([]struct {
		Path string `json:"Path"`
	}, 0, len(paths))
	for _, path := range paths {
		rows = append(rows, struct {
			Path string `json:"Path"`
		}{Path: path})
	}
	return rows
}

func patternTable(patterns []string) []struct {
	Pattern string `json:"Pattern"`
} {
	rows := make([]struct {
		Pattern string `json:"Pattern"`
	}, 0, len(patterns))
	for _, pattern := range patterns {
		rows = append(rows, struct {
			Pattern string `json:"Pattern"`
		}{Pattern: pattern})
	}
	return rows
}

func mergeImportedPaths(current, incoming []string) []string {
	merged := append([]string(nil), current...)
	for _, path := range incoming {
		if path == "" || containsImportedPath(merged, path) {
			continue
		}
		merged = append(merged, path)
	}
	return merged
}

func mergeImportedPatterns(current, incoming []string) []string {
	merged := append([]string(nil), current...)
	for _, pattern := range incoming {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" || containsIgnorePattern(merged, pattern) {
			continue
		}
		merged = append(merged, pattern)
	}
	return merged
}

func withMandatoryIgnore(patterns []string) []string {
	required := indexpolicy.WoxFileSearchStorageIgnorePattern
	if containsIgnorePattern(patterns, required) {
		return patterns
	}
	return append(append([]string(nil), patterns...), required)
}

func containsImportedPath(paths []string, path string) bool {
	for _, item := range paths {
		if sameImportedPath(item, path) {
			return true
		}
	}
	return false
}

func containsIgnorePattern(patterns []string, pattern string) bool {
	for _, item := range patterns {
		if strings.EqualFold(strings.TrimSpace(item), strings.TrimSpace(pattern)) {
			return true
		}
	}
	return false
}

func sameImportedPath(left, right string) bool {
	return strings.EqualFold(cleanImportedPath(left), cleanImportedPath(right))
}

// cleanImportedPath expands a home-relative Raycast directory and cleans the result.
func cleanImportedPath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if path == "~" || strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		home, err := os.UserHomeDir()
		if err != nil || strings.TrimSpace(home) == "" {
			return filepath.Clean(path)
		}
		if path == "~" {
			return filepath.Clean(home)
		}
		rest := strings.TrimPrefix(strings.TrimPrefix(path, "~/"), `~\`)
		return filepath.Clean(filepath.Join(home, rest))
	}
	return filepath.Clean(path)
}

// applyFileSearchDefaults fills the comparison baseline used when a setting has never been saved.
// An empty stored value means these defaults, so matching Raycast values are not offered again.
func applyFileSearchDefaults(snap *woxSnapshot) {
	snap.FileRootsKnown = true
	snap.FileRoots = defaultMigrationRoots()
	snap.FileIgnoreKnown = true
	snap.FileIgnorePatterns = indexpolicy.DefaultIgnorePatterns()
	snap.SkipHiddenKnown = true
	snap.SkipHidden = true
	snap.ShowPreviewKnown = true
	snap.ShowPreview = true
	snap.ContentSearchKnown = true
	snap.ContentSearch = false
	snap.ContentRootsKnown = true
	snap.ContentRoots = append([]string(nil), snap.FileRoots...)
}

func defaultMigrationRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(home) == "" {
		return nil
	}
	return []string{filepath.Clean(home)}
}

func parseImportedPathTable(raw string) ([]string, error) {
	var rows []struct {
		Path string `json:"Path"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(rows))
	for _, row := range rows {
		if path := cleanImportedPath(row.Path); path != "" {
			paths = append(paths, path)
		}
	}
	return paths, nil
}

func parseImportedPatternTable(raw string) ([]string, error) {
	var rows []struct {
		Pattern string `json:"Pattern"`
	}
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		return nil, err
	}
	patterns := make([]string, 0, len(rows))
	for _, row := range rows {
		if pattern := strings.TrimSpace(row.Pattern); pattern != "" {
			patterns = append(patterns, pattern)
		}
	}
	return patterns, nil
}
