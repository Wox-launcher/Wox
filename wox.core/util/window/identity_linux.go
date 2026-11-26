package window

import "context"

// GetActiveWindowIdentity captures the window target before Wox takes focus.
func GetActiveWindowIdentity(ctx context.Context) (pid int, id string, name string) {
	// The clipboard watcher's cache can still name a different window of the same app.
	ctx, cancel := context.WithTimeout(ctx, linuxForegroundTimeout)
	defer cancel()
	app, err := queryLinuxForeground(ctx)
	if err != nil {
		return -1, "", ""
	}
	app = finishLinuxForeground(app)
	return app.Pid, app.WindowID, app.Name
}
