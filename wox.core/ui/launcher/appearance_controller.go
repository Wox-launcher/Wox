package launcher

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// appearanceSettingsSnapshot is the immutable Appearance tab state consumed by the view layer.
type appearanceSettingsSnapshot struct {
	FontFamilies         []string
	FontsLoading         bool
	FontsLoaded          bool
	FontsError           string
	GlanceCatalog        []glanceCatalogItem
	GlanceCatalogLoading bool
	GlanceCatalogLoaded  bool
	GlanceCatalogError   string
}

// appearanceSettingsController owns the Appearance tab state: the system font family list
// used by the application-font choice picker and the glance provider catalog used by the
// primary-glance choice picker. Both are loaded once from core and cached until reset.
type appearanceSettingsController struct {
	deps                 CommonDeps
	mu                   sync.RWMutex
	fontFamilies         []string
	fontsLoading         bool
	fontsLoaded          bool
	fontsError           string
	glanceCatalog        []glanceCatalogItem
	glanceCatalogLoading bool
	glanceCatalogLoaded  bool
	glanceCatalogError   string
}

func newAppearanceSettingsController(deps CommonDeps) *appearanceSettingsController {
	return &appearanceSettingsController{deps: deps}
}

// ReloadFonts loads the system font family list from core. It is a no-op if a load has
// already completed or is in flight. Mirrors the original loadSystemFontFamilies behavior:
// core enumerates fonts, the controller trims/dedups/sorts the portable family names.
func (c *appearanceSettingsController) ReloadFonts(ctx context.Context, client backendClient) {
	c.mu.Lock()
	if c.fontsLoaded || c.fontsLoading {
		c.mu.Unlock()
		return
	}
	c.fontsLoading = true
	c.fontsError = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	var families []string
	err := client.Post(timeoutCtx, "/setting/ui/fonts", map[string]any{}, &families)
	cancel()
	if err == nil {
		seen := make(map[string]bool, len(families))
		filtered := make([]string, 0, len(families))
		for _, family := range families {
			family = strings.TrimSpace(family)
			key := strings.ToLower(family)
			if family == "" || seen[key] {
				continue
			}
			seen[key] = true
			filtered = append(filtered, family)
		}
		sort.SliceStable(filtered, func(i, j int) bool { return strings.ToLower(filtered[i]) < strings.ToLower(filtered[j]) })
		families = filtered
	}

	c.mu.Lock()
	c.fontsLoading = false
	if err != nil {
		c.fontsError = err.Error()
	} else {
		c.fontFamilies = families
		c.fontsLoaded = true
		c.fontsError = ""
	}
	c.mu.Unlock()
	c.deps.Invalidate()
}

// ReloadGlanceCatalog loads the available glance providers from core. It is a no-op if a
// load has already completed or is in flight. onLoaded is invoked on a successful load so
// the caller (App) can reschedule the active glance refresh against the new catalog without
// the controller needing a back-reference to *App.
func (c *appearanceSettingsController) ReloadGlanceCatalog(ctx context.Context, client backendClient, onLoaded func()) {
	c.mu.Lock()
	if c.glanceCatalogLoaded || c.glanceCatalogLoading {
		c.mu.Unlock()
		return
	}
	c.glanceCatalogLoading = true
	c.glanceCatalogError = ""
	c.mu.Unlock()
	c.deps.Invalidate()

	var plugins []struct {
		ID      string         `json:"Id"`
		Name    string         `json:"Name"`
		Glances []pluginGlance `json:"Glances"`
	}
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	err := client.Post(timeoutCtx, "/plugin/installed", map[string]any{}, &plugins)
	cancel()
	catalog := make([]glanceCatalogItem, 0)
	if err == nil {
		for _, plugin := range plugins {
			for _, glance := range plugin.Glances {
				if strings.TrimSpace(plugin.ID) == "" || strings.TrimSpace(glance.ID) == "" {
					continue
				}
				catalog = append(catalog, glanceCatalogItem{
					Ref: glanceRef{PluginID: plugin.ID, GlanceID: glance.ID}, PluginName: plugin.Name, Name: glance.Name,
					Description: glance.Description, RefreshIntervalMs: glance.RefreshIntervalMs,
				})
			}
		}
		sort.SliceStable(catalog, func(i, j int) bool {
			return strings.ToLower(catalog[i].Name+catalog[i].PluginName) < strings.ToLower(catalog[j].Name+catalog[j].PluginName)
		})
	}

	c.mu.Lock()
	c.glanceCatalogLoading = false
	if err != nil {
		c.glanceCatalogError = err.Error()
	} else {
		c.glanceCatalog = catalog
		c.glanceCatalogLoaded = true
		c.glanceCatalogError = ""
	}
	c.mu.Unlock()
	c.deps.Invalidate()
	if err == nil && onLoaded != nil {
		onLoaded()
	}
}

// ResetGlanceCatalog clears the cached catalog so the next ReloadGlanceCatalog refetches
// from core. Called when installed plugins change.
func (c *appearanceSettingsController) ResetGlanceCatalog() {
	c.mu.Lock()
	c.glanceCatalog = nil
	c.glanceCatalogLoaded = false
	c.glanceCatalogError = ""
	c.mu.Unlock()
	c.deps.Invalidate()
}

// Snapshot returns a copy of the Appearance state for the view layer.
func (c *appearanceSettingsController) Snapshot() appearanceSettingsSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return appearanceSettingsSnapshot{
		FontFamilies:         append([]string(nil), c.fontFamilies...),
		FontsLoading:         c.fontsLoading,
		FontsLoaded:          c.fontsLoaded,
		FontsError:           c.fontsError,
		GlanceCatalog:        append([]glanceCatalogItem(nil), c.glanceCatalog...),
		GlanceCatalogLoading: c.glanceCatalogLoading,
		GlanceCatalogLoaded:  c.glanceCatalogLoaded,
		GlanceCatalogError:   c.glanceCatalogError,
	}
}
