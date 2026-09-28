package shell

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"unicode/utf16"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
)

// writeLaunchShortcut creates real Shell links without launching their targets.
func writeLaunchShortcut(t *testing.T, path, target, args, directory string, show int32) {
	t.Helper()
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cleanup, err := initializeCOMForShell()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	link, err := ole.CreateInstance(ole.NewGUID("{00021401-0000-0000-C000-000000000046}"), ole.NewGUID("{000214F9-0000-0000-C000-000000000046}"))
	if err != nil {
		t.Fatal(err)
	}
	defer link.Release()
	for _, field := range []struct {
		slot  int
		value string
	}{{20, target}, {11, args}, {9, directory}} {
		value, err := windows.UTF16PtrFromString(field.value)
		if err != nil || !shortcutCOMCall(link, field.slot, uintptr(unsafe.Pointer(value))) {
			t.Fatalf("set Shell link field %d: %v", field.slot, err)
		}
	}
	if !shortcutCOMCall(link, 15, uintptr(show)) {
		t.Fatal("set show command")
	}
	var persist *ole.IUnknown
	if err := link.PutQueryInterface(ole.NewGUID("{0000010b-0000-0000-C000-000000000046}"), &persist); err != nil {
		t.Fatal(err)
	}
	defer persist.Release()
	if !shortcutCOMCall(persist, 6, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(path))), 1) {
		t.Fatal("save Shell link")
	}
}

func TestShortcutLaunchRequestPreservesLaunchFields(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "音乐播放器.LNK")
	target := filepath.Join(os.Getenv("WINDIR"), "explorer.exe")
	args := `--profile="测试 profile" --literal=%TEMP% "a&b" ""`
	t.Setenv("WOX_SHORTCUT_TEST_DIR", root)
	for _, show := range []int32{1, 3, 7} {
		writeLaunchShortcut(t, path, target, args, "%WOX_SHORTCUT_TEST_DIR%", show)
		for _, verb := range []string{"open", "runas"} {
			req, ok := shortcutLaunchRequest(path, verb)
			if !ok || !strings.EqualFold(req.File, target) || req.Parameters != args || req.Directory != root || req.Show != show || req.Verb != verb {
				t.Fatalf("show=%d verb=%s: resolved=%v request=%+v", show, verb, ok, req)
			}
		}
	}
	writeLaunchShortcut(t, path, `%WINDIR%\explorer.exe`, args, "", 1)
	if req, ok := shortcutLaunchRequest(path, "open"); !ok || !strings.EqualFold(req.File, target) || req.Parameters != args {
		t.Fatalf("environment target lost: resolved=%v request=%+v", ok, req)
	}
	writeLaunchShortcut(t, path, target, "", "", 1)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	binary.LittleEndian.PutUint32(data[20:], binary.LittleEndian.Uint32(data[20:])|shortcutRunAsUser)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	req, ok := shortcutLaunchRequest(path, "open")
	if !ok || req.Verb != "runas" || req.Directory != "" || req.Parameters != "" {
		t.Fatalf("shortcut elevation/empty fields lost: resolved=%v request=%+v", ok, req)
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, after) {
		t.Fatal("reading a shortcut must not modify it")
	}
}

func TestShortcutLaunchRequestFallsBack(t *testing.T) {
	root := t.TempDir()
	gui := filepath.Join(os.Getenv("WINDIR"), "explorer.exe")
	for _, tc := range []struct {
		name, target, directory string
	}{
		{"missing", filepath.Join(root, "missing.exe"), ""},
		{"folder", root, ""},
		{"console", filepath.Join(os.Getenv("WINDIR"), "System32", "cmd.exe"), ""},
		{"missing_directory", gui, filepath.Join(root, "missing")},
		{"relative_directory", gui, "relative"},
		{"unexpanded_directory", gui, "%WOX_NONEXISTENT_SHORTCUT_DIR%"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name+".lnk")
			writeLaunchShortcut(t, path, tc.target, "", tc.directory, 1)
			for _, verb := range []string{"open", "runas"} {
				req, ok := shortcutLaunchRequest(path, verb)
				if ok || req.File != path || req.Verb != verb || req.Parameters != "" || req.Directory != "" {
					t.Fatalf("expected unchanged fallback, got resolved=%v request=%+v", ok, req)
				}
			}
		})
	}
	inner, outer := filepath.Join(root, "inner.lnk"), filepath.Join(root, "outer.lnk")
	writeLaunchShortcut(t, inner, gui, "inner", "", 1)
	// SetPath eagerly flattens an existing .lnk. Patch a same-length target after saving to exercise an actual chain.
	writeLaunchShortcut(t, outer, filepath.Join(root, "inner.exe"), "outer", "", 1)
	data, err := os.ReadFile(outer)
	if err != nil {
		t.Fatal(err)
	}
	data = bytes.ReplaceAll(data, []byte("inner.exe"), []byte("inner.lnk"))
	var oldName, newName bytes.Buffer
	_ = binary.Write(&oldName, binary.LittleEndian, utf16.Encode([]rune("inner.exe")))
	_ = binary.Write(&newName, binary.LittleEndian, utf16.Encode([]rune("inner.lnk")))
	data = bytes.ReplaceAll(data, oldName.Bytes(), newName.Bytes())
	if err := os.WriteFile(outer, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if req, ok := shortcutLaunchRequest(outer, "open"); ok || req.File != outer {
		t.Fatalf("nested shortcut must fall back: %+v, %v", req, ok)
	}
	for _, path := range []string{gui, "https://example.com/a.lnk", "shell:AppsFolder\\Example", filepath.Join(root, "absent.lnk"), "bad\x00.lnk"} {
		if req, ok := shortcutLaunchRequest(path, "open"); ok || req.File != path {
			t.Fatalf("expected unchanged fallback for %q", path)
		}
	}
	for _, path := range []string{`\\server\share\app.exe`, "relative.exe", `C:relative.exe`, `%UNKNOWN%\app.exe`, "shell:AppsFolder\\Example"} {
		if shortcutLocalPath(path) {
			t.Fatalf("nonlocal/relative target accepted: %q", path)
		}
	}
}

