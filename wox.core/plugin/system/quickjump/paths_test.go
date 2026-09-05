package quickjump

import "testing"

func TestWindowsAndUnixQuickJumpPathDetection(t *testing.T) {
	if !isWindowsQuickJumpPath(`C:\Users\qianlifeng\Projects`) {
		t.Fatal("expected a drive path to count as Windows")
	}
	if !isWindowsQuickJumpPath(`\\server\share\docs`) {
		t.Fatal("expected a UNC path to count as Windows")
	}
	if isWindowsQuickJumpPath(`/Users/qianlifeng/Projects`) {
		t.Fatal("unix paths must not count as Windows")
	}
	if !isUnixQuickJumpPath(`/Users/qianlifeng/Projects`) {
		t.Fatal("expected an absolute unix path")
	}
	if isUnixQuickJumpPath(`C:\Users\qianlifeng\Projects`) {
		t.Fatal("drive paths must not count as unix")
	}
	if isUnixQuickJumpPath(`\\server\share\docs`) {
		t.Fatal("UNC paths must not count as unix")
	}
}
