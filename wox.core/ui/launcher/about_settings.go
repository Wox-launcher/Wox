package launcher

import (
	"context"
	"time"
)

// reloadAboutVersion reads the running core version instead of duplicating build metadata in the UI binary.
func (a *App) reloadAboutVersion() {
	a.mu.Lock()
	if a.aboutLoading {
		a.mu.Unlock()
		return
	}
	a.aboutLoading = true
	a.aboutError = ""
	a.mu.Unlock()
	a.invalidateSettingsWindow()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var version string
	err := a.client.Post(ctx, "/version", map[string]any{}, &version)

	a.mu.Lock()
	a.aboutLoading = false
	if err != nil {
		a.aboutError = err.Error()
	} else {
		a.aboutVersion = version
		a.aboutLoaded = true
	}
	a.mu.Unlock()
	a.invalidateSettingsWindow()
}
