//go:build darwin

package recentfiles

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"
)

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <stdlib.h>
#include "recentfiles_darwin.h"
*/
import "C"

// listRecent uses Spotlight last-used dates so empty File Search can show OS recents.
func listRecent(ctx context.Context, option ListOption) ([]File, error) {
	_ = ctx
	limit := normalizeLimit(option.Limit)

	var raw *C.char
	if !C.wox_recent_files_mdquery(C.int(limit), &raw) {
		return nil, fmt.Errorf("spotlight recent-file query failed")
	}
	if raw == nil {
		return nil, nil
	}
	defer C.wox_recent_files_free(raw)

	seen := make(map[string]bool, limit)
	results := make([]File, 0, limit)
	for _, path := range splitCStringList(raw) {
		if len(results) >= limit {
			break
		}
		cleaned := filepath.Clean(path)
		info, err := os.Lstat(cleaned)
		if err != nil {
			continue
		}
		if !keepExistingRecentFile(cleaned, seen) {
			continue
		}
		results = append(results, File{
			Path:     cleaned,
			LastUsed: info.ModTime(),
		})
	}
	return results, nil
}

func recordAccess(_ context.Context, _ string) error {
	// Opening a file through Launch Services already updates Spotlight last-used dates.
	return nil
}

func splitCStringList(raw *C.char) []string {
	if raw == nil {
		return nil
	}
	// Walk the double-NUL terminated buffer copied from MDQuery.
	ptr := unsafe.Pointer(raw)
	values := make([]string, 0)
	for {
		part := C.GoString((*C.char)(ptr))
		if part == "" {
			break
		}
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			values = append(values, trimmed)
		}
		ptr = unsafe.Pointer(uintptr(ptr) + uintptr(len(part)+1))
	}
	return values
}
