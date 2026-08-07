package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"

	"wox/util"
)

const (
	oleSFalse       = 0x00000001
	rpcEChangedMode = 0x80010106
)

var (
	shortcutNativeSuccessCount atomic.Int64
	shortcutFallbackCount      atomic.Int64
	shortcutTargets            = newShortcutTargetResolver()
)

type shortcutTargetCacheEntry struct {
	targetPath       string
	modifiedUnixNano int64
	size             int64
}

type shortcutTargetResolver struct {
	mutex   sync.RWMutex
	entries map[string]shortcutTargetCacheEntry
}

func newShortcutTargetResolver() *shortcutTargetResolver {
	return &shortcutTargetResolver{entries: make(map[string]shortcutTargetCacheEntry)}
}

// resolveShortcutTarget reuses the target until the shortcut file changes.
func resolveShortcutTarget(ctx context.Context, shortcutPath string) (string, error) {
	return shortcutTargets.resolve(ctx, shortcutPath, resolveShortcutTargetUncached)
}

// resolve avoids recreating WScript.Shell during the per-second running-app refresh.
func (r *shortcutTargetResolver) resolve(ctx context.Context, shortcutPath string, load func(context.Context, string) (string, error)) (string, error) {
	fileInfo, statErr := os.Stat(shortcutPath)
	if statErr != nil {
		return load(ctx, shortcutPath)
	}

	cacheKey := strings.ToLower(filepath.Clean(shortcutPath))
	r.mutex.RLock()
	entry, cached := r.entries[cacheKey]
	r.mutex.RUnlock()
	if cached && entry.modifiedUnixNano == fileInfo.ModTime().UnixNano() && entry.size == fileInfo.Size() {
		return entry.targetPath, nil
	}

	targetPath, err := load(ctx, shortcutPath)
	if err != nil {
		return "", err
	}
	r.mutex.Lock()
	r.entries[cacheKey] = shortcutTargetCacheEntry{
		targetPath:       targetPath,
		modifiedUnixNano: fileInfo.ModTime().UnixNano(),
		size:             fileInfo.Size(),
	}
	r.mutex.Unlock()
	return targetPath, nil
}

// resolveShortcutTargetUncached resolves a Windows shortcut using in-process COM APIs.
func resolveShortcutTargetUncached(ctx context.Context, shortcutPath string) (string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	initialized := false
	if err := ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED); err != nil {
		if oleErr, ok := err.(*ole.OleError); ok {
			switch oleErr.Code() {
			case ole.S_OK, oleSFalse:
				initialized = true
			case rpcEChangedMode:
				// COM already initialized with different concurrency model; continue without reinitializing.
			default:
				return "", fmt.Errorf("CoInitializeEx failed: %w", err)
			}
		} else {
			return "", fmt.Errorf("CoInitializeEx failed: %w", err)
		}
	} else {
		initialized = true
	}
	if initialized {
		defer ole.CoUninitialize()
	}

	unknown, err := oleutil.CreateObject("WScript.Shell")
	if err != nil {
		return "", fmt.Errorf("create WScript.Shell COM object: %w", err)
	}
	defer unknown.Release()

	shellDispatch, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return "", fmt.Errorf("query IDispatch from WScript.Shell: %w", err)
	}
	defer shellDispatch.Release()

	shortcutVariant, err := oleutil.CallMethod(shellDispatch, "CreateShortcut", shortcutPath)
	if err != nil {
		return "", fmt.Errorf("CreateShortcut call failed: %w", err)
	}
	defer shortcutVariant.Clear()

	shortcutDispatch := shortcutVariant.ToIDispatch()
	if shortcutDispatch == nil {
		return "", fmt.Errorf("shortcut IDispatch is nil")
	}

	// Attempt to resolve the shortcut in-place; ignore failures because not all shortcuts require it.
	if _, callErr := oleutil.CallMethod(shortcutDispatch, "Resolve", 0); callErr != nil {
		// Some shortcuts may not implement Resolve; treat as non-fatal.
	}

	targetVariant, err := oleutil.GetProperty(shortcutDispatch, "TargetPath")
	if err != nil {
		return "", fmt.Errorf("read TargetPath property failed: %w", err)
	}
	defer targetVariant.Clear()

	targetPath := strings.TrimSpace(targetVariant.ToString())
	if targetPath == "" {
		return "", fmt.Errorf("shortcut target is empty")
	}

	cleanPath := filepath.Clean(targetPath)
	recordShortcutResolution(ctx, "native", shortcutPath)

	// Normalize path separators for downstream consumers.
	return cleanPath, nil
}

func recordShortcutResolution(ctx context.Context, mode string, shortcutPath string) {
	var native, fallback int64
	switch mode {
	case "native":
		native = shortcutNativeSuccessCount.Add(1)
		fallback = shortcutFallbackCount.Load()
	case "fallback":
		fallback = shortcutFallbackCount.Add(1)
		native = shortcutNativeSuccessCount.Load()
	default:
		native = shortcutNativeSuccessCount.Load()
		fallback = shortcutFallbackCount.Load()
	}

	total := native + fallback

	// Always log fallbacks. For native successes, sample the log to avoid noise.
	if mode == "fallback" || total <= 10 || native%100 == 0 {
		util.GetLogger().Debug(ctx, fmt.Sprintf(
			"shortcut resolution stats: total=%d native=%d fallback=%d mode=%s path=%s",
			total, native, fallback, mode, shortcutPath,
		))
	}
}
