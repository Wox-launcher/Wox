package launcher

import (
	"context"
)

// reloadAboutVersion delegates to aboutSettingsController so App no longer holds about state directly.
func (a *App) reloadAboutVersion() {
	a.aboutSettings.Reload(context.Background(), a.services, a.sessionID)
}
