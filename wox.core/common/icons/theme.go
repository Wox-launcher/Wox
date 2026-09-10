package icons

import "wox/common"

// Theme resolves semantic icon names to images for one icon pack.
type Theme interface {
	Name() string
	Icon(name string) (common.WoxImage, bool)
}

// MapTheme is a name-to-image pack used by the default theme and future packs.
type MapTheme struct {
	name  string
	icons map[string]common.WoxImage
}

// NewMapTheme copies icons into an immutable lookup table.
func NewMapTheme(name string, icons map[string]common.WoxImage) Theme {
	copied := make(map[string]common.WoxImage, len(icons))
	for key, icon := range icons {
		copied[key] = icon
	}
	return MapTheme{name: name, icons: copied}
}

func (t MapTheme) Name() string {
	return t.name
}

func (t MapTheme) Icon(name string) (common.WoxImage, bool) {
	icon, ok := t.icons[name]
	return icon, ok
}
