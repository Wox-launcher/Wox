//go:build windows

package quickjump

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"
	"wox/util"

	"golang.org/x/sys/windows"
)

const (
	directoryOpusListerClass  = "dopus.lister"
	directoryOpusQueryTimeout = 2 * time.Second
)

// findDirectoryOpusLister returns the lister when windowID is a still-open Opus lister owned
// by pid. ok is false for any other window, so callers can fall back to Explorer handling.
func findDirectoryOpusLister(pid int, windowID string) (lister directoryOpusLister, ok bool) {
	hwndValue, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || hwndValue == 0 || pid <= 0 {
		return directoryOpusLister{}, false
	}
	hwnd := windows.HWND(hwndValue)
	if !windows.IsWindow(hwnd) {
		return directoryOpusLister{}, false
	}

	var ownerPid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &ownerPid); err != nil || int(ownerPid) != pid {
		return directoryOpusLister{}, false
	}

	var className [64]uint16
	n, err := windows.GetClassName(hwnd, &className[0], int32(len(className)))
	if err != nil || !strings.EqualFold(windows.UTF16ToString(className[:n]), directoryOpusListerClass) {
		return directoryOpusLister{}, false
	}

	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return directoryOpusLister{}, false
	}
	defer windows.CloseHandle(process)

	var imagePath [windows.MAX_LONG_PATH]uint16
	size := uint32(len(imagePath))
	if err := windows.QueryFullProcessImageName(process, 0, &imagePath[0], &size); err != nil {
		return directoryOpusLister{}, false
	}
	exePath := windows.UTF16ToString(imagePath[:size])
	if !strings.EqualFold(filepath.Base(exePath), "dopus.exe") {
		return directoryOpusLister{}, false
	}
	return directoryOpusLister{
		pid:  pid,
		hwnd: uintptr(hwndValue),
		// dopusrt.exe always ships next to dopus.exe.
		dopusrtPath: filepath.Join(filepath.Dir(exePath), "dopusrt.exe"),
	}, true
}

// sourcePath returns the real folder of the lister's source tab, or "" when it is a virtual
// folder such as "This PC" or dopusrt fails.
func (l directoryOpusLister) sourcePath(ctx context.Context) string {
	data, err := queryDirectoryOpusPaths(ctx, l.dopusrtPath)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("Directory Opus path query failed: pid=%d hwnd=%d error=%s", l.pid, l.hwnd, err.Error()))
		return ""
	}

	path := selectDirectoryOpusSourcePath(data, l.hwnd)
	if path == "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("Directory Opus lister has no filesystem source tab: pid=%d hwnd=%d", l.pid, l.hwnd))
		return ""
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		util.GetLogger().Info(ctx, fmt.Sprintf("Directory Opus source path is not a directory: pid=%d hwnd=%d path=%q", l.pid, l.hwnd, path))
		return ""
	}
	return path
}

// goTo runs Opus' Go command on targetPath. A folder path opens that folder; a file path reads
// its parent folder and selects the file, so one command covers "jump to folder" and "reveal file".
func (l directoryOpusLister) goTo(ctx context.Context, targetPath string) error {
	// /acmd runs in the active lister. Wox's launcher is not a lister, so Opus keeps the
	// lister that was focused before type-to-search as its active one.
	return runDirectoryOpusRT(ctx, l.dopusrtPath, buildDirectoryOpusGoArgs(targetPath))
}

// queryDirectoryOpusPaths runs `dopusrt /info <file>,paths`, the documented way for external
// programs to read Opus tab paths. dopusrt writes the XML before it exits (~20ms when warm).
func queryDirectoryOpusPaths(ctx context.Context, dopusrtPath string) ([]byte, error) {
	outputFile, err := os.CreateTemp("", "wox-dopus-paths-*.xml")
	if err != nil {
		return nil, err
	}
	outputPath := outputFile.Name()
	outputFile.Close()
	defer os.Remove(outputPath)

	if err := runDirectoryOpusRT(ctx, dopusrtPath, fmt.Sprintf(`/info "%s",paths`, outputPath)); err != nil {
		return nil, err
	}
	return os.ReadFile(outputPath)
}

// runDirectoryOpusRT runs dopusrt with a raw argument string and waits for it to exit.
func runDirectoryOpusRT(ctx context.Context, dopusrtPath string, args string) error {
	queryCtx, cancel := context.WithTimeout(ctx, directoryOpusQueryTimeout)
	defer cancel()

	cmd := exec.CommandContext(queryCtx, dopusrtPath)
	// dopusrt parses its own command line (e.g. a quoted file name and the info command as one
	// comma-joined argument); Go's default argument escaping would quote such pairs as a whole.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
		CmdLine:       fmt.Sprintf(`"%s" %s`, dopusrtPath, args),
	}
	return cmd.Run()
}
