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
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"

	woxui "wox/ui/runtime"
	woxsvg "wox/util/svg"
)

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
	CacheKey    string   `json:"cacheKey"`
	Placeholder woxImage `json:"placeholder"`
}

const launcherImageCacheLimit = 512

func (a *App) imageFor(source woxImage) *woxui.Image {
	return a.imageForTint(source, nil, 256)
}

// imageForSize resolves display images at a caller-selected resolution while sharing the image cache.
func (a *App) imageForSize(source woxImage, size int) *woxui.Image {
	return a.imageForTint(source, nil, size)
}

// physicalImageSize keeps rasterized vector assets sharp at the window's current backing scale.
func physicalImageSize(logicalSize int, scale float32) int {
	if scale <= 0 {
		scale = 1
	}
	return max(1, int(math.Ceil(float64(float32(logicalSize)*scale))))
}

// imageForTint applies a source-in tint to SVG images and sets the resolution for core-resolved assets.
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
	a.imageUseSequence++
	a.imageLastUsed[key] = a.imageUseSequence
	image := a.images[key]
	requestedSource, requested := a.imageRequested[key]
	if image != nil || requested && requestedSource == source.ImageData {
		a.mu.Unlock()
		return image
	}
	a.imageRequested[key] = source.ImageData
	delete(a.imageErrors, key)
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
		err := a.client.Post(ctx, "/image/resolve", map[string]any{"Image": source, "Size": svgSize}, &resolved)
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
	if _, exists := a.images[key]; !exists && len(a.images) >= launcherImageCacheLimit {
		a.evictOldestImageLocked(key)
	}
	a.images[key] = image
	delete(a.imageErrors, key)
	a.mu.Unlock()
	a.invalidateAllWindows()
}

func (a *App) storeImageError(key string, err error) {
	a.mu.Lock()
	a.imageErrors[key] = err.Error()
	a.mu.Unlock()
	a.invalidateAllWindows()
}

// evictOldestImageLocked removes one cold image without invalidating the rest of the decoded cache.
func (a *App) evictOldestImageLocked(keepKey string) {
	oldestKey := ""
	oldestUse := ^uint64(0)
	for key := range a.images {
		if key == keepKey {
			continue
		}
		used := a.imageLastUsed[key]
		if oldestKey == "" || used < oldestUse {
			oldestKey = key
			oldestUse = used
		}
	}
	if oldestKey == "" {
		return
	}
	delete(a.images, oldestKey)
	delete(a.imageRequested, oldestKey)
	delete(a.imageLastUsed, oldestKey)
	delete(a.imageErrors, oldestKey)
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
	rgba, err := woxsvg.Render(data, size, size)
	if err != nil {
		return nil, err
	}
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
	if source.ImageType == "lazyloadimage" {
		var payload lazyImagePayload
		if json.Unmarshal([]byte(source.ImageData), &payload) == nil && payload.CacheKey != "" {
			// Lazy authorization tokens change for every query and must not invalidate
			// an icon that was already resolved from the same stable source.
			return "lazy-" + payload.CacheKey
		}
	}
	hash := sha256.Sum256([]byte(source.ImageType + "\x00" + source.ImageData))
	return fmt.Sprintf("%x", hash[:])
}
