//go:build windows

package recentfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// listRecent reads Recent/*.lnk first, then AutomaticDestinations. The .lnk
// files are missing when Windows Settings disables Start/Explorer recent files
// and Jump Lists; leftover jump-list DestList entries can still have paths.
func listRecent(ctx context.Context, option ListOption) ([]File, error) {
	limit := normalizeLimit(option.Limit)
	folder, err := windowsRecentFolder()
	if err != nil {
		return nil, err
	}

	candidates := make([]File, 0)
	lnkCount := appendRecentShortcutCandidates(folder, &candidates)
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
		if _, statErr := os.Lstat(candidate.Path); statErr != nil {
			continue
		}
		if !keepExistingRecentFile(candidate.Path, seen) {
			continue
		}
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

// appendRecentShortcutCandidates keeps the older Recent/*.lnk source when it still exists.
func appendRecentShortcutCandidates(folder string, candidates *[]File) int {
	entries, err := os.ReadDir(folder)
	if err != nil {
		return 0
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.EqualFold(filepath.Ext(entry.Name()), ".lnk") {
			continue
		}
		count++
		info, infoErr := entry.Info()
		if infoErr != nil {
			continue
		}
		target, ok := resolveRecentShortcut(filepath.Join(folder, entry.Name()))
		if !ok {
			continue
		}
		*candidates = append(*candidates, File{
			Path:     target,
			LastUsed: info.ModTime(),
		})
	}
	return count
}

// appendRecentJumpListCandidates reads DestList paths from AutomaticDestinations jump lists.
func appendRecentJumpListCandidates(folder string, candidates *[]File) (int, int) {
	jumpListDir := filepath.Join(folder, "AutomaticDestinations")
	entries, err := os.ReadDir(jumpListDir)
	if err != nil {
		return 0, 0
	}

	files := 0
	parsed := 0
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".automaticdestinations-ms") {
			continue
		}
		files++
		data, ok := readOleStream(filepath.Join(jumpListDir, entry.Name()), "DestList")
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
