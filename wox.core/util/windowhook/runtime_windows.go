//go:build windows

package windowhook

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"wox/util"

	"golang.org/x/sys/windows"
)

const (
	runtimeName   = "window_hook"
	hookDLL64     = "WoxWindowHook64.dll"
	hookDLL32     = "WoxWindowHook32.dll"
	hookHelper32  = "wox-window-hook-helper32.exe"
	retiredPrefix = ".retired-"
)

type hookRuntime struct {
	directory string
	// The open handle denies directory renames until the Wox process exits.
	lease windows.Handle
}

var activeRuntime atomic.Pointer[hookRuntime]

// PrepareRuntime installs the Windows hook bundle and returns the filter that
// leaves legacy hook files out of the general resource extraction.
func PrepareRuntime(assets fs.FS) (func(string) bool, error) {
	bundle, err := installRuntime(hookRuntimeRoot(), assets)
	if err != nil {
		return nil, err
	}
	if bundle != nil && !activeRuntime.CompareAndSwap(nil, bundle) {
		_ = windows.CloseHandle(bundle.lease)
	}
	return isLegacyResource, nil
}

func hookRuntimeRoot() string {
	return filepath.Join(util.GetLocation().GetRuntimeDirectory(), runtimeName)
}

func isLegacyResource(name string) bool {
	name = strings.ToLower(name)
	return name == runtimeName || strings.HasPrefix(name, runtimeName+".stale-")
}

func runtimeFilePath(name string) string {
	if bundle := activeRuntime.Load(); bundle != nil {
		return filepath.Join(bundle.directory, name)
	}
	return ""
}

// installRuntime hashes the DLLs and helper together so each process uses one
// matched bundle, even when Wox builds share a data directory.
func installRuntime(root string, assets fs.FS) (*hookRuntime, error) {
	type runtimeAsset struct {
		name string
		data []byte
	}
	var files []runtimeAsset
	hash := sha256.New()
	for _, name := range []string{hookDLL64, hookDLL32, hookHelper32} {
		data, err := fs.ReadFile(assets, "others/"+runtimeName+"/"+name)
		if errors.Is(err, fs.ErrNotExist) {
			// Development builds can omit native artifacts.
			continue
		}
		if err != nil {
			return nil, err
		}
		_, _ = fmt.Fprintf(hash, "%s:%x\n", name, sha256.Sum256(data))
		files = append(files, runtimeAsset{name, data})
	}
	if len(files) == 0 {
		return nil, nil
	}

	directory := filepath.Join(root, hex.EncodeToString(hash.Sum(nil)))
	lease, err := leaseRuntime(directory)
	if err != nil {
		return nil, err
	}
	for _, file := range files {
		if err := installRuntimeFile(filepath.Join(directory, file.name), file.data); err != nil {
			_ = windows.CloseHandle(lease)
			return nil, err
		}
	}
	return &hookRuntime{directory: directory, lease: lease}, nil
}

// leaseRuntime retries if cleanup retires the directory between creating it
// and opening the handle that protects it.
func leaseRuntime(directory string) (windows.Handle, error) {
	path, err := windows.UTF16PtrFromString(directory)
	if err != nil {
		return 0, err
	}
	for attempt := 0; attempt < 3; attempt++ {
		if err := os.MkdirAll(directory, 0755); err != nil {
			return 0, err
		}
		handle, openErr := windows.CreateFile(path, windows.FILE_LIST_DIRECTORY, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
		if openErr == nil {
			return handle, nil
		}
		err = openErr
		if !os.IsNotExist(err) && !errors.Is(err, windows.ERROR_SHARING_VIOLATION) {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	return 0, &os.PathError{Op: "open window hook runtime", Path: directory, Err: err}
}

// installRuntimeFile publishes a complete file so concurrent starts can reuse
// it safely. A matching DLL can be reused while Windows has it mapped.
func installRuntimeFile(path string, data []byte) error {
	existing, err := os.ReadFile(path)
	if err == nil && bytes.Equal(existing, data) {
		return nil
	}
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	stage, err := os.CreateTemp(filepath.Dir(path), ".install-")
	if err != nil {
		return err
	}
	defer os.Remove(stage.Name())
	_, writeErr := stage.Write(data)
	closeErr := stage.Close()
	if writeErr != nil || closeErr != nil {
		return errors.Join(writeErr, closeErr)
	}
	if err := os.Rename(stage.Name(), path); err != nil {
		// Another process may have installed and already loaded the same file.
		if existing, readErr := os.ReadFile(path); readErr != nil || !bytes.Equal(existing, data) {
			return err
		}
	}
	return nil
}

// CleanupRuntime retires other bundles and legacy copies after Wox is ready.
// Failures are retried at the next startup.
func CleanupRuntime(ctx context.Context) {
	current := ""
	if bundle := activeRuntime.Load(); bundle != nil {
		current = bundle.directory
	}
	for _, err := range cleanupRuntime(hookRuntimeRoot(), util.GetLocation().GetOthersDirectory(), current) {
		util.GetLogger().Debug(ctx, fmt.Sprintf("window hook cleanup deferred: %v", err))
	}
}

// cleanupRuntime moves stale bundles out of circulation before deleting files.
// If a Wox process holds a lease, the rename fails before any files are removed.
func cleanupRuntime(root, legacyRoot, current string) []error {
	var failures []error
	entries, err := os.ReadDir(root)
	if err != nil && !os.IsNotExist(err) {
		failures = append(failures, err)
	}
	for _, entry := range entries {
		directory := filepath.Join(root, entry.Name())
		if !entry.IsDir() || directory == current {
			continue
		}
		if strings.HasPrefix(entry.Name(), retiredPrefix) {
			if err := os.RemoveAll(directory); err != nil {
				failures = append(failures, err)
			}
		} else if len(entry.Name()) == sha256.Size*2 {
			if _, err := hex.DecodeString(entry.Name()); err == nil {
				if err := retireRuntime(root, directory); err != nil {
					failures = append(failures, err)
				}
			}
		}
	}

	legacy, err := os.ReadDir(legacyRoot)
	if err != nil && !os.IsNotExist(err) {
		failures = append(failures, err)
	}
	for _, entry := range legacy {
		if entry.IsDir() && isLegacyResource(entry.Name()) {
			if err := retireRuntime(root, filepath.Join(legacyRoot, entry.Name())); err != nil {
				failures = append(failures, err)
			}
		}
	}
	return failures
}

// retireRuntime gives a detached directory its own path so the original hash
// can be reinstalled while a DLL in the retired copy is still mapped.
func retireRuntime(root, directory string) error {
	if err := os.MkdirAll(root, 0755); err != nil {
		return err
	}
	retired, err := os.MkdirTemp(root, retiredPrefix)
	if err != nil {
		return err
	}
	if err := os.Rename(directory, filepath.Join(retired, runtimeName)); err != nil {
		_ = os.Remove(retired)
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	return os.RemoveAll(retired)
}
