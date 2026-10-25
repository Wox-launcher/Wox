package launcher

import (
	"context"
	"strings"
	"time"

	"wox/ui/contract"
	woxcomponent "wox/ui/launcher/component"
	woxwidget "wox/ui/widget"
)

// hotkeySettingsSnapshot is the immutable Hotkey tab state consumed by the view layer.
// It carries the inline-settings form snapshot, the hotkey-focus flag used by the general
// tab key handler, the ignored-app picker candidates, and the candidate load status.
type hotkeySettingsSnapshot struct {
	Form          *formFieldsSnapshot
	Focused       bool
	AppCandidates []ignoredHotkeyApp
	AppsLoading   bool
	AppsLoaded    bool
	AppsError     string
}

// hotkeySettingsController owns the Hotkey tab state: the inline hotkey settings form
// (main/selection hotkeys, ignored apps, query hotkeys), the focus flag used by the
// hotkey-tab key handler, and the ignored-app picker candidate catalog. It also builds the
// Hotkey page and the general tab's query hotkey fields; collaborators are injected by
// bindHotkeySettings. Cross-domain readers (hotkeyRecordingTargetCurrentLocked, form_table helpers)
// read the live form pointer through Form().
type hotkeySettingsController struct {
	deps CommonDeps

	services        contract.HotkeySettingsServices
	sessionID       string
	settings        *settingsWindowController
	formTables      *formTableController
	hotkeyRecording *hotkeyRecordingController
	generalSettings func() *generalSettingsController
	buildFormField  func(fields formFieldsSnapshot, callbacks formFieldCallbacks, palette woxcomponent.ControlTheme, index int, definition formDefinition, width, height float32) woxwidget.Widget

	// Inline form state for the hotkey settings tab. Built by the App from loaded
	// settingsData and handed to the controller via SetForm. The active flag is mutated
	// in place by the App (selectSettingTab) to reflect whether the general tab is current.
	form *formFieldsState

	// Focus flag for the hotkey form fields, toggled by the general-tab key handler.
	focused bool

	// Ignored-app picker candidates loaded once from core; feeds the app picker used by
	// the IgnoredHotkeyApps table and any app-type form fields.
	appCandidates []ignoredHotkeyApp
	appsLoading   bool
	appsLoaded    bool
	appsError     string
	appsRevision  uint64
}

func newHotkeySettingsController(deps CommonDeps) *hotkeySettingsController {
	return &hotkeySettingsController{deps: deps}
}

// bindHotkeySettings wires the Hotkey page to core services, the Settings window and the recorder.
func (a *App) bindHotkeySettings() {
	c := a.hotkeySettings
	if c == nil {
		return
	}
	c.services = a.services
	c.sessionID = a.sessionID
	c.settings = &a.settingsWindow
	c.formTables = &a.formTables
	c.hotkeyRecording = &a.hotkeyRecording
	c.generalSettings = func() *generalSettingsController { return a.generalSettings }
	c.buildFormField = a.buildFormField
}

// SetForm installs the hotkey settings inline form. Passing nil clears it. The form is
// built by the App from loaded settingsData and then handed to the controller so the view
// layer can read it through the snapshot. The active flag on the form is mutated in place
// by the App (selectSettingTab) to reflect whether the general tab is currently selected.
func (c *hotkeySettingsController) SetForm(form *formFieldsState) {
	c.form = form
}

// Form returns the live hotkey settings form pointer. Callers compare table-editor and
// recording targets against this pointer (formTableTargetCurrentLocked,
// hotkeyRecordingTargetCurrentLocked, formTableTargetUsesSettingsLocked) and mutate it in
// place on the UI thread when opening or focusing tables.
func (c *hotkeySettingsController) Form() *formFieldsState {
	return c.form
}

// Focused reports whether the hotkey form fields currently hold focus (general-tab key handler).
func (c *hotkeySettingsController) Focused() bool {
	return c.focused
}

// SetFocused records whether the hotkey form fields hold focus. Called by the general-tab
// key handler and selectSettingTab when entering/leaving the general tab.
func (c *hotkeySettingsController) SetFocused(focused bool) {
	c.focused = focused
}

