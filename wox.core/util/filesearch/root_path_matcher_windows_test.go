//go:build windows

package filesearch

import (
	"path/filepath"
	"testing"
)

func TestRootPathMatcherWindowsCaseInsensitiveLongestRoot(t *testing.T) {
	parent := RootRecord{ID: "root-parent", Path: `C:\Users\Qian`}
	dynamic := RootRecord{ID: "root-dynamic", Path: `C:\Users\Qian\Dev\Wox`, Kind: RootKindDynamic, DynamicParentRootID: parent.ID}
	matcher := newRootPathMatcher([]RootRecord{parent, dynamic})

	matched, ok := matcher.findClean(filepath.Clean(`c:\users\qian\dev\wox\main.go`))
	if !ok || matched.ID != dynamic.ID {
		t.Fatalf("expected case-insensitive dynamic root match, got %#v ok=%t", matched, ok)
	}
}

func TestRootPathMatcherWindowsRejectsVolumeSiblingPrefix(t *testing.T) {
	root := RootRecord{ID: "root-project", Path: `D:\Projects\Wox`}
	matcher := newRootPathMatcher([]RootRecord{root})

	if matched, ok := matcher.findClean(filepath.Clean(`D:\Projects\WoxBackup\main.go`)); ok {
		t.Fatalf("expected sibling prefix not to match, got %#v", matched)
	}
}
