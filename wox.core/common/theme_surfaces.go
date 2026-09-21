package common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"path"
	"strings"
)

// ThemeInsets uses logical units, except when used as an image's source Slice.
type ThemeInsets struct{ Top, Right, Bottom, Left int }
type ThemeSize struct{ Width, Height int }
type ThemeOffset struct{ X, Y int }

// ThemeSurface decorates an existing launcher region without replacing its controls.
type ThemeSurface struct {
	Background    *ThemeSurfaceImage `json:",omitempty"`
	Frame         *ThemeSurfaceImage `json:",omitempty"`
	Decorations   []ThemeDecoration  `json:",omitempty"`
	ContentInsets *ThemeInsets       `json:",omitempty"`
}

// ThemeSurfaceImage separates source pixel slices from destination logical insets.
type ThemeRepeat struct {
	X string `json:",omitempty"`
	Y string `json:",omitempty"`
}

type ThemeSurfaceImage struct {
	Repeat *ThemeRepeat `json:",omitempty"`
	Source string
	Mode   string
	Slice  *ThemeInsets `json:",omitempty"`
	Insets *ThemeInsets `json:",omitempty"`
	Size   *ThemeSize   `json:",omitempty"`
}

type ThemeDecoration struct {
	Source string
	Anchor string
	Offset ThemeOffset
	Size   ThemeSize
}

// ThemeSurfaces uses fixed names; a nil surface explicitly clears an inherited region.
type ThemeSurfaces map[string]*ThemeSurface

// ValidThemeAssetPath rejects aliases and paths that are unsafe on any supported OS.
func ValidThemeAssetPath(name string) bool {
	if !fs.ValidPath(name) || name == "." || strings.ContainsAny(name, "\\:\x00*?\"<>|") {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if strings.TrimRight(part, " .") != part {
			return false
		}
		base := strings.ToUpper(strings.SplitN(part, ".", 2)[0])
		if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '0' && base[3] <= '9') {
			return false
		}
	}
	return true
}

// UnmarshalJSON catches misspelled nested fields before a theme silently loses decoration.
func (s *ThemeSurface) UnmarshalJSON(data []byte) error {
	type plain ThemeSurface
	var value plain
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&value); err != nil {
		return err
	}
	*s = ThemeSurface(value)
	return nil
}

// Validate checks all geometry before it reaches layout or image allocation.
func (s ThemeSurfaces) Validate() error {
	validInsets := func(v *ThemeInsets) bool {
		return v == nil || (v.Top >= 0 && v.Right >= 0 && v.Bottom >= 0 && v.Left >= 0 && v.Top <= 4096 && v.Right <= 4096 && v.Bottom <= 4096 && v.Left <= 4096)
	}
	validSource := func(source string) bool {
		ext := strings.ToLower(path.Ext(source))
		return ValidThemeAssetPath(source) && (ext == ".png" || ext == ".jpg" || ext == ".jpeg")
	}
	for name, surface := range s {
		switch name {
		case "App", "QueryBox", "ResultItemActive", "ActionContainer", "Preview", "Toolbar":
		default:
			return fmt.Errorf("unknown theme surface %q", name)
		}
		if surface == nil {
			continue
		}
		if !validInsets(surface.ContentInsets) || (surface.ContentInsets != nil && name != "App") {
			return fmt.Errorf("%s: ContentInsets is only supported on App and must be 0..4096", name)
		}
		for _, layer := range []*ThemeSurfaceImage{surface.Background, surface.Frame} {
			if layer == nil {
				continue
			}
			if !validSource(layer.Source) || !validInsets(layer.Slice) || !validInsets(layer.Insets) {
				return fmt.Errorf("%s: invalid image path or insets", name)
			}
			if layer.Repeat != nil {
				if layer.Mode != "nineSlice" {
					return fmt.Errorf("%s: Repeat requires nineSlice", name)
				}
				for _, mode := range []string{layer.Repeat.X, layer.Repeat.Y} {
					if mode != "" && mode != "stretch" && mode != "tile" {
						return fmt.Errorf("%s: invalid Repeat mode %q", name, mode)
					}
				}
			}
			switch layer.Mode {
			case "stretch", "tile":
				if layer.Slice != nil || layer.Insets != nil {
					return fmt.Errorf("%s: Slice/Insets require nineSlice", name)
				}
			case "nineSlice":
				if layer.Slice == nil || layer.Insets == nil {
					return fmt.Errorf("%s: nineSlice requires Slice and Insets", name)
				}
			default:
				return fmt.Errorf("%s: invalid image mode %q", name, layer.Mode)
			}
			if layer.Size != nil && (layer.Mode != "tile" || !validThemeSize(*layer.Size)) {
				return fmt.Errorf("%s: Size requires tile mode and positive dimensions up to 4096", name)
			}
		}
		if len(surface.Decorations) > 16 {
			return fmt.Errorf("%s: too many decorations", name)
		}
		for _, d := range surface.Decorations {
			if !validSource(d.Source) || !validThemeSize(d.Size) || d.Offset.X < -4096 || d.Offset.X > 4096 || d.Offset.Y < -4096 || d.Offset.Y > 4096 {
				return fmt.Errorf("%s: invalid decoration", name)
			}
			switch d.Anchor {
			case "topLeft", "topCenter", "topRight", "centerLeft", "center", "centerRight", "bottomLeft", "bottomCenter", "bottomRight":
			default:
				return fmt.Errorf("%s: invalid anchor %q", name, d.Anchor)
			}
		}
	}
	return nil
}

func validThemeSize(size ThemeSize) bool {
	return size.Width > 0 && size.Height > 0 && size.Width <= 4096 && size.Height <= 4096
}
