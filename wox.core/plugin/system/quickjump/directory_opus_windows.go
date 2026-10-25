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

// getDirectoryOpusListerPath resolves the source tab folder of a Directory Opus lister.
// handled is false when the window is not an Opus lister, so callers can fall back to Explorer.
func getDirectoryOpusListerPath(pid int, windowID string) (path string, handled bool) {
	hwnd, dopusPath := directoryOpusListerProcessPath(pid, windowID)
	if dopusPath == "" {
		return "", false
	}

	ctx := util.NewTraceContext()
	data, err := queryDirectoryOpusPaths(ctx, filepath.Join(filepath.Dir(dopusPath), "dopusrt.exe"))
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("Directory Opus path query failed: pid=%d windowId=%q error=%s", pid, windowID, err.Error()))
		return "", true
	}

	path = selectDirectoryOpusSourcePath(data, hwnd)
	if path == "" {
		util.GetLogger().Info(ctx, fmt.Sprintf("Directory Opus lister has no filesystem source tab: pid=%d windowId=%q", pid, windowID))
		return "", true
	}
	if info, statErr := os.Stat(path); statErr != nil || !info.IsDir() {
		util.GetLogger().Info(ctx, fmt.Sprintf("Directory Opus source path is not a directory: pid=%d windowId=%q path=%q", pid, windowID, path))
		return "", true
	}
	return path, true
}

// directoryOpusListerProcessPath returns the lister HWND and dopus.exe's full path when
// windowID is a still-open Opus lister owned by pid. dopusrt.exe ships next to dopus.exe.
func directoryOpusListerProcessPath(pid int, windowID string) (uintptr, string) {
	hwndValue, err := strconv.ParseUint(strings.TrimSpace(windowID), 10, 64)
	if err != nil || hwndValue == 0 || pid <= 0 {
		return 0, ""
	}
	hwnd := windows.HWND(hwndValue)
	if !windows.IsWindow(hwnd) {
		return 0, ""
	}

	var ownerPid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &ownerPid); err != nil || int(ownerPid) != pid {
		return 0, ""
	}

	var className [64]uint16
	n, err := windows.GetClassName(hwnd, &className[0], int32(len(className)))
	if err != nil || !strings.EqualFold(windows.UTF16ToString(className[:n]), directoryOpusListerClass) {
		return 0, ""
	}

	process, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return 0, ""
	}
	defer windows.CloseHandle(process)

	var imagePath [windows.MAX_LONG_PATH]uint16
	size := uint32(len(imagePath))
	if err := windows.QueryFullProcessImageName(process, 0, &imagePath[0], &size); err != nil {
		return 0, ""
	}
	exePath := windows.UTF16ToString(imagePath[:size])
	if !strings.EqualFold(filepath.Base(exePath), "dopus.exe") {
		return 0, ""
	}
	return uintptr(hwndValue), exePath
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

	queryCtx, cancel := context.WithTimeout(ctx, directoryOpusQueryTimeout)
	defer cancel()

	cmd := exec.CommandContext(queryCtx, dopusrtPath)
	// dopusrt expects the quoted file name and the info command as one comma-joined
	// argument; Go's default argument escaping would quote the whole pair instead.
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: windows.CREATE_NO_WINDOW,
		CmdLine:       fmt.Sprintf(`"%s" /info "%s",paths`, dopusrtPath, outputPath),
	}
	if err := cmd.Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(outputPath)
}
