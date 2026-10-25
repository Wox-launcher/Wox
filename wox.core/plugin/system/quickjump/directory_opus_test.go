package quickjump

import "testing"

// Captured from `dopusrt /info <file>,paths` on Directory Opus 13.20 with a dual-pane
// lister holding background tabs, plus a second lister whose source pane is "This PC".
const directoryOpusPathsFixture = `<?xml version="1.0"?>
<results command="paths" result="1">
	<path active_lister="0" active_tab="1" display_path="C:\Users\liuhao\Documents\dev\Dev" lister="0xc088a" side="1" tab="0x80d30" tab_state="2">C:\Users\liuhao\workspace\dev\Dev</path>
	<path active_lister="1" display_path="D:\" lister="0x91032" side="1" tab="0x60f5e">D:\</path>
	<path active_lister="1" active_tab="1" display_path="E:\M" lister="0x91032" side="1" tab="0x2d112a" tab_state="2">E:\M</path>
	<path active_lister="1" display_path="C:\Users\liuhao\Documents\dev\Dev" lister="0x91032" side="2" tab="0x19063a">C:\Users\liuhao\workspace\dev\Dev</path>
	<path active_lister="1" active_tab="2" display_path="C:\Users\liuhao\Documents" lister="0x91032" side="2" tab="0x100f6c" tab_state="1">C:\Users\liuhao\workspace</path>
</results>`

func TestSelectDirectoryOpusSourcePathUsesSourceTabRealPath(t *testing.T) {
	got := selectDirectoryOpusSourcePath([]byte(directoryOpusPathsFixture), 0x91032)
	if got != `C:\Users\liuhao\workspace` {
		t.Fatalf("source path = %q, want the real path of the tab_state=1 tab", got)
	}
}

func TestSelectDirectoryOpusSourcePathSupportsSinglePaneLister(t *testing.T) {
	// Single-pane listers report side="0"; the visible tab is still the tab_state=1 source.
	output := `<?xml version="1.0"?>
<results command="paths" result="1">
	<path active_lister="1" display_path="D:\btsync_book_war" lister="0x91032" side="0" tab="0x150de8">D:\btsync_book_war</path>
	<path active_lister="1" active_tab="1" display_path="E:\M" lister="0x91032" side="0" tab="0xd1414" tab_state="1">E:\M</path>
</results>`
	if got := selectDirectoryOpusSourcePath([]byte(output), 0x91032); got != `E:\M` {
		t.Fatalf("source path = %q, want the visible tab of the single pane", got)
	}
}

func TestSelectDirectoryOpusSourcePathDecodesUTF8Paths(t *testing.T) {
	// dopusrt writes UTF-8 without a BOM or an encoding declaration.
	output := `<?xml version="1.0"?>
<results command="paths" result="1">
	<path active_lister="1" active_tab="2" display_path="D:\中文 目录" lister="0x91032" side="2" tab="0x1c11a2" tab_state="1">D:\中文 目录</path>
</results>`
	if got := selectDirectoryOpusSourcePath([]byte(output), 0x91032); got != `D:\中文 目录` {
		t.Fatalf("source path = %q", got)
	}
}

func TestSelectDirectoryOpusSourcePathSkipsListerWithoutFilesystemSource(t *testing.T) {
	// The source pane of lister 0xc088a is the virtual "This PC" folder, which dopusrt
	// omits. Falling back to the destination pane would jump to a folder the user is not viewing.
	if got := selectDirectoryOpusSourcePath([]byte(directoryOpusPathsFixture), 0xc088a); got != "" {
		t.Fatalf("source path = %q, want empty", got)
	}
}

func TestSelectDirectoryOpusSourcePathRejectsUnknownListerAndBadOutput(t *testing.T) {
	if got := selectDirectoryOpusSourcePath([]byte(directoryOpusPathsFixture), 0x1234); got != "" {
		t.Fatalf("unknown lister path = %q, want empty", got)
	}
	failed := `<?xml version="1.0"?><results command="paths" result="0"><path lister="0x91032" tab_state="1">C:\x</path></results>`
	if got := selectDirectoryOpusSourcePath([]byte(failed), 0x91032); got != "" {
		t.Fatalf("failed result path = %q, want empty", got)
	}
	if got := selectDirectoryOpusSourcePath([]byte("not xml"), 0x91032); got != "" {
		t.Fatalf("malformed output path = %q, want empty", got)
	}
}
