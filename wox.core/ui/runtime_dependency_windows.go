package ui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"unsafe"
	"wox/i18n"
	"wox/util"
	"wox/util/shell"

	"golang.org/x/sys/windows"
)

const (
	windowsVCRedistDownloadURL = "https://aka.ms/vs/17/release/vc_redist.x64.exe"
	statusDLLNotFoundExitCode  = uint32(0xC0000135)

	messageBoxYesNoIconError = 0x00000004 | 0x00000010
	messageBoxResultYes      = 6
)

var (
	requiredWindowsUIRuntimeDLLs = []string{
		"MSVCP140.dll",
		"VCRUNTIME140.dll",
		"VCRUNTIME140_1.dll",
	}

	user32ProcMessageBoxW = windows.NewLazySystemDLL("user32.dll").NewProc("MessageBoxW")
)

func ensureUIRuntimeDependencies(ctx context.Context, appPath string) error {
	if util.IsGoUIImplementation() {
		return nil
	}
	missingDLLs := findMissingWindowsUIRuntimeDLLs(appPath)
	if len(missingDLLs) == 0 {
		return nil
	}

	missingLogTemplate := i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_missing_log")
	err := errors.New(fmt.Sprintf(missingLogTemplate, strings.Join(missingDLLs, ", "), windowsVCRedistDownloadURL))
	util.GetLogger().Error(ctx, err.Error())
	showWindowsUIRuntimePrompt(ctx, missingDLLs, false)
	return err
}

func handleUIRuntimeLaunchFailure(ctx context.Context, waitErr error) {
	if !isStatusDLLNotFoundExit(waitErr) {
		return
	}

	// Bug fix: the preflight list covers the known Flutter MSVC runtime DLLs, but
	// Windows can still reject the UI process if a future native plugin adds a new
	// dependency. Keep this launch-result check so STATUS_DLL_NOT_FOUND remains an
	// actionable redistributable prompt instead of an unexplained UI exit.
	statusCode := fmt.Sprintf("0x%X", statusDLLNotFoundExitCode)
	statusLogTemplate := i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_status_log")
	util.GetLogger().Error(ctx, fmt.Sprintf(statusLogTemplate, statusCode, windowsVCRedistDownloadURL))
	showWindowsUIRuntimePrompt(ctx, nil, true)
}

func findMissingWindowsUIRuntimeDLLs(appPath string) []string {
	searchDirs := getWindowsUIRuntimeSearchDirs(appPath)
	var missing []string
	for _, dll := range requiredWindowsUIRuntimeDLLs {
		if !windowsUIRuntimeDLLExists(dll, searchDirs) {
			missing = append(missing, dll)
		}
	}
	return missing
}

func getWindowsUIRuntimeSearchDirs(appPath string) []string {
	var dirs []string
	if appPath != "" {
		dirs = append(dirs, filepath.Dir(appPath))
	}

	for _, root := range []string{os.Getenv("SystemRoot"), os.Getenv("WINDIR")} {
		if root == "" {
			continue
		}
		dirs = append(dirs, filepath.Join(root, "System32"))
	}

	dirs = append(dirs, filepath.SplitList(os.Getenv("PATH"))...)
	return uniqueWindowsUIRuntimeSearchDirs(dirs)
}

func windowsUIRuntimeDLLExists(dll string, searchDirs []string) bool {
	for _, dir := range searchDirs {
		if util.IsFileExists(filepath.Join(dir, dll)) {
			return true
		}
	}
	return false
}

func uniqueWindowsUIRuntimeSearchDirs(dirs []string) []string {
	seen := map[string]struct{}{}
	var result []string
	for _, dir := range dirs {
		normalized := strings.TrimSpace(dir)
		if normalized == "" {
			continue
		}

		cleaned := filepath.Clean(normalized)
		key := strings.ToLower(cleaned)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, cleaned)
	}
	return result
}

func isStatusDLLNotFoundExit(err error) bool {
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ProcessState == nil {
		return false
	}
	return uint32(exitErr.ProcessState.ExitCode()) == statusDLLNotFoundExitCode
}

func showWindowsUIRuntimePrompt(ctx context.Context, missingDLLs []string, launchFailed bool) {
	message := buildWindowsUIRuntimePromptMessage(ctx, missingDLLs, launchFailed)
	result := showWindowsMessageBox(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_missing_title"), message)
	if result != messageBoxResultYes {
		return
	}

	if openErr := shell.Open(windowsVCRedistDownloadURL); openErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to open Visual C++ Redistributable download url: %s", openErr.Error()))
	}
}

func buildWindowsUIRuntimePromptMessage(ctx context.Context, missingDLLs []string, launchFailed bool) string {
	var builder strings.Builder
	// Bug fix follow-up: this prompt appears before Flutter can render any UI,
	// so every visible string is translated in Go using the user's current Wox
	// language instead of hard-coded English.
	builder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_missing_intro"))
	builder.WriteString("\n\n")
	if len(missingDLLs) > 0 {
		builder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_missing_dlls"))
		builder.WriteByte('\n')
		for _, dll := range missingDLLs {
			builder.WriteString("- ")
			builder.WriteString(dll)
			builder.WriteByte('\n')
		}
		builder.WriteByte('\n')
	}
	if launchFailed {
		statusCode := fmt.Sprintf("0x%X", statusDLLNotFoundExitCode)
		statusPromptTemplate := i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_status_dll_not_found")
		builder.WriteString(fmt.Sprintf(statusPromptTemplate, statusCode))
		builder.WriteString("\n\n")
	}
	builder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_install"))
	builder.WriteString("\n\n")
	builder.WriteString(i18n.GetI18nManager().TranslateWox(ctx, "ui_runtime_windows_vc_redist_open_download"))
	builder.WriteByte('\n')
	builder.WriteString(windowsVCRedistDownloadURL)
	return builder.String()
}

func showWindowsMessageBox(title string, message string) uintptr {
	titlePtr := windows.StringToUTF16Ptr(title)
	messagePtr := windows.StringToUTF16Ptr(message)
	result, _, _ := user32ProcMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(messagePtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(messageBoxYesNoIconError),
	)
	return result
}
