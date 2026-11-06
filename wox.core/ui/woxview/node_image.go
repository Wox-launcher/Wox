package woxview

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"strings"

	"wox/common"
	"wox/common/icons"
	component "wox/ui/launcher/component"
	woxui "wox/ui/runtime"
	widget "wox/ui/widget"
	"wox/util"
	woxsvg "wox/util/svg"
)

// ImageFit controls aspect-ratio handling inside the image box.
type ImageFit string

func (ImageFit) enumValues() []string {
	return []string{"", string(ImageFitContain), string(ImageFitCover), string(ImageFitFill)}
}

const (
	ImageFitContain ImageFit = "contain"
	ImageFitCover   ImageFit = "cover"
	ImageFitFill    ImageFit = "fill"
)

const svgThemeIconColor = "var(--wox-theme-icon-color)"

// IconColorMode chooses authored colors by default or an explicitly tinted glyph.
type IconColorMode string

const (
	IconColorTint     IconColorMode = "tint"
	IconColorOriginal IconColorMode = "original"
)

func (IconColorMode) enumValues() []string {
	return []string{"", string(IconColorTint), string(IconColorOriginal)}
}

func (mode IconColorMode) validate() error {
	if mode != "" && mode != IconColorTint && mode != IconColorOriginal {
		return fmt.Errorf("unknown iconColorMode %q", mode)
	}
	return nil
}

// ImageSource mirrors the plugin WoxImage shape. Only local types are accepted so building
// never waits on the network.
type ImageSource struct {
	ImageType string `json:"imageType"`
	ImageData string `json:"imageData"`
}

// ImageProps shows either a catalog icon (Icon) or an image (Source).
type ImageProps struct {
	Styled
	Source *ImageSource `json:"source,omitempty"`
	// Icon is a catalog name from wox/common/icons.
	Icon          string        `json:"icon,omitempty"`
	IconColorMode IconColorMode `json:"iconColorMode,omitempty"`
	Fit           ImageFit      `json:"fit,omitempty"`
	// digest identifies Source in the image cache without rehashing large data every frame.
	digest [32]byte
	// themed marks SVGs using the Wox theme icon variable, whose raster depends on the theme.
	themed bool
}

func (p *ImageProps) prepare() error {
	if err := p.IconColorMode.validate(); err != nil {
		return err
	}
	if p.Source != nil && p.IconColorMode != "" {
		return errors.New("iconColorMode requires a catalog icon")
	}
	if (p.Source == nil) == (p.Icon == "") {
		return errors.New("image needs exactly one of source or icon")
	}
	if p.Icon != "" && icons.Get(p.Icon).IsEmpty() {
		return fmt.Errorf("unknown icon %q", p.Icon)
	}
	if p.Source != nil {
		switch p.Source.ImageType {
		case common.WoxImageTypeBase64, common.WoxImageTypeSvg, common.WoxImageTypeAbsolutePath:
		default:
			return fmt.Errorf("unsupported image type %q; use base64, svg, or absolute", p.Source.ImageType)
		}
		if p.Source.ImageData == "" {
			return errors.New("image source has no data")
		}
		p.digest = sha256.Sum256([]byte(p.Source.ImageType + "\x00" + p.Source.ImageData))
		p.themed = p.Source.ImageType == common.WoxImageTypeSvg && strings.Contains(p.Source.ImageData, svgThemeIconColor)
	}
	switch p.Fit {
	case "", ImageFitContain, ImageFitCover, ImageFitFill:
	default:
		return fmt.Errorf("unknown fit %q", p.Fit)
	}
	return p.Style.validate()
}

