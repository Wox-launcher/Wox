//go:build darwin

package selection

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa -framework ApplicationServices
#include <stdlib.h>
#include <stdbool.h>

char* getSelectedTextA11y();
char* getSelectedFilesA11y();
bool hasAccessibilityPermissions();
void muteAlertSound();
void restoreAlertSound();
*/
import "C"
import (
	"context"
	"errors"
	"strings"
	"unsafe"
	"wox/util"
)

// GetSelected is the macOS implementation that tries A11y API first, then falls back to clipboard
func GetSelected(ctx context.Context) (Selection, error) {

	// Try accessibility API first
	// First try to get selected text
	if text, err := getSelectedTextViaA11y(ctx); err == nil && text != "" {
		util.GetLogger().Debug(ctx, "selection: Successfully got text via A11y")
		return Selection{
			Type: SelectionTypeText,
			Text: text,
		}, nil
	}

	// Then try to get selected files
	if files, err := getSelectedFilesViaA11y(ctx); err == nil && len(files) > 0 {
		util.GetLogger().Debug(ctx, "selection: Successfully got files via A11y")
		return Selection{
			Type:      SelectionTypeFile,
			FilePaths: files,
		}, nil
	}

	// Fallback to clipboard method with muted alert sound
	C.muteAlertSound()
	defer C.restoreAlertSound()

	util.GetLogger().Debug(ctx, "selection: Falling back to clipboard method")
	return getSelectedByClipboard(ctx)
}

// hasA11yPermissions checks if the application has accessibility permissions
func hasA11yPermissions() bool {
	return bool(C.hasAccessibilityPermissions())
}

// getSelectedTextViaA11y gets selected text using macOS Accessibility API
func getSelectedTextViaA11y(ctx context.Context) (string, error) {
	if !hasA11yPermissions() {
		util.GetLogger().Warn(ctx, "selection: No accessibility permissions")
		return "", errors.New("no accessibility permissions")
	}

	cstr := C.getSelectedTextA11y()
	if cstr == nil {
		util.GetLogger().Debug(ctx, "selection: Failed to get selected text via A11y")
		return "", errors.New("failed to get selected text via A11y")
	}
	defer C.free(unsafe.Pointer(cstr))

	return C.GoString(cstr), nil
}

// getSelectedFilesViaA11y gets selected files using macOS Accessibility API
func getSelectedFilesViaA11y(ctx context.Context) ([]string, error) {
	if !hasA11yPermissions() {
		util.GetLogger().Warn(ctx, "selection: No accessibility permissions")
		return nil, errors.New("no accessibility permissions")
	}

	cstr := C.getSelectedFilesA11y()
	if cstr == nil {
		util.GetLogger().Debug(ctx, "selection: Failed to get selected files via A11y")
		return nil, errors.New("failed to get selected files via A11y")
	}
	defer C.free(unsafe.Pointer(cstr))

	paths := C.GoString(cstr)
	if paths == "" {
		util.GetLogger().Debug(ctx, "selection: No files selected")
		return nil, errors.New("no files selected")
	}

	// Split the paths by newline
	return strings.Split(strings.TrimSpace(paths), "\n"), nil
}
