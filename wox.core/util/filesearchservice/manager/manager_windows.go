//go:build windows

package manager

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
	"wox/util/filesearchservice"
	"wox/util/shell"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

func embeddedExecutable() string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "file_index_service", "wox-file-index-service.exe")
}

// GetStatus reads SCM state and the version encoded in the installed binary path.
func GetStatus() (result filesearchservice.Status) {
	result = filesearchservice.Status{State: filesearchservice.StateNotInstalled, EmbeddedVersion: filesearchservice.EmbeddedVersion}
	defer func() {
		filesearchservice.SetRunning(result.State == filesearchservice.StateRunning || result.State == filesearchservice.StateUpdateReady)
	}()
	// Isolated test runs must not attach to the machine-wide service and its production index.
	if util.IsTestMode() {
		result.State = filesearchservice.StateUnavailable
		return result
	}
	_, embeddedErr := os.Stat(embeddedExecutable())
	scm, err := windows.OpenSCManager(nil, nil, windows.SC_MANAGER_CONNECT)
	if err != nil {
		result.State = filesearchservice.StateUnavailable
		result.Detail = err.Error()
		return result
	}
	defer windows.CloseServiceHandle(scm)
	name, err := windows.UTF16PtrFromString(filesearchservice.Name)
	if err != nil {
		return result
	}
	handle, err := windows.OpenService(scm, name, windows.SERVICE_QUERY_CONFIG|windows.SERVICE_QUERY_STATUS)
	if errors.Is(err, windows.ERROR_SERVICE_DOES_NOT_EXIST) {
		if embeddedErr != nil {
			result.State = filesearchservice.StateUnavailable
			result.Detail = embeddedErr.Error()
		}
		return result
	}
	if err != nil {
		result.State = filesearchservice.StateUnavailable
		result.Detail = err.Error()
		return result
	}
	s := &mgr.Service{Name: filesearchservice.Name, Handle: handle}
	defer s.Close()
	if config, configErr := s.Config(); configErr == nil {
		result.InstalledVersion = filepath.Base(filepath.Dir(strings.Trim(config.BinaryPathName, `"`)))
	}
	serviceStatus, err := s.Query()
	if err != nil {
		result.State = filesearchservice.StateStopped
		result.Detail = err.Error()
		return result
	}
	if serviceStatus.State == svc.Running {
		healthCtx, cancel := context.WithTimeout(context.Background(), time.Second)
		health, healthErr := filesearchservice.Health(healthCtx)
		cancel()
		if healthErr != nil {
			if errors.Is(healthErr, filesearchservice.ErrProtocolMismatch) || filesearchservice.UpdateAvailable(result.InstalledVersion, filesearchservice.EmbeddedVersion) {
				result.State = filesearchservice.StateNeedsUpdate
				result.Detail = result.InstalledVersion + " → " + filesearchservice.EmbeddedVersion
			} else {
				result.State = filesearchservice.StateUnhealthy
				result.Detail = healthErr.Error()
			}
			return result
		}
		result.InstalledVersion = health.Version
		result.Detail = health.Version
		if filesearchservice.UpdateAvailable(result.InstalledVersion, filesearchservice.EmbeddedVersion) {
			if util.IsDev() {
				// Unsigned development helpers cannot safely replace a SYSTEM service
				// through the non-elevated update pipe.
				result.State = filesearchservice.StateNeedsUpdate
				result.Detail = result.InstalledVersion + " → " + filesearchservice.EmbeddedVersion
			} else {
				result.State = filesearchservice.StateUpdateReady
			}
		} else {
			result.State = filesearchservice.StateRunning
		}
	} else {
		result.State = filesearchservice.StateStopped
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
		hash, err := filesearchservice.HashFile(executable)
		if err != nil {
			return err
		}
		if err := filesearchservice.Update(ctx, executable, filesearchservice.EmbeddedVersion, hash); err != nil {
			return err
		}
		err = waitForHealthyService(ctx, filesearchservice.EmbeddedVersion)
		if err == nil {
			filesearchservice.SetRunning(true)
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
	ownerSID, err := filesearchservice.CurrentUserSID()
	if err != nil {
		return err
	}
	parameters := action + " --result " + strconv.Quote(resultPath) + " --owner-sid " + strconv.Quote(ownerSID)
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
		filesearchservice.SetRunning(false)
	} else {
		if err := waitForHealthyService(ctx, filesearchservice.EmbeddedVersion); err != nil {
			return err
		}
		filesearchservice.SetRunning(true)
	}
	return nil
}

// waitForHealthyService waits for IPC readiness, independently of index build progress.
func waitForHealthyService(ctx context.Context, version string) error {
	deadline := time.Now().Add(30 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		healthCtx, cancel := context.WithTimeout(ctx, time.Second)
		response, err := filesearchservice.Health(healthCtx)
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
	return fmt.Errorf("file index service did not become healthy: %v", lastErr)
}
