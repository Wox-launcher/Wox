package launcher

import (
	"context"
	"strings"
	"sync"
	"time"

	"wox/ui/contract"
)

// hotkeySettingsSnapshot is the immutable Hotkey tab state consumed by the view layer.
// It carries the inline-settings form snapshot, the hotkey-focus flag used by the general
// tab key handler, the ignored-app picker candidates, and the candidate load status.
// Recording state is intentionally omitted: it is pure runtime UI state consumed through
// dedicated accessors (Recording/ClearRecording) by hotkey_recording.go, not through snapshots.
type hotkeySettingsSnapshot struct {
	Form          *formFieldsSnapshot
	Focused       bool
	AppCandidates []ignoredHotkeyApp
	AppsLoading   bool
	AppsLoaded    bool
	AppsError     string
}

// hotkeySettingsController owns the Hotkey tab state: the inline hotkey settings form
// (main/selection hotkeys, ignored apps, query hotkeys/shortcuts, tray queries), the
// focus flag used by the general-tab key handler, the active hotkey recording state
// machine, and the ignored-app picker candidate catalog. The controller is free of any
// *App back-dependency; callers wire App-side side effects through the deps.Invalidate
// callback. Cross-domain readers (hotkeyRecordingTargetCurrentLocked, form_table helpers)
// read the live form pointer through Form().
type hotkeySettingsController struct {
	deps CommonDeps
	mu   sync.RWMutex

	// Inline form state for the hotkey settings tab. Built by the App from loaded
	// settingsData and handed to the controller via SetForm. The active flag is mutated
	// in place by the App (selectSettingTab) to reflect whether the general tab is current.
	form *formFieldsState

	// Focus flag for the hotkey form fields, toggled by the general-tab key handler.
	focused bool

	// Active hotkey recording state machine. Owned here because it is started/stopped
	// from the hotkey settings fields and from the shared table row editor, but its
	// lifecycle is managed by hotkey_recording.go through Recording/SetRecording/ClearRecording.
	// The controller's mu protects the pointer swap, not the recording state's internal
	// fields, which are mutated under the App lock by the single-threaded UI key path.
	recording *hotkeyRecordingState

	// Ignored-app picker candidates loaded once from core; feeds the app picker used by
	// the IgnoredHotkeyApps table and any app-type form fields.
	appCandidates []ignoredHotkeyApp
	appsLoading   bool
	appsLoaded    bool
	appsError     string
}

func newHotkeySettingsController(deps CommonDeps) *hotkeySettingsController {
	return &hotkeySettingsController{deps: deps}
}

// SetForm installs the hotkey settings inline form. Passing nil clears it. The form is
// built by the App from loaded settingsData and then handed to the controller so the view
// layer can read it through the snapshot. The active flag on the form is mutated in place
// by the App (selectSettingTab) to reflect whether the general tab is currently selected.
func (c *hotkeySettingsController) SetForm(form *formFieldsState) {
	c.mu.Lock()
	c.form = form
	c.mu.Unlock()
}

// Form returns the live hotkey settings form pointer. Callers compare table-editor and
// recording targets against this pointer (formTableTargetCurrentLocked,
// hotkeyRecordingTargetCurrentLocked, formTableTargetUsesSettingsLocked) and mutate it in
// place under the App lock when opening/focusing tables.
func (c *hotkeySettingsController) Form() *formFieldsState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.form
}

// Focused reports whether the hotkey form fields currently hold focus (general-tab key handler).
func (c *hotkeySettingsController) Focused() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.focused
}

// SetFocused records whether the hotkey form fields hold focus. Called by the general-tab
// key handler and selectSettingTab when entering/leaving the general tab.
func (c *hotkeySettingsController) SetFocused(focused bool) {
	c.mu.Lock()
	c.focused = focused
	c.mu.Unlock()
}

// Recording returns the active hotkey recording state, or nil when no recording is in
// flight. Callers in hotkey_recording.go read and compare the pointer under the App lock;
// the controller's mu only guards the pointer swap, not the recording state's fields.
func (c *hotkeySettingsController) Recording() *hotkeyRecordingState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.recording
}

// SetRecording installs the hotkey recording state. Used by startHotkeyRecording to publish
// a freshly created recording state, and by stopHotkeyRecording/acceptRecordedHotkey to
// clear it. The App holds its own mu around the read-modify-write sequences in
// hotkey_recording.go, so this method only needs to guard the pointer store.
func (c *hotkeySettingsController) SetRecording(state *hotkeyRecordingState) {
	c.mu.Lock()
	c.recording = state
	c.mu.Unlock()
}

// ClearRecording clears the active hotkey recording state. Equivalent to SetRecording(nil)
// but named for clarity at call sites that are ending a recording.
func (c *hotkeySettingsController) ClearRecording() {
	c.mu.Lock()
	c.recording = nil
	c.mu.Unlock()
}

// AppCandidates returns a copy of the cached ignored-app candidates. The app picker uses
// this to populate its list; mutating the returned slice does not affect the controller state.
func (c *hotkeySettingsController) AppCandidates() []ignoredHotkeyApp {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]ignoredHotkeyApp(nil), c.appCandidates...)
}

// SetAppCandidates stores the ignored-app candidates. Called after a successful reload
// (ReloadAppCandidates) with the filtered list from core.
func (c *hotkeySettingsController) SetAppCandidates(candidates []ignoredHotkeyApp) {
	c.mu.Lock()
	c.appCandidates = append([]ignoredHotkeyApp(nil), candidates...)
	c.mu.Unlock()
}

// AppsLoading reports whether an ignored-app candidate load is in flight.
func (c *hotkeySettingsController) AppsLoading() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.appsLoading
}

// SetAppsLoading records that an ignored-app candidate load is starting or ending.
func (c *hotkeySettingsController) SetAppsLoading(loading bool) {
	c.mu.Lock()
	c.appsLoading = loading
	c.mu.Unlock()
}

// AppsLoaded reports whether the ignored-app candidate catalog has been loaded at least once.
func (c *hotkeySettingsController) AppsLoaded() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.appsLoaded
}

// SetAppsLoaded records that the ignored-app candidate catalog has been loaded.
func (c *hotkeySettingsController) SetAppsLoaded(loaded bool) {
	c.mu.Lock()
	c.appsLoaded = loaded
	c.mu.Unlock()
}

// AppsError returns the last ignored-app candidate load error, if any.
func (c *hotkeySettingsController) AppsError() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.appsError
}

// SetAppsError records an ignored-app candidate load error message.
func (c *hotkeySettingsController) SetAppsError(msg string) {
	c.mu.Lock()
	c.appsError = msg
	c.mu.Unlock()
}

// ReloadAppCandidates fetches the platform-specific ignored-app identities from core and
// caches them. It is a no-op if a reload has already completed or is in flight. Mirrors the
// old App.loadHotkeyAppCandidates behavior: dedupes by lowercased identity before storing.
func (c *hotkeySettingsController) ReloadAppCandidates(ctx context.Context, service contract.HotkeySettingsServices, sessionID string) {
	c.mu.Lock()
	if c.appsLoading || c.appsLoaded {
		c.mu.Unlock()
		return
	}
	c.appsLoading = true
	c.appsError = ""
	c.mu.Unlock()
	c.deps.Invalidate()

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

	c.mu.Lock()
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
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the hotkey tab state for the view layer. Recording state is
// not included: it is pure runtime UI state read through Recording() by hotkey_recording.go
// and rendered through hotkeyRecordingFieldStatus, not through the settings snapshot.
func (c *hotkeySettingsController) Snapshot() hotkeySettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
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