func buildImage(b *builder, node *Node, id string) widget.Widget {
	props := node.Props.(*ImageProps)
	style := props.Style.resolve(b.theme())
	defaultSize := float32(24)
	if props.Icon != "" {
		defaultSize = 16
	}
	width, height := style.Width, style.Height
	if width <= 0 {
		width = defaultSize
	}
	if height <= 0 {
		height = width
	}
	var content widget.Widget
	if props.Icon != "" && props.IconColorMode != IconColorTint {
		fit := widget.ImageFitContain
		switch props.Fit {
		case ImageFitCover:
			fit = widget.ImageFitCover
		case ImageFitFill:
			fit = widget.ImageFitFill
		}
		content = widget.Image{Source: b.originalIcon(props.Icon, max(width, height)), Width: width, Height: height, Fit: fit, Radius: style.BorderRadius}
	} else if props.Icon != "" {
		content = component.IconGlyph(props.Icon, min(width, height), style.Color.resolve(b.theme(), b.theme().Text))
	} else if image := b.view.images.get(props, isDarkText(b.theme().Text), width, height); image != nil {
		fit := widget.ImageFitContain
		switch props.Fit {
		case ImageFitCover:
			fit = widget.ImageFitCover
		case ImageFitFill:
			fit = widget.ImageFitFill
		}
		content = widget.Image{Source: image, Width: width, Height: height, Fit: fit, Radius: style.BorderRadius}
	} else {
		content = widget.Container{Width: width, Height: height}
	}
	return style.wrapOuter(b.theme(), content)
}

// originalIcon uses the view's image cache and resolves only the explicit Wox
// theme variable, leaving fixed colors and gradients in catalog artwork intact.
func (b *builder) originalIcon(name string, size float32) *woxui.Image {
	icon := icons.Get(name)
	props := &ImageProps{Source: &ImageSource{ImageType: icon.ImageType, ImageData: icon.ImageData}}
	if err := props.prepare(); err != nil {
		return nil
	}
	return b.view.images.get(props, !b.theme().Dark, size, size)
}

// isDarkText reports a dark text color, i.e. a light theme.
func isDarkText(text woxui.Color) bool {
	return 299*int(text.R)+587*int(text.G)+114*int(text.B) <= 128*1000
}

type imageCacheKey struct {
	digest     [32]byte
	rasterSize int
	lightTheme bool
}

type imageCacheEntry struct {
	image *woxui.Image
	used  bool
}

// imageCache keeps decoded images across tree updates; entries not referenced by the latest
// Build are dropped so replaced images do not accumulate. Failed decodes are cached as nil
// so a broken source is not retried every frame.
type imageCache struct {
	entries map[imageCacheKey]*imageCacheEntry
}

func (c *imageCache) beginBuild() {
	for _, entry := range c.entries {
		entry.used = false
	}
}

func (c *imageCache) endBuild() {
	for key, entry := range c.entries {
		if !entry.used {
			delete(c.entries, key)
		}
	}
}

// get decodes synchronously on first use. SVG rasterizes at twice the logical size for HiDPI;
// raster formats keep their own pixels and are scaled by the Image widget.
func (c *imageCache) get(props *ImageProps, lightTheme bool, width, height float32) *woxui.Image {
	source := *props.Source
	key := imageCacheKey{digest: props.digest, lightTheme: lightTheme && props.themed}
	if source.ImageType == common.WoxImageTypeSvg {
		key.rasterSize = max(32, int(math.Ceil(float64(max(width, height)*2))))
	}
	if entry, ok := c.entries[key]; ok {
		entry.used = true
		return entry.image
	}
	if props.themed {
		// The launcher pipeline resolves the Wox theme variable to white on dark themes and black on light themes.
		color := "#ffffff"
		if lightTheme {
			color = "#000000"
		}
		source.ImageData = strings.ReplaceAll(source.ImageData, svgThemeIconColor, color)
	}
	image, err := decodeImage(source, key.rasterSize)
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("woxview: failed to decode %s image: %v", source.ImageType, err))
	}
	c.entries[key] = &imageCacheEntry{image: image, used: true}
	return image
}

func decodeImage(source ImageSource, rasterSize int) (*woxui.Image, error) {
	if source.ImageType == common.WoxImageTypeSvg {
		rgba, err := woxsvg.Render(source.ImageData, rasterSize, rasterSize)
		if err != nil {
			return nil, err
		}
		return woxui.NewImage(rgba)
	}
	decoded, err := common.WoxImage{ImageType: source.ImageType, ImageData: source.ImageData}.ToImageWithoutRemoteFetch()
	if err != nil {
		return nil, err
	}
	return woxui.NewImage(decoded)
}
