package quickjump

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"wox/common"
	"wox/util"
	"wox/util/window"
)

type ignoredExplorerApplication struct {
	Name     string          `json:"Name"`
	Identity string          `json:"Identity"`
	Path     string          `json:"Path"`
	Icon     common.WoxImage `json:"Icon"`
}

type ignoredExplorerApplicationRow struct {
	App ignoredExplorerApplication `json:"App"`
}

// ignoredApplicationState caches the shared app-picker table so key and
// activation paths do not re-parse JSON on every event.
type ignoredApplicationState struct {
	mu         sync.RWMutex
	rows       []ignoredExplorerApplicationRow
	failClosed bool
}

// reloadIgnoredApplications refreshes the cached ignore list from plugin settings.
func (c *QuickJumpPlugin) reloadIgnoredApplications(ctx context.Context) {
	rows, err := parseIgnoredExplorerApplications(c.api.GetSetting(ctx, ignoredApplicationsSettingKey))
	c.ignoredApps.mu.Lock()
	defer c.ignoredApps.mu.Unlock()
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("explorer: failed to parse ignored applications: %v", err))
		// A corrupted ignore list must not keep intercepting apps the user excluded.
		c.ignoredApps.rows = nil
		c.ignoredApps.failClosed = true
		return
	}
	c.ignoredApps.rows = rows
	c.ignoredApps.failClosed = false
}

// isIgnoredApplicationPid reports whether Quick Jump should leave this process alone.
func (c *QuickJumpPlugin) isIgnoredApplicationPid(pid int) bool {
	if pid <= 0 {
		return false
	}

	c.ignoredApps.mu.RLock()
	failClosed := c.ignoredApps.failClosed
	rows := c.ignoredApps.rows
	c.ignoredApps.mu.RUnlock()
	if failClosed {
		return true
	}
	if len(rows) == 0 {
		return false
	}

	identity := strings.TrimSpace(window.GetProcessIdentity(pid))
	return isIgnoredExplorerApplication(rows, identity)
}

// parseIgnoredExplorerApplications decodes rows produced by the shared application table.
func parseIgnoredExplorerApplications(value string) ([]ignoredExplorerApplicationRow, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}
	var rows []ignoredExplorerApplicationRow
	if err := json.Unmarshal([]byte(value), &rows); err != nil {
		return nil, err
	}
	return rows, nil
}

// isIgnoredExplorerApplication compares stable platform process identities.
func isIgnoredExplorerApplication(rows []ignoredExplorerApplicationRow, identity string) bool {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return false
	}
	for _, row := range rows {
		if strings.EqualFold(strings.TrimSpace(row.App.Identity), identity) {
			return true
		}
	}
	return false
}
