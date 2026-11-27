//go:build windows

package recentfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf16"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
)

/*
#cgo LDFLAGS: -lole32 -lshell32 -luuid
#include <stdlib.h>
#include <wchar.h>
#include "recentfiles_windows.h"
*/
import "C"

type cachedRecentShortcut struct {
	modTime int64
	size    int64
	target  string
	valid   bool
}

type cachedRecentJumpList struct {
	modTime int64
	size    int64
	items   []File
}

var (
	shortcutCache = util.NewHashMap[string, cachedRecentShortcut]()
	jumpListCache = util.NewHashMap[string, cachedRecentJumpList]()

	resolveRecentShortcutFn = resolveRecentShortcut
	readOleStreamFn         = readOleStream
)

// listRecent reads Recent/*.lnk first, then AutomaticDestinations. The .lnk
// files are missing when Windows Settings disables Start/Explorer recent files
// and Jump Lists; leftover jump-list DestList entries can still have paths.
func listRecent(ctx context.Context, option ListOption) ([]File, error) {
	folder, err := windowsRecentFolder()
	if err != nil {
		return nil, err
	}
	return listRecentFromFolder(ctx, folder, option)
}

func listRecentFromFolder(ctx context.Context, folder string, option ListOption) ([]File, error) {
	limit := normalizeLimit(option.Limit)

	candidates := make([]File, 0)
	lnkCount := appendRecentShortcutCandidates(folder, limit, &candidates)
	jumpListFiles, parsedEntries := appendRecentJumpListCandidates(folder, &candidates)

	sort.SliceStable(candidates, func(i, j int) bool {
		return candidates[i].LastUsed.After(candidates[j].LastUsed)
	})

	seen := make(map[string]bool, limit)
	results := make([]File, 0, limit)
	for _, candidate := range candidates {
		if len(results) >= limit {
			break
		}
		// Deduplicate before Lstat so overlapping .lnk and JumpList paths are checked at most once.
		if !keepExistingRecentFile(candidate.Path, seen) {
			continue
		}
		info, statErr := os.Lstat(candidate.Path)
		if statErr != nil {
			continue
		}
		candidate.HasMeta = true
		candidate.IsDir = info.IsDir()
		candidate.ModTime = info.ModTime()
		candidate.Size = info.Size()
		results = append(results, candidate)
	}
	if !util.IsTestMode() {
		util.GetLogger().Info(ctx, fmt.Sprintf(
			"windows recent files: folder=%s lnks=%d jumplists=%d parsed=%d candidates=%d kept=%d",
			folder, lnkCount, jumpListFiles, parsedEntries, len(candidates), len(results),
		))
	}
	return results, nil
}

type lnkCandidateEntry struct {
	key     string
	path    string
	modTime time.Time
	size    int64
}

// appendRecentShortcutCandidates keeps the older Recent/*.lnk source when it still exists,
// caching resolved targets by (modTime, size) and capping COM resolution to the newest entries.
func appendRecentShortcutCandidates(folder string, limit int, candidates *[]File) int {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0
	}

	lnkEntries := make([]lnkCandidateEntry, 0, len(entries))
	present := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lnk") {
			continue
		}
		fullPath := filepath.Join(folder, entry.Name())
		key := strings.ToLower(fullPath)
		present[key] = struct{}{}
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		lnkEntries = append(lnkEntries, lnkCandidateEntry{
			key:     key,
			path:    fullPath,
			modTime: info.ModTime(),
			size:    info.Size(),
		})
	}

	sort.Slice(lnkEntries, func(i, j int) bool {
		if !lnkEntries[i].modTime.Equal(lnkEntries[j].modTime) {
			return lnkEntries[i].modTime.After(lnkEntries[j].modTime)
		}
		return lnkEntries[i].key < lnkEntries[j].key
	})

	capCount := max(limit*2, 120)
	if len(lnkEntries) > capCount {
		lnkEntries = lnkEntries[:capCount]
	}

	for _, item := range lnkEntries {
		modNano := item.modTime.UnixNano()
		if cached, hit := shortcutCache.Load(item.key); hit && cached.modTime == modNano && cached.size == item.size {
			if cached.valid {
				*candidates = append(*candidates, File{Path: cached.target, LastUsed: item.modTime})
			}
			continue
		}
		target, ok := resolveRecentShortcutFn(item.path)
		shortcutCache.Store(item.key, cachedRecentShortcut{
			modTime: modNano,
			size:    item.size,
			target:  target,
			valid:   ok,
		})
		if ok {
			*candidates = append(*candidates, File{Path: target, LastUsed: item.modTime})
		}
	}

	folderPrefix := strings.ToLower(filepath.Clean(folder)) + string(filepath.Separator)
	for _, key := range shortcutCache.Keys() {
		if strings.HasPrefix(key, folderPrefix) {
			if _, ok := present[key]; !ok {
				shortcutCache.Delete(key)
			}
		}
	}
	return len(present)
}