// AppCandidates returns a copy of the cached ignored-app candidates. The app picker uses
// this to populate its list; mutating the returned slice does not affect the controller state.
func (c *hotkeySettingsController) AppCandidates() []ignoredHotkeyApp {
	return append([]ignoredHotkeyApp(nil), c.appCandidates...)
}

// SetAppCandidates stores the ignored-app candidates. Called after a successful reload
// (ReloadAppCandidates) with the filtered list from core.
func (c *hotkeySettingsController) SetAppCandidates(candidates []ignoredHotkeyApp) {
	c.appCandidates = append([]ignoredHotkeyApp(nil), candidates...)
}

// AppsLoading reports whether an ignored-app candidate load is in flight.
func (c *hotkeySettingsController) AppsLoading() bool {
	return c.appsLoading
}

// SetAppsLoading records that an ignored-app candidate load is starting or ending.
func (c *hotkeySettingsController) SetAppsLoading(loading bool) {
	c.appsLoading = loading
}

// AppsLoaded reports whether the ignored-app candidate catalog has been loaded at least once.
func (c *hotkeySettingsController) AppsLoaded() bool {
	return c.appsLoaded
}

// SetAppsLoaded records that the ignored-app candidate catalog has been loaded.
func (c *hotkeySettingsController) SetAppsLoaded(loaded bool) {
	c.appsLoaded = loaded
}

// AppsError returns the last ignored-app candidate load error, if any.
func (c *hotkeySettingsController) AppsError() string {
	return c.appsError
}

// SetAppsError records an ignored-app candidate load error message.
func (c *hotkeySettingsController) SetAppsError(msg string) {
	c.appsError = msg
}

// ReleaseWindowMemory drops settings-only form and application-picker state.
func (c *hotkeySettingsController) ReleaseWindowMemory() {
	c.form = nil
	c.focused = false
	c.appCandidates = nil
	c.appsLoading = false
	c.appsLoaded = false
	c.appsError = ""
	c.appsRevision++
}

// ReloadAppCandidates fetches the platform-specific ignored-app identities from core and
// caches them. It is a no-op if a reload has already completed or is in flight. Mirrors the
// old App.loadHotkeyAppCandidates behavior: dedupes by lowercased identity before storing.
func (c *hotkeySettingsController) ReloadAppCandidates(ctx context.Context, service contract.HotkeySettingsServices, sessionID string) {
	shouldLoad := false
	var revision uint64
	if !c.deps.OnUI("start loading hotkey app candidates", func() {
		if c.appsLoading || c.appsLoaded {
			return
		}
		c.appsRevision++
		revision = c.appsRevision
		c.appsLoading = true
		c.appsError = ""
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	loaded, err := service.HotkeyAppCandidates(timeoutCtx, sessionID)
	cancel()
	apps := make([]ignoredHotkeyApp, len(loaded))
	for index, app := range loaded {
		apps[index] = ignoredHotkeyApp{
			Name: app.Name, Identity: app.Identity, Path: app.Path,
			Icon: woxImage{ImageType: app.Icon.ImageType, ImageData: app.Icon.ImageData},
		}
	}

	c.deps.OnUI("apply hotkey app candidates", func() {
		if revision != c.appsRevision {
			return
		}
		c.appsLoading = false
		if err != nil {
			c.appsError = err.Error()
		} else {
			seen := make(map[string]bool, len(apps))
			filtered := make([]ignoredHotkeyApp, 0, len(apps))
			for _, app := range apps {
				identity := strings.ToLower(strings.TrimSpace(app.Identity))
				if identity == "" || seen[identity] {
					continue
				}
				seen[identity] = true
				filtered = append(filtered, app)
			}
			c.appCandidates = filtered
			c.appsLoaded = true
			c.appsError = ""
		}
		c.deps.Invalidate()
	})
}

// Snapshot returns a copy of the hotkey tab state for the view layer.
func (c *hotkeySettingsController) Snapshot() hotkeySettingsSnapshot {
	var form *formFieldsSnapshot
	if c.form != nil {
		snapshot := snapshotFormFieldsLocked(c.form)
		form = &snapshot
	}
	return hotkeySettingsSnapshot{
		Form:          form,
		Focused:       c.focused,
		AppCandidates: append([]ignoredHotkeyApp(nil), c.appCandidates...),
		AppsLoading:   c.appsLoading,
		AppsLoaded:    c.appsLoaded,
		AppsError:     c.appsError,
	}
}
