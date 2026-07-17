package launcher

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	"wox/ui/contract"
	woxui "wox/ui/runtime"
)

type appInstanceRegistry struct {
	mu sync.Mutex

	primary       *App
	bySessionID   map[string]*App
	sessionByName map[string]string
}

// newAppInstanceRegistry creates the process-local launcher session index.
func newAppInstanceRegistry() *appInstanceRegistry {
	return &appInstanceRegistry{bySessionID: map[string]*App{}, sessionByName: map[string]string{}}
}

func (r *appInstanceRegistry) registerPrimary(app *App) {
	if r == nil || app == nil {
		return
	}
	r.mu.Lock()
	r.primary = app
	r.bySessionID[app.sessionID] = app
	r.mu.Unlock()
}

// open creates or reuses a named secondary session and applies its query before presentation.
func (r *appInstanceRegistry) open(ctx context.Context, options contract.OpenInstanceOptions) error {
	if r == nil {
		return errors.New("launcher instance registry is not initialized")
	}
	if options.Role != string(woxInstanceRoleSecondary) {
		r.mu.Lock()
		primary := r.primary
		r.mu.Unlock()
		if primary == nil {
			return errors.New("primary launcher instance is unavailable")
		}
		return openAppInstance(ctx, primary, options)
	}

	r.mu.Lock()
	if sessionID := r.sessionByName[options.InstanceName]; options.InstanceName != "" && sessionID != "" {
		if existing := r.bySessionID[sessionID]; existing != nil && existing.isLive() {
			r.mu.Unlock()
			return openAppInstance(ctx, existing, options)
		}
		delete(r.sessionByName, options.InstanceName)
		delete(r.bySessionID, sessionID)
	}
	primary := r.primary
	if primary == nil {
		r.mu.Unlock()
		return errors.New("primary launcher instance is unavailable")
	}
	secondary := newApp(primary.isDev, primary.services, primary.clientFactory, primary.windows, r, primary, false, options.InstanceName, "")
	if options.Show.WindowWidth > 0 {
		secondary.show.WindowWidth = options.Show.WindowWidth
	}
	if err := secondary.start(); err != nil {
		r.mu.Unlock()
		_ = secondary.Close()
		return err
	}
	r.bySessionID[secondary.sessionID] = secondary
	if options.InstanceName != "" {
		r.sessionByName[options.InstanceName] = secondary.sessionID
	}
	r.mu.Unlock()
	if err := openAppInstance(ctx, secondary, options); err != nil {
		_ = secondary.Close()
		return err
	}
	return nil
}

func openAppInstance(ctx context.Context, app *App, options contract.OpenInstanceOptions) error {
	if app == nil {
		return errors.New("launcher instance is unavailable")
	}
	app.prepareInstanceShow(options.Show)
	if err := app.ChangeQuery(ctx, options.Query); err != nil {
		return err
	}
	return app.Show(ctx, options.Show)
}

// prepareInstanceShow makes early query frames use the target window layout before presentation.
func (a *App) prepareInstanceShow(options contract.ShowOptions) {
	params := fromCoreShowOptions(options)
	if params.WindowWidth <= 0 {
		params.WindowWidth = defaultWidth
	}
	if params.MaxResultCount <= 0 {
		params.MaxResultCount = defaultMaxResult
	}
	a.mu.Lock()
	a.show = params
	a.mu.Unlock()
}

func (r *appInstanceRegistry) remove(app *App) {
	if r == nil || app == nil {
		return
	}
	r.mu.Lock()
	if r.bySessionID[app.sessionID] == app {
		delete(r.bySessionID, app.sessionID)
	}
	if app.instanceName != "" && r.sessionByName[app.instanceName] == app.sessionID {
		delete(r.sessionByName, app.instanceName)
	}
	r.mu.Unlock()
}

func (a *App) isLive() bool {
	a.mu.RLock()
	launcher := a.launcher
	destroyed := a.destroyed
	a.mu.RUnlock()
	if launcher == nil || destroyed {
		return false
	}
	lifecycle := launcher.Lifecycle()
	return lifecycle != woxui.WindowLifecycleClosing && lifecycle != woxui.WindowLifecycleClosed
}

func (a *App) isDestroyed() bool {
	a.mu.RLock()
	destroyed := a.destroyed
	a.mu.RUnlock()
	return destroyed
}

// OpenInstance implements primary handoff and named secondary launcher reuse.
func (a *App) OpenInstance(ctx context.Context, options contract.OpenInstanceOptions) error {
	if a.instances == nil {
		return errors.New("launcher instance registry is unavailable")
	}
	return a.instances.open(ctx, options)
}

// destroySecondary releases session subscriptions, core caches, and transport ownership exactly once.
func (a *App) destroySecondary() {
	if a == nil || a.isPrimary {
		return
	}
	a.destroyOnce.Do(func() {
		a.mu.Lock()
		a.destroyed = true
		cancel := a.cancel
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		a.releaseTerminalSubscription()
		a.unsubscribeAll()
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		if err := a.services.DestroyInstance(ctx, a.sessionID); err != nil {
			log.Printf("destroy secondary Wox instance %s: %v", a.sessionID, err)
		}
		cancel()
		if a.instances != nil {
			a.instances.remove(a)
		}
		if a.client != nil {
			if err := a.client.Close(); err != nil {
				log.Printf("close secondary Wox instance %s backend: %v", a.sessionID, err)
			}
		}
	})
}

func (a *App) releaseTerminalSubscription() {
	a.mu.Lock()
	a.terminalPreview = nil
	a.mu.Unlock()
	a.terminalSubscriptionMu.Lock()
	defer a.terminalSubscriptionMu.Unlock()
	if a.terminalSubscribed == "" {
		return
	}
	if err := a.services.UnsubscribeTerminal(context.Background(), a.sessionID, a.terminalSubscribed); err != nil {
		log.Printf("unsubscribe terminal during Wox instance cleanup: %v", err)
	}
	a.terminalSubscribed = ""
}

func (a *App) unsubscribeAll() {
	a.mu.Lock()
	unsubscribers := a.unsubscribers
	a.unsubscribers = nil
	a.mu.Unlock()
	for _, unsubscribe := range unsubscribers {
		if unsubscribe != nil {
			unsubscribe()
		}
	}
}

func (a *App) onSharedSettingsChanged(message woxui.WindowMessage) {
	go func() {
		if a.client == nil {
			return
		}
		if err := a.reloadSettings(); err != nil {
			log.Printf("reload shared settings for %s: %v", a.sessionID, err)
		}
		if err := a.reloadTranslations(); err != nil {
			log.Printf("reload shared translations for %s: %v", a.sessionID, err)
		}
		if err := a.reloadTheme(); err != nil {
			log.Printf("reload shared theme for %s: %v", a.sessionID, err)
		}
		if kind, ok := message.Payload.(string); ok && kind == "plugins" {
			a.reloadGlanceCatalogFromCore()
		}
		a.mu.RLock()
		window := a.window
		fontFamily := a.settings.AppFontFamily
		a.mu.RUnlock()
		if window != nil {
			if err := window.SetFontFamily(fontFamily); err != nil {
				log.Printf("apply shared font for %s: %v", a.sessionID, err)
			}
			_ = window.Invalidate()
		}
		a.invalidateSettingsWindow()
	}()
}

type woxInstanceRole string

const woxInstanceRoleSecondary woxInstanceRole = "secondary"
