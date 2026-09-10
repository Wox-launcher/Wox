package icons

import (
	"sort"
	"sync/atomic"

	"wox/common"
)

var currentTheme atomic.Value

func init() {
	currentTheme.Store(DefaultTheme())
}

// Get returns the current theme's icon, falling back to the default pack.
func Get(name string) common.WoxImage {
	if theme, ok := currentTheme.Load().(Theme); ok && theme != nil {
		if icon, found := theme.Icon(name); found {
			return icon
		}
	}
	if icon, found := DefaultTheme().Icon(name); found {
		return icon
	}
	return common.WoxImage{}
}

// MustGet is Get for call sites that treat a missing catalog name as a programming error.
func MustGet(name string) common.WoxImage {
	icon := Get(name)
	if icon.IsEmpty() {
		panic("unknown icon: " + name)
	}
	return icon
}

// SetTheme replaces the process-wide icon pack. Missing names keep using default.
func SetTheme(theme Theme) {
	if theme == nil {
		theme = DefaultTheme()
	}
	currentTheme.Store(theme)
}

// CurrentTheme returns the active icon pack.
func CurrentTheme() Theme {
	if theme, ok := currentTheme.Load().(Theme); ok && theme != nil {
		return theme
	}
	return DefaultTheme()
}

var defaultTheme Theme

// DefaultTheme returns the built-in Wox icon pack.
func DefaultTheme() Theme {
	if defaultTheme == nil {
		defaultTheme = NewMapTheme("default", defaultIcons())
	}
	return defaultTheme
}

func defaultIcons() map[string]common.WoxImage {
	icons := make(map[string]common.WoxImage, len(defaultUIIcons)+len(defaultPluginIcons)+len(defaultActionIcons)+len(defaultSysIcons)+len(defaultMiscIcons))
	for name, icon := range defaultUIIcons {
		icons[name] = icon
	}
	for name, icon := range defaultPluginIcons {
		icons[name] = icon
	}
	for name, icon := range defaultActionIcons {
		icons[name] = icon
	}
	for name, icon := range defaultSysIcons {
		icons[name] = icon
	}
	for name, icon := range defaultMiscIcons {
		icons[name] = icon
	}
	return icons
}

// Names returns every semantic name registered by the default theme.
func Names() []string {
	icons := defaultIcons()
	names := make([]string, 0, len(icons))
	for name := range icons {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
