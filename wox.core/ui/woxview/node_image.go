package woxview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"os"
	"path/filepath"
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

// Budgets apply before pixel allocation, independently of JSON and logical layout limits.
const (
	maxSVGRasterSize    = 2048
	maxImagePixels      = 16 * 1024 * 1024
	maxImageSourceBytes = 32 * 1024 * 1024
	maxImageCacheBytes  = 64 * 1024 * 1024
)

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
	} else if image := b.view.images.get(props, !b.theme().Dark, width, height); image != nil {
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

type imageCacheKey struct {
	digest     [32]byte
	rasterSize int
	lightTheme bool
}

type imageCacheEntry struct {
	image *woxui.Image
	used  bool
	bytes int
}

// imageCache keeps decoded images across tree updates; entries not referenced by the latest
// Build are dropped so replaced images do not accumulate. Failed decodes are cached as nil
// so a broken source is not retried every frame.
type imageCache struct {
	entries    map[imageCacheKey]*imageCacheEntry
	bytes      int
	frameBytes int
}

func (c *imageCache) beginBuild() {
	c.frameBytes = 0
	for _, entry := range c.entries {
		entry.used = false
	}
}

func (c *imageCache) endBuild() {
	for key, entry := range c.entries {
		if !entry.used {
			c.bytes -= entry.bytes
			delete(c.entries, key)
		}
	}
}

// get decodes synchronously on first use. SVG rasterizes at twice the logical size for HiDPI;
// raster formats keep their own pixels and are scaled by the Image widget.
func (c *imageCache) get(props *ImageProps, lightTheme bool, width, height float32) *woxui.Image {
	source := *props.Source
	key := imageCacheKey{digest: props.digest, lightTheme: lightTheme && props.themed}
	// File sources may also contain SVG. Clamp before converting to int and
	// before decoding so a large logical box cannot allocate a giant square.
	key.rasterSize = int(min(float64(maxSVGRasterSize), max(32, math.Ceil(float64(max(width, height)*2)))))
	bitmapKey := key
	bitmapKey.rasterSize = 0
	if _, ok := c.entries[bitmapKey]; ok {
		key = bitmapKey
	}
	if entry, ok := c.entries[key]; ok {
		if !entry.used {
			if c.frameBytes+entry.bytes > maxImageCacheBytes {
				return nil
			}
			c.frameBytes += entry.bytes
		}
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
	decoded, vector, err := decodeImage(source, key.rasterSize, maxImageCacheBytes-c.frameBytes)
	if errors.Is(err, errImageFrameBudget) {
		// Retry next frame: removing other images can make room without changing this source.
		return nil
	}
	if err != nil {
		util.GetLogger().Warn(context.Background(), fmt.Sprintf("woxview: failed to decode %s image: %v", source.ImageType, err))
	}
	if !vector {
		key.rasterSize = 0
	}
	size := 0
	if decoded != nil {
		size = decoded.Width * decoded.Height * 4
	}
	for oldKey, entry := range c.entries {
		if c.bytes+size <= maxImageCacheBytes {
			break
		}
		if !entry.used {
			c.bytes -= entry.bytes
			delete(c.entries, oldKey)
		}
	}
	c.entries[key] = &imageCacheEntry{image: decoded, used: true, bytes: size}
	c.bytes += size
	c.frameBytes += size
	return decoded
}

var errImageFrameBudget = errors.New("woxview image frame exceeds 64 MiB")

// decodeImage bounds encoded input and decoded pixels before allocating raster buffers.
func decodeImage(source ImageSource, rasterSize, remainingBytes int) (*woxui.Image, bool, error) {
	var data []byte
	switch source.ImageType {
	case common.WoxImageTypeSvg:
		data = []byte(source.ImageData)
	case common.WoxImageTypeAbsolutePath:
		file, err := os.Open(source.ImageData)
		if err != nil {
			return nil, false, err
		}
		defer file.Close()
		data, err = io.ReadAll(io.LimitReader(file, maxImageSourceBytes+1))
		if err != nil {
			return nil, false, err
		}
	case common.WoxImageTypeBase64:
		_, encoded, found := strings.Cut(source.ImageData, ",")
		if !found {
			return nil, false, errors.New("invalid base64 image data")
		}
		if base64.StdEncoding.DecodedLen(len(encoded)) > maxImageSourceBytes {
			return nil, false, errors.New("image source exceeds 32 MiB")
		}
		var err error
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, false, err
		}
	default:
		return nil, false, fmt.Errorf("unsupported image type %q", source.ImageType)
	}
	if len(data) > maxImageSourceBytes {
		return nil, false, errors.New("image source exceeds 32 MiB")
	}
	isSVG := source.ImageType == common.WoxImageTypeSvg || source.ImageType == common.WoxImageTypeAbsolutePath && (strings.EqualFold(filepath.Ext(source.ImageData), ".svg") || bytes.Contains(data[:min(len(data), 512)], []byte("<svg")))
	if isSVG {
		icon, err := woxsvg.Parse(bytes.NewReader(data))
		if err != nil {
			return nil, false, err
		}
		width, height := icon.ViewBoxSize()
		scale := float64(min(maxSVGRasterSize, max(1, rasterSize))) / max(width, height)
		pixelWidth, pixelHeight := max(1, int(math.Round(width*scale))), max(1, int(math.Round(height*scale)))
		if pixelWidth*pixelHeight*4 > remainingBytes {
			return nil, false, errImageFrameBudget
		}
		// Keep the source aspect ratio; Image owns contain/cover/fill in logical coordinates.
		rgba, err := icon.Render(pixelWidth, pixelHeight)
		if err != nil {
			return nil, false, err
		}
		converted, err := woxui.NewImageFromPackedRGBA(rgba)
		return converted, true, err
	}
	config, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return nil, false, err
	}
	if config.Width <= 0 || config.Height <= 0 || config.Width > 16384 || config.Height > 16384 || int64(config.Width)*int64(config.Height) > maxImagePixels {
		return nil, false, errors.New("image exceeds dimension or 16 megapixel limit")
	}
	if config.Width*config.Height*4 > remainingBytes {
		return nil, false, errImageFrameBudget
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, false, err
	}
	converted, err := woxui.NewImage(decoded)
	return converted, false, err
}
