package launcher

import (
	"context"
	"sort"
	"strings"
	"time"

	"wox/ui/contract"
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
func (c *appearanceSettingsController) ReloadFonts(ctx context.Context, service contract.AppearanceSettingsServices, sessionID string) {
	shouldLoad := false
	if !c.deps.OnUI("start loading system font families", func() {
		if c.fontsLoaded || c.fontsLoading {
			return
		}
		c.fontsLoading = true
		c.fontsError = ""
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	families, err := service.SystemFontFamilies(timeoutCtx, sessionID)
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

	c.deps.OnUI("apply system font families", func() {
		c.fontsLoading = false
		if err != nil {
			c.fontsError = err.Error()
		} else {
			c.fontFamilies = families
			c.fontsLoaded = true
			c.fontsError = ""
		}
		c.deps.Invalidate()
	})
}

// ReloadGlanceCatalog loads the available glance providers from core. It is a no-op if a
// load has already completed or is in flight. onLoaded is invoked on a successful load so
// the caller (App) can reschedule the active glance refresh against the new catalog without
// the controller needing a back-reference to *App.
func (c *appearanceSettingsController) ReloadGlanceCatalog(ctx context.Context, service contract.AppearanceSettingsServices, sessionID string, onLoaded func()) {
	shouldLoad := false
	if !c.deps.OnUI("start loading glance catalog", func() {
		if c.glanceCatalogLoaded || c.glanceCatalogLoading {
			return
		}
		c.glanceCatalogLoading = true
		c.glanceCatalogError = ""
		shouldLoad = true
		c.deps.Invalidate()
	}) || !shouldLoad {
		return
	}

	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	loaded, err := service.GlanceCatalog(timeoutCtx, sessionID)
	cancel()
	catalog := make([]glanceCatalogItem, 0)
	if err == nil {
		for _, item := range loaded {
			if strings.TrimSpace(item.PluginID) == "" || strings.TrimSpace(item.GlanceID) == "" {
				continue
			}
			catalog = append(catalog, glanceCatalogItem{
				Ref: glanceRef{PluginID: item.PluginID, GlanceID: item.GlanceID}, PluginName: item.PluginName, Name: item.Name,
				Description: item.Description, RefreshIntervalMs: item.RefreshIntervalMs,
			})
		}
		sort.SliceStable(catalog, func(i, j int) bool {
			return strings.ToLower(catalog[i].Name+catalog[i].PluginName) < strings.ToLower(catalog[j].Name+catalog[j].PluginName)
		})
	}

	c.deps.OnUI("apply glance catalog", func() {
		c.glanceCatalogLoading = false
		if err != nil {
			c.glanceCatalogError = err.Error()
		} else {
			c.glanceCatalog = catalog
			c.glanceCatalogLoaded = true
			c.glanceCatalogError = ""
		}
		c.deps.Invalidate()
		if err == nil && onLoaded != nil {
			onLoaded()
		}
	})
}

// ResetGlanceCatalog clears the cached catalog so the next ReloadGlanceCatalog refetches
// from core. Called when installed plugins change.
func (c *appearanceSettingsController) ResetGlanceCatalog() {
	c.deps.OnUI("reset glance catalog", func() {
		c.glanceCatalog = nil
		c.glanceCatalogLoaded = false
		c.glanceCatalogError = ""
		c.deps.Invalidate()
	})
}

// Snapshot returns a copy of the Appearance state for the view layer.
func (c *appearanceSettingsController) Snapshot() appearanceSettingsSnapshot {
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