// appendRecentJumpListCandidates reads DestList paths from AutomaticDestinations jump lists,
// reusing parsed items when (modTime, size) is unchanged.
func appendRecentJumpListCandidates(folder string, candidates *[]File) (int, int) {
	jumpListDir := filepath.Join(folder, "AutomaticDestinations")
	entries, err := os.ReadDir(jumpListDir)
	if err != nil {
		return 0, 0
	}

	files := 0
	parsed := 0
	present := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".automaticdestinations-ms") {
			continue
		}
		files++
		fullPath := filepath.Join(jumpListDir, entry.Name())
		key := strings.ToLower(fullPath)
		present[key] = struct{}{}

		info, infoErr := entry.Info()
		if infoErr == nil {
			modNano := info.ModTime().UnixNano()
			size := info.Size()
			if cached, hit := jumpListCache.Load(key); hit && cached.modTime == modNano && cached.size == size {
				*candidates = append(*candidates, cached.items...)
				parsed += len(cached.items)
				continue
			}

			var items []File
			if data, ok := readOleStreamFn(fullPath, "DestList"); ok {
				for _, item := range parseDestList(data) {
					if strings.TrimSpace(item.Path) == "" {
						continue
					}
					items = append(items, File{
						Path:     filepath.Clean(item.Path),
						LastUsed: item.LastUsed,
					})
				}
			}
			jumpListCache.Store(key, cachedRecentJumpList{
				modTime: modNano,
				size:    size,
				items:   items,
			})
			*candidates = append(*candidates, items...)
			parsed += len(items)
			continue
		}

		data, ok := readOleStreamFn(fullPath, "DestList")
		if !ok {
			continue
		}
		for _, item := range parseDestList(data) {
			parsed++
			if strings.TrimSpace(item.Path) == "" {
				continue
			}
			*candidates = append(*candidates, File{
				Path:     filepath.Clean(item.Path),
				LastUsed: item.LastUsed,
			})
		}
	}

	dirPrefix := strings.ToLower(filepath.Clean(jumpListDir)) + string(filepath.Separator)
	for _, key := range jumpListCache.Keys() {
		if strings.HasPrefix(key, dirPrefix) {
			if _, ok := present[key]; !ok {
				jumpListCache.Delete(key)
			}
		}
	}
	return files, parsed
}

func readOleStream(storagePath string, streamName string) ([]byte, bool) {
	cPath, err := utf16PtrForC(storagePath)
	if err != nil {
		return nil, false
	}
	cStream, err := utf16PtrForC(streamName)
	if err != nil {
		return nil, false
	}
	var raw *C.uchar
	var length C.int
	if C.wox_recent_read_ole_stream(cPath, cStream, &raw, &length) != 0 || raw == nil || length <= 0 {
		return nil, false
	}
	defer C.wox_recent_free(unsafe.Pointer(raw))
	return C.GoBytes(unsafe.Pointer(raw), length), true
}

func recordAccess(_ context.Context, path string) error {
	cleaned := strings.TrimSpace(path)
	if cleaned == "" {
		return nil
	}
	cPath, err := utf16PtrForC(cleaned)
	if err != nil {
		return err
	}
	C.wox_recent_record_access(cPath)
	return nil
}

func windowsRecentFolder() (string, error) {
	if folder, err := windows.KnownFolderPath(windows.FOLDERID_Recent, 0); err == nil && strings.TrimSpace(folder) != "" {
		return folder, nil
	}
	appData := strings.TrimSpace(os.Getenv("APPDATA"))
	if appData == "" {
		return "", fmt.Errorf("Windows recent folder is unavailable")
	}
	return filepath.Join(appData, "Microsoft", "Windows", "Recent"), nil
}

func resolveRecentShortcut(lnkPath string) (string, bool) {
	cPath, err := utf16PtrForC(lnkPath)
	if err != nil {
		return "", false
	}
	out := make([]uint16, windows.MAX_PATH)
	if C.wox_recent_resolve_lnk(cPath, (*C.wchar_t)(unsafe.Pointer(&out[0])), C.int(len(out))) != 0 {
		return "", false
	}
	target := strings.TrimSpace(utf16ToString(out))
	if target == "" {
		return "", false
	}
	return filepath.Clean(target), true
}

func utf16PtrForC(value string) (*C.wchar_t, error) {
	ptr, err := windows.UTF16PtrFromString(value)
	if err != nil {
		return nil, err
	}
	return (*C.wchar_t)(unsafe.Pointer(ptr)), nil
}

func utf16ToString(values []uint16) string {
	end := 0
	for end < len(values) && values[end] != 0 {
		end++
	}
	return string(utf16.Decode(values[:end]))
}
