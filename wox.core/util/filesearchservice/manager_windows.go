//go:build windows

package filesearchservice

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"wox/util"
	"wox/util/shell"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// indexedVolumeRoots prefers the last published service snapshot, then the
// same local fixed NTFS volumes the service itself enumerates.
func indexedVolumeRoots() []string {
	if !IsRunning() {
		return nil
	}
	if cached := cachedIndexedVolumeRoots(); len(cached) > 0 {
		return cached
	}
	roots, err := fixedNTFSVolumeRoots()
	if err != nil {
		return nil
	}
	return roots
}

func embeddedExecutable() string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "file_index_service", "wox-file-index-service.exe")
}

// GetStatus reads SCM state and the version encoded in the installed binary path.
func GetStatus() (result Status) {
	result = Status{State: StateNotInstalled, EmbeddedVersion: EmbeddedVersion}
	defer func() {
		running.Store(result.State == StateRunning || result.State == StateUpdateReady)
	}()
	// Isolated test runs must not attach to the machine-wide service and its production index.
	if util.IsTestMode() {
		result.State = StateUnavailable
		return result
	}
	_, embeddedErr := os.Stat(embeddedExecutable())
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		result.State = StateUnavailable
		result.Detail = err.Error()
		return result
	}
	defer windows.CloseServiceHandle(scm)
	name, err := windows.UTF16PtrFromString(Name)
	if err != nil {
		return result
	}
	handle, err := windows.OpenService(scm, name, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		if embeddedErr != nil {
			result.State = StateUnavailable
			result.Detail = embeddedErr.Error()
		}
		return result
	}
	if err != nil {
		result.State = StateUnavailable
		result.Detail = err.Error()
		return result
	}
	s := &mgr.Service{Name: Name, Handle: handle}
	defer s.Close()
	if config, configErr := s.Config(); configErr == nil {
		result.InstalledVersion = filepath.Base(filepath.Dir(strings.Trim(config.BinaryPathName, `"`)))
	}
	serviceStatus, err := s.Query()
	if err != nil {
		result.State = StateStopped
		result.Detail = err.Error()
		return result
	}
	if serviceStatus.State == svc.Running {
		healthCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		health, healthErr := Health(healthCtx)
		cancel()
		if healthErr != nil {
			if updateAvailable(result.InstalledVersion, EmbeddedVersion) {
				result.State = StateNeedsUpdate
				result.Detail = result.InstalledVersion + " → " + EmbeddedVersion
			} else {
				result.State = StateUnhealthy
				result.Detail = healthErr.Error()
			}
			return result
		}
		result.InstalledVersion = health.Version
		result.Detail = health.Version
		if updateAvailable(result.InstalledVersion, EmbeddedVersion) {
			if util.IsDev() {
				// Unsigned development helpers cannot safely replace a SYSTEM service
				// through the non-elevated update pipe.
				result.State = StateNeedsUpdate
				result.Detail = result.InstalledVersion + " → " + EmbeddedVersion
			} else {
				result.State = StateUpdateReady
			}
		} else {
			result.State = StateRunning
		}
	} else {
		result.State = StateStopped
	}
	return result
}

// Execute performs one user-requested lifecycle action.
func Execute(ctx context.Context, action string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	executable := embeddedExecutable()
	if _, err := os.Stat(executable); err != nil {
		return fmt.Errorf("embedded file index service is unavailable: %w", err)
	}
	if action == "update" {
		hash, err := HashFile(executable)
		if err != nil {
			return err
		}
		if err := Update(ctx, executable, EmbeddedVersion, hash); err != nil {
			return err
		}
		err = waitForUpdatedService(ctx, EmbeddedVersion)
		if err == nil {
			markRunning()
		}
		return err
	}
	if action != "install" && action != "start" && action != "uninstall" {
		return fmt.Errorf("unsupported file index service action %q", action)
	}
	resultFile, err := os.CreateTemp("", "wox-file-index-service-*.result")
	if err != nil {
		return err
	}
	resultPath := resultFile.Name()
	if err := resultFile.Close(); err != nil {
		return err
	}
	defer os.Remove(resultPath)
	ownerSID, err := currentUserSID()
	if err != nil {
		return err
	}
	parameters := action + " --result " + strconv.Quote(resultPath) + " --owner-sid " + strconv.Quote(ownerSID)
	if action == "install" || action == "start" {
		indexPath := filepath.Join(util.GetLocation().GetFileSearchDirectory(), IndexDirectory)
		if err := os.MkdirAll(indexPath, 0755); err != nil {
			return err
		}
		parameters += " --index-dir " + strconv.Quote(indexPath)
	}
	wait, err := shell.RunElevated(executable, parameters, filepath.Dir(executable))
	if err != nil {
		return err
	}
	exitCode, err := wait()
	if err != nil {
		return err
	}
	if exitCode != 0 {
		if detail, readErr := os.ReadFile(resultPath); readErr == nil && len(detail) > 0 {
			return fmt.Errorf("file index service action %q failed: %s", action, strings.TrimSpace(string(detail)))
		}
		return fmt.Errorf("file index service action %q exited with code %d", action, exitCode)
	}
	if action == "uninstall" {
		markStopped()
		_ = os.RemoveAll(filepath.Join(util.GetLocation().GetFileSearchDirectory(), IndexDirectory))
	} else {
		markRunning()
	}
	return nil
}

func waitForUpdatedService(ctx context.Context, version string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		healthCtx, cancel := context.WithTimeout(ctx, time.Second)
		response, err := Health(healthCtx)
		cancel()
		if err == nil && response.Version == version {
			return nil
		}
		lastErr = err
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	return fmt.Errorf("updated file index service did not become healthy: %v", lastErr)
}