func TestShortcutLaunchRequestPreservesLongArguments(t *testing.T) {
	path := filepath.Join(t.TempDir(), "long.lnk")
	args := strings.Repeat("参数 ", 2000)
	writeLaunchShortcut(t, path, filepath.Join(os.Getenv("WINDIR"), "explorer.exe"), args, "", 1)
	if req, ok := shortcutLaunchRequest(path, "open"); !ok || req.Parameters != args {
		t.Fatalf("arguments truncated: resolved=%v length=%d want=%d", ok, len(req.Parameters), len(args))
	}
}

// minimalShortcutData provides the MS-SHLLINK header and terminal block for framing tests.
func minimalShortcutData() []byte {
	data := make([]byte, 80)
	binary.LittleEndian.PutUint32(data, 76)
	copy(data[4:], []byte{1, 20, 2, 0, 0, 0, 0, 0, 0xc0, 0, 0, 0, 0, 0, 0, 0x46})
	binary.LittleEndian.PutUint32(data[20:], 0x80)
	binary.LittleEndian.PutUint32(data[60:], 1)
	return data
}

func TestShortcutDataSupportedRejectsSpecialAndMalformedData(t *testing.T) {
	if !shortcutDataSupported(minimalShortcutData()) {
		t.Fatal("basic framing rejected")
	}
	for _, flag := range []uint32{0x1000, 0x20000, 0x400, 0x800000, 0x80000000} {
		data := minimalShortcutData()
		binary.LittleEndian.PutUint32(data[20:], 0x80|flag)
		if shortcutDataSupported(data) {
			t.Fatalf("unsafe/unknown flag 0x%x accepted", flag)
		}
	}
	for _, signature := range []uint32{0xa0000002, 0xa0000004, 0xa0000006, 0xa0000008, 0xa000000c, 0xa000ffff} {
		data := append(minimalShortcutData()[:76], make([]byte, 12)...)
		binary.LittleEndian.PutUint32(data[76:], 8)
		binary.LittleEndian.PutUint32(data[80:], signature)
		if shortcutDataSupported(data) {
			t.Fatalf("unsafe/unknown extra block 0x%x accepted", signature)
		}
	}
	for i := range 80 {
		if shortcutDataSupported(minimalShortcutData()[:i]) {
			t.Fatalf("truncated header accepted at %d", i)
		}
	}
	for _, offset := range []int{0, 4, 60, 64, 66, 68, 72, 76} {
		data := minimalShortcutData()
		binary.LittleEndian.PutUint32(data[offset:], 0xffffffff)
		if shortcutDataSupported(data) {
			t.Fatalf("invalid field accepted at %d", offset)
		}
	}
	// A property store with AppUserModel.ID must fall back even though its target could exist.
	store := make([]byte, 45)
	binary.LittleEndian.PutUint32(store, 41)
	binary.LittleEndian.PutUint32(store[4:], 0x53505331)
	copy(store[8:], []byte{0x55, 0x28, 0x4c, 0x9f, 0x79, 0x9f, 0x39, 0x4b, 0xa8, 0xd0, 0xe1, 0xd4, 0x2d, 0xe1, 0xd5, 0xf3})
	binary.LittleEndian.PutUint32(store[24:], 13)
	binary.LittleEndian.PutUint32(store[28:], 5)
	binary.LittleEndian.PutUint16(store[33:], 31)
	if shortcutMetadataSupported(store) {
		t.Fatal("AppUserModel.ID must stay with Shell")
	}
}

func TestShortcutStringRejectsTruncation(t *testing.T) {
	for _, buffer := range [][]uint16{{'a'}, {'a', 0}, {0xd800, 0, 0}} {
		if _, ok := shortcutString(buffer); ok {
			t.Fatalf("unsafe buffer accepted: %v", buffer)
		}
	}
}

func FuzzShortcutDataSupported(f *testing.F) {
	f.Add(minimalShortcutData())
	f.Add([]byte("not a shortcut"))
	f.Fuzz(func(t *testing.T, data []byte) {
		shortcutDataSupported(data)
		shortcutMetadataSupported(data)
	})
}
