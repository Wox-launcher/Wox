//go:build windows

package ocr

/*
#cgo CXXFLAGS: -std=c++17
#cgo LDFLAGS: -static -static-libgcc -static-libstdc++ -lruntimeobject -lshcore -lstdc++
#include <stdint.h>
#include <windows.h>
#include <winstring.h>

HRESULT woxOCRRecognizeMedia(PCWSTR path, PCWSTR languages, uintptr_t cancelEvent, HSTRING *text);
*/
import "C"

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"
	"wox/util"
	"wox/util/shell"

	"golang.org/x/sys/windows"
)

type windowsHelperResponse struct {
	Engine string `json:"engine"`
	Text   string `json:"text"`
	Error  string `json:"error"`
	Code   string `json:"code"`
}

func recognizePlatform(ctx context.Context, request Request) (Result, error) {
	// The Windows App SDK AI helper is optional. Use the built-in WinRT engine
	// when the helper or its runtime is unavailable.
	if result, err := recognizeWindowsWithAIHelper(ctx, request); err == nil {
		return result, nil
	} else if !errors.Is(err, ErrUnavailable) && !errors.Is(err, ErrUnsupported) {
		util.GetLogger().Warn(ctx, "windows ai text recognizer helper failed: "+err.Error())
	}

	return recognizeWindowsWithMediaOCR(ctx, request)
}

func recognizeWindowsWithAIHelper(ctx context.Context, request Request) (Result, error) {
	helperPath := findWindowsAITextRecognizerHelper()
	if helperPath == "" {
		return Result{}, fmt.Errorf("%w: windows ai text recognizer helper is not installed", ErrUnavailable)
	}

	helperCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	cmd := shell.BuildCommandContext(helperCtx, helperPath, nil)
	input := map[string]any{
		"imagePath": request.ImagePath,
		"languages": request.Languages,
	}
	var stdin bytes.Buffer
	if err := json.NewEncoder(&stdin).Encode(input); err != nil {
		return Result{}, err
	}
	cmd.Stdin = &stdin
	output, err := cmd.Output()
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrUnavailable, err)
	}

	var response windowsHelperResponse
	if err := json.Unmarshal(output, &response); err != nil {
		return Result{}, fmt.Errorf("failed to decode windows ai text recognizer response: %w", err)
	}
	if response.Error != "" {
		return Result{}, fmt.Errorf("%w: %s", ErrUnavailable, response.Error)
	}
	if response.Engine == "" {
		response.Engine = EngineWindowsAITextRecognizer
	}
	return Result{Engine: response.Engine, Text: response.Text}, nil
}

func findWindowsAITextRecognizerHelper() string {
	// Keep helper discovery file-based rather than hard-coded to the running executable directory.
	// Embedded resources are extracted into the data directory on startup, while local development
	// builds may place the helper beside wox.exe for manual OCR verification.
	candidates := []string{
		filepath.Join(util.GetLocation().GetOthersDirectory(), "ocr", "windows", "wox-ocr-helper.exe"),
	}
	if executablePath, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(executablePath), "wox-ocr-helper.exe"))
	}
	for _, candidate := range candidates {
		if util.IsFileExists(candidate) {
			return candidate
		}
	}
	return ""
}

// recognizeWindowsWithMediaOCR runs WinRT on the calling native thread and
// signals its async waits when the request is canceled or times out.
func recognizeWindowsWithMediaOCR(ctx context.Context, request Request) (Result, error) {
	ocrCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := ocrCtx.Err(); err != nil {
		return Result{}, err
	}

	path, err := windows.UTF16PtrFromString(request.ImagePath)
	if err != nil {
		return Result{}, err
	}
	languages, err := windows.UTF16PtrFromString(strings.Join(request.Languages, "\n"))
	if err != nil {
		return Result{}, err
	}

	cancelEvent, err := windows.CreateEvent(nil, 1, 0, nil)
	if err != nil {
		return Result{}, fmt.Errorf("create Windows OCR cancellation event: %w", err)
	}
	defer windows.CloseHandle(cancelEvent)
	signaled := make(chan struct{})
	stop := context.AfterFunc(ocrCtx, func() {
		windows.SetEvent(cancelEvent)
		close(signaled)
	})
	defer func() {
		// Wait for a running callback before the event handle can be closed.
		if !stop() {
			<-signaled
		}
	}()

	var text C.HSTRING
	status := C.woxOCRRecognizeMedia((*C.WCHAR)(unsafe.Pointer(path)), (*C.WCHAR)(unsafe.Pointer(languages)),
		C.uintptr_t(cancelEvent), &text)
	defer C.WindowsDeleteString(text)
	if err := ocrCtx.Err(); err != nil {
		return Result{}, err
	}
	if status < 0 {
		return Result{}, fmt.Errorf("%w: Windows Media OCR failed: %w", ErrUnavailable, windows.Errno(uint32(status)))
	}

	var length C.UINT32
	buffer := C.WindowsGetStringRawBuffer(text, &length)
	recognized := windows.UTF16ToString(unsafe.Slice((*uint16)(unsafe.Pointer(buffer)), int(length)))
	// Multiple recognizers can return the same text. Keep the first spelling and order.
	lines := make([]string, 0)
	seen := make(map[string]struct{})
	for _, line := range strings.Split(recognized, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key := strings.ToLower(line)
		if _, exists := seen[key]; !exists {
			seen[key] = struct{}{}
			lines = append(lines, line)
		}
	}
	return Result{Engine: EngineWindowsMediaOCR, Text: strings.Join(lines, "\n")}, nil
}
