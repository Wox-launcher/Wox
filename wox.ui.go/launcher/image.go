package launcher

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	woxui "github.com/Wox-launcher/wox.ui.go"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

var svgRootEmDimensionPattern = regexp.MustCompile(`\s(?:width|height)=["'][^"']*em["']`)

type woxImage struct {
	ImageType string `json:"ImageType"`
	ImageData string `json:"ImageData"`
}

// UnmarshalJSON accepts both the structured image DTO and legacy type:data strings.
func (w *woxImage) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var value string
		if err := json.Unmarshal(data, &value); err != nil {
			return err
		}
		if value == "" {
			*w = woxImage{}
			return nil
		}
		imageType, imageData, ok := strings.Cut(value, ":")
		if !ok {
			return fmt.Errorf("invalid Wox image string")
		}
		w.ImageType = imageType
		w.ImageData = imageData
		return nil
	}
	type imageAlias woxImage
	var decoded imageAlias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*w = woxImage(decoded)
	return nil
}

type lazyImagePayload struct {
	Token       string   `json:"token"`
	Placeholder woxImage `json:"placeholder"`
}

func (a *App) imageFor(source woxImage) *woxui.Image {
	return a.imageForTint(source, nil, 256)
}

// imageForTint applies a Flutter-compatible srcIn tint to SVG images without changing raster assets.
func (a *App) imageForTint(source woxImage, tint *woxui.Color, svgSize int) *woxui.Image {
	if source.ImageType == "" || source.ImageData == "" {
		return nil
	}
	key := imageKey(source)
	key += fmt.Sprintf("-svg-%d", svgSize)
	if tint != nil {
		key += fmt.Sprintf("-tint-%02x%02x%02x%02x", tint.R, tint.G, tint.B, tint.A)
	}
	a.mu.Lock()
	image := a.images[key]
	if image != nil || a.imageRequested[key] {
		a.mu.Unlock()
		return image
	}
	a.imageRequested[key] = true
	a.mu.Unlock()
	go a.loadImage(key, source, tint, svgSize)
	return nil
}

func (a *App) loadImage(key string, source woxImage, tint *woxui.Color, svgSize int) {
	if source.ImageType == "lazyloadimage" {
		var payload lazyImagePayload
		if err := json.Unmarshal([]byte(source.ImageData), &payload); err != nil {
			log.Printf("decode lazy result image payload: %v", err)
			a.storeImageError(key, err)
			return
		}
		if placeholder, err := decodeWoxImageWithTint(payload.Placeholder, tint, svgSize); err == nil {
			a.storeImage(key, placeholder)
		}
		if payload.Token == "" {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var resolved woxImage
		err := a.client.Post(ctx, "/image/lazy/load", map[string]string{"token": payload.Token}, &resolved)
		cancel()
		if err != nil {
			log.Printf("load lazy result image: %v", err)
			a.storeImageError(key, err)
			return
		}
		image, err := decodeWoxImageWithTint(resolved, tint, svgSize)
		if err != nil {
			log.Printf("decode resolved lazy result image: %v", err)
			a.storeImageError(key, err)
			return
		}
		a.storeImage(key, image)
		return
	}
	if source.ImageType == "url" || source.ImageType == "emoji" || source.ImageType == "fileicon" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		var resolved woxImage
		err := a.client.Post(ctx, "/image/resolve", map[string]any{"Image": source, "Size": 256}, &resolved)
		cancel()
		if err != nil {
			log.Printf("resolve %s result image %q: %v", source.ImageType, source.ImageData, err)
			a.storeImageError(key, err)
			return
		}
		image, err := decodeWoxImageWithTint(resolved, tint, svgSize)
		if err != nil {
			log.Printf("decode resolved %s result image: %v", source.ImageType, err)
			a.storeImageError(key, err)
			return
		}
		a.storeImage(key, image)
		return
	}

	image, err := decodeWoxImageWithTint(source, tint, svgSize)
	if err != nil {
		log.Printf("decode %s result image: %v", source.ImageType, err)
		a.storeImageError(key, err)
		return
	}
	a.storeImage(key, image)
}

func (a *App) storeImage(key string, image *woxui.Image) {
	if image == nil {
		return
	}
	a.mu.Lock()
	if len(a.images) >= 512 {
		// ponytail: A bounded reset is enough for invalidated launcher frames; add LRU bookkeeping only if real sessions churn this cache.
		a.images = map[string]*woxui.Image{}
		a.imageRequested = map[string]bool{key: true}
		a.imageErrors = map[string]string{}
	}
	a.images[key] = image
	delete(a.imageErrors, key)
	a.mu.Unlock()
	_ = a.window.Invalidate()
}

