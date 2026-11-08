package launcher

import (
	"context"
	"time"

	"wox/resource/licenses"
	"wox/ui/contract"
)

// aboutSettingsSnapshot is the immutable About tab state consumed by the view layer.
type aboutSettingsSnapshot struct {
	Version         string
	Loading         bool
	Loaded          bool
	Error           string
	LicensesOpen    bool
	SelectedLicense *licenses.Entry
}

// aboutSettingsController owns About data and license navigation.
type aboutSettingsController struct {
	deps            CommonDeps
	version         string
	loading         bool
	loaded          bool
	errMsg          string
	licensesOpen    bool
	selectedLicense *licenses.Entry
}

func newAboutSettingsController(deps CommonDeps) *aboutSettingsController {
	return &aboutSettingsController{deps: deps}
}

// Reload fetches the running core version. Mirrors the old reloadAboutVersion behavior:
// it is a no-op if a reload is already in flight, clears any prior error, and invalidates
// the view before and after the service call.
func (c *aboutSettingsController) Reload(ctx context.Context, service contract.AboutSettingsServices, sessionID string) {
	shouldLoad := false
	c.deps.OnUI("start about settings reload", func() {
		if c.loading {
			return
		}
		c.loading = true
		c.errMsg = ""
		shouldLoad = true
		c.deps.Invalidate()
	})
	if !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	version, err := service.Version(timeoutCtx, sessionID)

	c.deps.OnUI("finish about settings reload", func() {
		c.loading = false
		if err != nil {
			c.errMsg = err.Error()
		} else {
			c.version = version
			c.loaded = true
		}
		c.deps.Invalidate()
	})
}

// SetError records an error from outside the reload path (e.g. onboarding/open link failures).
func (c *aboutSettingsController) SetError(msg string) {
	c.deps.OnUI("set about settings error", func() {
		c.errMsg = msg
		c.deps.Invalidate()
	})
}

// ShowLicenses opens the catalog or returns to it from the selected license.
func (c *aboutSettingsController) ShowLicenses() {
	c.setLicensePage(true, nil)
}

func (c *aboutSettingsController) setLicensePage(open bool, entry *licenses.Entry) {
	c.deps.OnUI("navigate About licenses", func() {
		c.licensesOpen = open
		c.selectedLicense = entry
		c.deps.Invalidate()
	})
}

// Snapshot returns a copy of the About state for the view layer.
func (c *aboutSettingsController) Snapshot() aboutSettingsSnapshot {
	return aboutSettingsSnapshot{
		Version:         c.version,
		Loading:         c.loading,
		Loaded:          c.loaded,
		Error:           c.errMsg,
		LicensesOpen:    c.licensesOpen,
		SelectedLicense: c.selectedLicense,
	}
}