func (a *App) storeImageError(key string, err error) {
	a.mu.Lock()
	a.imageErrors[key] = err.Error()
	a.mu.Unlock()
	if a.window != nil {
		_ = a.window.Invalidate()
	}
}

func (a *App) imageErrorFor(source woxImage) string {
	key := imageKey(source)
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.imageErrors[key]
}

func decodeWoxImage(source woxImage) (*woxui.Image, error) {
	return decodeWoxImageWithTint(source, nil, 256)
}

func decodeWoxImageWithTint(source woxImage, tint *woxui.Color, svgSize int) (*woxui.Image, error) {
	switch source.ImageType {
	case "absolute":
		if strings.EqualFold(filepath.Ext(source.ImageData), ".svg") {
			data, err := os.ReadFile(source.ImageData)
			if err != nil {
				return nil, err
			}
			return decodeSVGImage(string(data), svgSize, tint)
		}
		file, err := os.Open(source.ImageData)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		return woxui.DecodeImage(file)
	case "base64":
		encoded := source.ImageData
		if comma := strings.IndexByte(encoded, ','); comma >= 0 {
			encoded = encoded[comma+1:]
		}
		pixels, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, err
		}
		if strings.Contains(strings.ToLower(source.ImageData), "image/svg+xml") {
			return decodeSVGImage(string(pixels), svgSize, tint)
		}
		return woxui.DecodeImage(bytes.NewReader(pixels))
	case "svg":
		return decodeSVGImage(source.ImageData, svgSize, tint)
	case "theme":
		return decodeThemeImage(source.ImageData)
	default:
		return nil, fmt.Errorf("unsupported Wox image type %q", source.ImageType)
	}
}

func decodeSVGImage(data string, size int, tint *woxui.Color) (*woxui.Image, error) {
	data = strings.ReplaceAll(data, "currentColor", "#000000")
	rootEnd := strings.IndexByte(data, '>')
	if rootEnd >= 0 && strings.Contains(data[:rootEnd], "<svg") {
		data = svgRootEmDimensionPattern.ReplaceAllString(data[:rootEnd], "") + data[rootEnd:]
	}
	icon, err := oksvg.ReadIconStream(strings.NewReader(data), oksvg.WarnErrorMode)
	if err != nil {
		return nil, err
	}
	icon.SetTarget(0, 0, float64(size), float64(size))
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	icon.Draw(rasterx.NewDasher(size, size, rasterx.NewScannerGV(size, size, rgba, rgba.Bounds())), 1)
	if tint != nil {
		for index := 0; index < len(rgba.Pix); index += 4 {
			alpha := uint8((uint16(rgba.Pix[index+3])*uint16(tint.A) + 127) / 255)
			rgba.Pix[index] = uint8((uint16(tint.R)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+1] = uint8((uint16(tint.G)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+2] = uint8((uint16(tint.B)*uint16(alpha) + 127) / 255)
			rgba.Pix[index+3] = alpha
		}
	}
	return woxui.NewImage(rgba)
}

func decodeThemeImage(data string) (*woxui.Image, error) {
	var theme struct {
		AppBackgroundColor              string
		QueryBoxBackgroundColor         string
		ResultItemActiveBackgroundColor string
		PreviewFontColor                string
	}
	if err := json.Unmarshal([]byte(data), &theme); err != nil {
		return nil, err
	}
	const size = 128
	rgba := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(rgba, rgba.Bounds(), image.NewUniform(themeRasterColor(parseThemeColor(theme.AppBackgroundColor, defaultPalette().background))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(14, 17, 114, 43), image.NewUniform(themeRasterColor(parseThemeColor(theme.QueryBoxBackgroundColor, defaultPalette().queryBackground))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(14, 54, 114, 88), image.NewUniform(themeRasterColor(parseThemeColor(theme.ResultItemActiveBackgroundColor, defaultPalette().selectedBackground))), image.Point{}, draw.Src)
	draw.Draw(rgba, image.Rect(23, 102, 105, 108), image.NewUniform(themeRasterColor(parseThemeColor(theme.PreviewFontColor, defaultPalette().previewText))), image.Point{}, draw.Src)
	return woxui.NewImage(rgba)
}

func themeRasterColor(value woxui.Color) color.NRGBA {
	return color.NRGBA{R: value.R, G: value.G, B: value.B, A: value.A}
}

func imageKey(source woxImage) string {
	hash := sha256.Sum256([]byte(source.ImageType + "\x00" + source.ImageData))
	return fmt.Sprintf("%x", hash[:])
}
