package common

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	"io"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"wox/util"
	"wox/util/fileicon"
	"wox/util/imagecache"
	woxsvg "wox/util/svg"
	"wox/util/timetracking"

	"github.com/disintegration/imaging"
	"github.com/forPelevin/gomoji"
)

type WoxImageType = string

var NOT_PNG_ERR = errors.New("image is not png")

var serverPort int

var pngEncoderBufferPool = sync.Pool{
	New: func() any {
		return &png.EncoderBuffer{}
	},
}

var fastPngEncoder = &png.Encoder{
	CompressionLevel: png.BestSpeed,
	BufferPool:       &pngBufferPool{},
}

var transparentPaddingBypassCache = util.NewHashMap[string, struct{}]()

const (
	ResultListIconSize     = util.ResultListIconSize
	ResultGridIconSize     = util.ResultGridIconSize
	resizeImageCachePrefix = "resize_v2_"
	pngCropLargeDimension  = 1024
)

var (
	pngFileSignature = [8]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}
	pngChunkIHDR     = [4]byte{'I', 'H', 'D', 'R'}
	pngChunkIDAT     = [4]byte{'I', 'D', 'A', 'T'}
	pngChunktRNS     = [4]byte{'t', 'R', 'N', 'S'}
)

const (
	pngColorTypeGrayscale      = 0
	pngColorTypeTruecolor      = 2
	pngColorTypeIndexed        = 3
	pngColorTypeGrayscaleAlpha = 4
	pngColorTypeTruecolorAlpha = 6
)

type pngBufferPool struct{}

type pngCropMetadata struct {
	width                  int
	height                 int
	mayContainTransparency bool
}

func (p *pngBufferPool) Get() *png.EncoderBuffer {
	return pngEncoderBufferPool.Get().(*png.EncoderBuffer)
}

func (p *pngBufferPool) Put(b *png.EncoderBuffer) {
	pngEncoderBufferPool.Put(b)
}

func savePngFast(img image.Image, filename string) error {
	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()
	return fastPngEncoder.Encode(f, img)
}

func rememberTransparentPaddingBypass(imageHash string) {
	transparentPaddingBypassCache.Store(imageHash, struct{}{})
}

func isKnownTransparentPaddingBypass(imageHash string) bool {
	return transparentPaddingBypassCache.Exist(imageHash)
}

// ClearConvertIconPathExistenceCache clears image conversion memory state after image cache removal.
// Callers should invoke this after removing the image cache directory to avoid stale conversion decisions.
func ClearConvertIconPathExistenceCache() {
	imagecache.ClearDerivedPathExistenceCache()
	transparentPaddingBypassCache.Clear()
}

const (
	WoxImageTypeAbsolutePath = "absolute"
	WoxImageTypeRelativePath = "relative"
	WoxImageTypeBase64       = "base64"
	WoxImageTypeSvg          = "svg"
	WoxImageTypeLottie       = "lottie" // only support lottie json data
	WoxImageTypeEmoji        = "emoji"
	WoxImageTypeUrl          = "url"
	WoxImageTypeTheme        = "theme"
	WoxImageTypeFileIcon     = "fileicon" // system associated file icon for given file absolute path
	WoxImageTypeLazyLoad     = "lazyloadimage"
)

type WoxImage struct {
	ImageType WoxImageType
	ImageData string
}

// WoxLazyLoadImagePayload is an internal payload created by core after a plugin
// has already returned a normal WoxImage. Source is used only inside core before
// manager token registration; UI receives the token form and asks core for
// the real resized icon only after the result image widget is built.
type WoxLazyLoadImagePayload struct {
	Token       string    `json:"token,omitempty"`
	Placeholder WoxImage  `json:"placeholder"`
	TargetSize  int       `json:"targetSize"`
	Source      *WoxImage `json:"source,omitempty"`
}

func (w *WoxImage) String() string {
	return fmt.Sprintf("%s:%s", w.ImageType, w.ImageData)
}

func (w *WoxImage) IsEmpty() bool {
	return w.ImageData == ""
}

func (w *WoxImage) ToPng() (image.Image, error) {
	if w.ImageType == WoxImageTypeBase64 {
		if !strings.HasPrefix(w.ImageData, "data:image/png;") {
			return nil, NOT_PNG_ERR
		}

		data := strings.Split(w.ImageData, ",")[1]
		decodedData, base64DecodeErr := base64.StdEncoding.DecodeString(data)
		if base64DecodeErr != nil {
			return nil, base64DecodeErr
		}

		imgReader := bytes.NewReader(decodedData)
		return png.Decode(imgReader)
	}

	if w.ImageType == WoxImageTypeAbsolutePath {
		if isSvgFilePath(w.ImageData) {
			return w.ToImage()
		}

		if !strings.EqualFold(filepath.Ext(w.ImageData), ".png") {
			return nil, NOT_PNG_ERR
		}

		imgReader, openErr := os.Open(w.ImageData)
		if openErr != nil {
			return nil, openErr
		}
		defer imgReader.Close()
		return png.Decode(imgReader)
	}

	if w.ImageType == WoxImageTypeSvg {
		img, imgErr := w.ToImage()
		if imgErr != nil {
			return nil, imgErr
		}
		return img, nil
	}

	return nil, NOT_PNG_ERR
}

func (w *WoxImage) ToImage() (image.Image, error) {
	return w.ToImageWithContext(util.NewTraceContext())
}

// ToImageWithContext converts an image while allowing remote fetch callers to apply cancellation or timeout.
func (w *WoxImage) ToImageWithContext(ctx context.Context) (image.Image, error) {
	return w.toImage(ctx, true)
}

func (w *WoxImage) ToImageWithoutRemoteFetch() (image.Image, error) {
	// Some user-visible flows, such as screenshot success notifications, only need a best-effort icon.
	// The previous implementation always routed emoji icons through Twemoji download on cache miss, which
	// blocked those flows on network latency. Callers that need predictable completion can use this local-only path.
	return w.toImage(util.NewTraceContext(), false)
}

func (w *WoxImage) toImage(ctx context.Context, allowRemoteFetch bool) (image.Image, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	if w.ImageType == WoxImageTypeAbsolutePath {
		if isSvgFilePath(w.ImageData) {
			svgData, err := os.ReadFile(w.ImageData)
			if err != nil {
				return nil, err
			}

			return renderSvgImage(string(svgData))
		}

		return imaging.Open(w.ImageData)
	}
	if w.ImageType == WoxImageTypeBase64 {
		parts := strings.SplitN(w.ImageData, ",", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("invalid base64 image data")
		}

		data := parts[1]
		decodedData, base64DecodeErr := base64.StdEncoding.DecodeString(data)
		if base64DecodeErr != nil {
			return nil, base64DecodeErr
		}

		imgReader := bytes.NewReader(decodedData)
		return imaging.Decode(imgReader)
	}
	if w.ImageType == WoxImageTypeSvg {
		return renderSvgImage(w.ImageData)
	}
	if w.ImageType == WoxImageTypeEmoji {
		emojiPath, err := w.emojiImageCachePath(w.ImageData)
		if err != nil {
			return nil, err
		}

		if info, err := os.Stat(emojiPath); err == nil {
			img, openErr := imaging.Open(emojiPath)
			if openErr != nil {
				return nil, openErr
			}
			imagecache.Touch(util.NewTraceContext(), emojiPath, info)
			return img, nil
		}

		if !allowRemoteFetch {
			return nil, fmt.Errorf("emoji image cache miss: %s", w.ImageData)
		}

		if err := os.MkdirAll(util.GetLocation().GetImageCacheDirectory(), 0755); err != nil {
			return nil, err
		}

		if err := w.downloadEmojiImage(ctx, w.ImageData, emojiPath); err != nil {
			return nil, err
		}

		return imaging.Open(emojiPath)
	}
	if w.ImageType == WoxImageTypeUrl {
		cachePath, err := w.urlImageCachePath(w.ImageData)
		if err != nil {
			return nil, err
		}

		if img, ok, err := w.loadCachedURLImage(cachePath); err != nil {
			return nil, err
		} else if ok {
			return img, nil
		}

		if !allowRemoteFetch {
			return nil, fmt.Errorf("url image cache miss: %s", w.ImageData)
		}

		if err := w.warmURLImageCache(ctx, w.ImageData, cachePath); err != nil {
			return nil, err
		}

		if img, ok, err := w.loadCachedURLImage(cachePath); err != nil {
			return nil, err
		} else if ok {
			return img, nil
		}

		return nil, fmt.Errorf("url image cache miss after download: %s", w.ImageData)
	}

	return nil, fmt.Errorf("unsupported image type: %s", w.ImageType)
}

func (w *WoxImage) emojiImageCachePath(emoji string) (string, error) {
	codePoints, err := w.emojiImageCodePointCandidates(emoji)
	if err != nil {
		return "", err
	}

	return path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("emoji_%s.png", codePoints[0])), nil
}

// downloadEmojiImage tries Twemoji filename variants because some assets omit text/presentation variation selectors.
func (w *WoxImage) downloadEmojiImage(ctx context.Context, emoji string, dest string) error {
	codePoints, err := w.emojiImageCodePointCandidates(emoji)
	if err != nil {
		return err
	}

	var lastErr error
	for _, codePoint := range codePoints {
		url := fmt.Sprintf("https://cdn.jsdelivr.net/gh/twitter/twemoji@v11.0.0/36x36/%s.png", codePoint)
		if downloadErr := util.HttpDownload(ctx, url, dest); downloadErr == nil {
			return nil
		} else {
			lastErr = downloadErr
		}
	}

	return lastErr
}

// emojiImageCodePointCandidates returns Twemoji asset names from most specific to fallback-compatible.
func (w *WoxImage) emojiImageCodePointCandidates(emoji string) ([]string, error) {
	emojiInfo, err := gomoji.GetInfo(emoji)
	if err != nil {
		return nil, err
	}

	parts := strings.Fields(strings.ToLower(emojiInfo.CodePoint))
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty emoji codepoint: %s", emoji)
	}

	codePoint := strings.Join(parts, "-")
	candidates := []string{codePoint}
	withoutVariation := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "fe0e" && part != "fe0f" {
			withoutVariation = append(withoutVariation, part)
		}
	}
	if len(withoutVariation) > 0 {
		fallbackCodePoint := strings.Join(withoutVariation, "-")
		if fallbackCodePoint != codePoint {
			candidates = append(candidates, fallbackCodePoint)
		}
	}

	return candidates, nil
}

func (w *WoxImage) urlImageCachePath(rawURL string) (string, error) {
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}

	ext := strings.ToLower(filepath.Ext(parsedURL.Path))
	if ext == "" || len(ext) > 8 {
		ext = ".img"
	}

	cacheName := fmt.Sprintf("remote_image_%x%s", md5.Sum([]byte(rawURL)), ext)
	return path.Join(util.GetLocation().GetImageCacheDirectory(), cacheName), nil
}

func (w *WoxImage) loadCachedURLImage(cachePath string) (image.Image, bool, error) {
	info, err := os.Stat(cachePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if info.IsDir() || info.Size() == 0 {
		return nil, false, nil
	}

	if isSvgFilePath(cachePath) {
		svgData, err := os.ReadFile(cachePath)
		if err != nil {
			return nil, false, err
		}
		img, err := renderSvgImage(string(svgData))
		if err != nil {
			return nil, false, err
		}
		imagecache.Touch(util.NewTraceContext(), cachePath, info)
		return img, true, nil
	}

	img, err := imaging.Open(cachePath)
	if err != nil {
		return nil, false, err
	}
	imagecache.Touch(util.NewTraceContext(), cachePath, info)
	return img, true, nil
}

func (w *WoxImage) warmURLImageCache(ctx context.Context, rawURL string, cachePath string) error {
	if err := os.MkdirAll(util.GetLocation().GetImageCacheDirectory(), 0755); err != nil {
		return err
	}

	data, err := util.HttpGet(ctx, rawURL)
	if err != nil {
		return err
	}

	return os.WriteFile(cachePath, data, 0644)
}

func isSvgFilePath(filePath string) bool {
	return strings.EqualFold(filepath.Ext(filePath), ".svg")
}

func renderSvgImage(svg string) (image.Image, error) {
	return woxsvg.Render(svg, 32, 32)
}

func (w *WoxImage) IsValid() bool {
	if w.ImageData == "" {
		return false
	}

	// check absolute path exists
	if w.ImageType == WoxImageTypeAbsolutePath {
		if _, err := os.Stat(w.ImageData); err != nil {
			return false
		}
	}

	return true
}

func (w *WoxImage) Hash() string {
	return util.Md5([]byte(w.ImageType + w.ImageData))
}

func (w *WoxImage) Overlay(overlay WoxImage, sizePercent, xPercent, yPercent float64) WoxImage {
	backgroundImg, backErr := w.ToImage()
	if backErr != nil {
		return *w
	}

	overlayImage, overlayErr := overlay.ToImage()
	if overlayErr != nil {
		return *w
	}

	bgWidth := backgroundImg.Bounds().Dx()
	bgHeight := backgroundImg.Bounds().Dy()

	size := int(float64(bgWidth) * sizePercent)
	x := int(float64(bgWidth) * xPercent)
	y := int(float64(bgHeight) * yPercent)

	resizedOverlayImg := imaging.Resize(overlayImage, size, size, imaging.Lanczos)
	finalImg := imaging.Overlay(backgroundImg, resizedOverlayImg, image.Pt(x, y), 1)
	overlayWoxImg, overlayWoxImgErr := NewWoxImage(finalImg)
	if overlayWoxImgErr != nil {
		return *w
	}

	return overlayWoxImg
}

func (w *WoxImage) OverlayFullPercentage(overlay WoxImage, percentage float64) WoxImage {
	backgroundImg, backErr := w.ToImage()
	if backErr != nil {
		return *w
	}

	overlayImage, overlayErr := overlay.ToImage()
	if overlayErr != nil {
		return *w
	}

	width := int(float64(backgroundImg.Bounds().Dx()) * percentage)
	height := int(float64(backgroundImg.Bounds().Dy()) * percentage)
	pt := image.Pt((backgroundImg.Bounds().Dx()-width)/2, (backgroundImg.Bounds().Dy()-height)/2)

	resizedOverlayImg := imaging.Resize(overlayImage, width, height, imaging.Lanczos)
	finalImg := imaging.Overlay(backgroundImg, resizedOverlayImg, pt, 1)
	overlayWoxImg, overlayWoxImgErr := NewWoxImage(finalImg)
	if overlayWoxImgErr != nil {
		return *w
	}

	return overlayWoxImg
}

func (w *WoxImage) IsGif() bool {
	return strings.HasSuffix(w.ImageData, ".gif")
}

func NewWoxImageSvg(svg string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeSvg,
		ImageData: svg,
	}
}

func NewWoxImageAbsolutePath(path string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeAbsolutePath,
		ImageData: path,
	}
}

func NewWoxImageFileIcon(filePath string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeFileIcon,
		ImageData: filePath,
	}
}

func NewWoxImageBase64(data string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeBase64,
		ImageData: data,
	}
}

func NewWoxImage(image image.Image) (WoxImage, error) {
	buf := new(bytes.Buffer)
	encodeErr := fastPngEncoder.Encode(buf, image)
	if encodeErr != nil {
		return WoxImage{}, fmt.Errorf("failed to encode image: %s", encodeErr.Error())
	}

	return NewWoxImageBase64(fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(buf.Bytes()))), nil
}

func NewWoxImageUrl(url string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeUrl,
		ImageData: url,
	}
}

func NewWoxImageEmoji(emoji string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeEmoji,
		ImageData: emoji,
	}
}

func NewWoxImageLottie(lottieJson string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeLottie,
		ImageData: lottieJson,
	}
}

func NewWoxImageTheme(theme Theme) WoxImage {
	themeJson, err := json.Marshal(theme)
	if err != nil {
		return WoxImage{}
	}

	return WoxImage{
		ImageType: WoxImageTypeTheme,
		ImageData: string(themeJson),
	}
}

func NewWoxImageLazyLoad(token string, placeholder WoxImage, targetSize int) WoxImage {
	// LazyLoad is an internal image type: core serializes the placeholder and
	// token together so UI can render immediately, then ask core for the
	// resized raster only when the image widget is built.
	payload, _ := json.Marshal(WoxLazyLoadImagePayload{
		Token:       token,
		Placeholder: placeholder,
		TargetSize:  targetSize,
	})
	return WoxImage{
		ImageType: WoxImageTypeLazyLoad,
		ImageData: string(payload),
	}
}

func NewWoxImageLazyLoadCandidate(source WoxImage, targetSize int) WoxImage {
	// Candidate lazy images are returned only inside core while polishing results.
	// The manager replaces this source-bearing marker with a token-bearing
	// lazyloadimage after it has registered the result in its cache.
	payload, _ := json.Marshal(WoxLazyLoadImagePayload{
		Placeholder: ImageThumbnailPlaceholderIcon,
		TargetSize:  targetSize,
		Source:      &source,
	})
	return WoxImage{
		ImageType: WoxImageTypeLazyLoad,
		ImageData: string(payload),
	}
}

func ParseWoxLazyLoadImagePayload(image WoxImage) (WoxLazyLoadImagePayload, error) {
	if image.ImageType != WoxImageTypeLazyLoad {
		return WoxLazyLoadImagePayload{}, fmt.Errorf("image type is not lazyloadimage: %s", image.ImageType)
	}

	var payload WoxLazyLoadImagePayload
	if err := json.Unmarshal([]byte(image.ImageData), &payload); err != nil {
		return WoxLazyLoadImagePayload{}, err
	}
	return payload, nil
}

func ParseWoxImageOrDefault(image string, defaultImage WoxImage) WoxImage {
	if image == "" {
		return defaultImage
	}

	parsedImage, parseErr := ParseWoxImage(image)
	if parseErr != nil {
		return defaultImage
	}

	return parsedImage
}

func ParseWoxImage(image string) (WoxImage, error) {
	n := strings.SplitN(image, ":", 2)
	if len(n) != 2 {
		return WoxImage{}, fmt.Errorf("invalid image format: %s", image)
	}

	imageType := n[0]
	imageData := n[1]

	if imageType == WoxImageTypeAbsolutePath {
		return NewWoxImageAbsolutePath(imageData), nil
	}
	if imageType == WoxImageTypeRelativePath {
		return WoxImage{
			ImageType: WoxImageTypeRelativePath,
			ImageData: imageData,
		}, nil
	}
	if imageType == WoxImageTypeBase64 {
		return NewWoxImageBase64(imageData), nil
	}
	if imageType == WoxImageTypeSvg {
		return NewWoxImageSvg(imageData), nil
	}
	if imageType == WoxImageTypeUrl {
		return NewWoxImageUrl(imageData), nil
	}
	if imageType == WoxImageTypeEmoji {
		return NewWoxImageEmoji(imageData), nil
	}
	if imageType == WoxImageTypeLottie {
		return NewWoxImageLottie(imageData), nil
	}
	if imageType == WoxImageTypeFileIcon {
		return NewWoxImageFileIcon(imageData), nil
	}
	if imageType == WoxImageTypeLazyLoad {
		return WoxImage{ImageType: WoxImageTypeLazyLoad, ImageData: imageData}, nil
	}

	return WoxImage{}, fmt.Errorf("unsupported image type: %s", imageType)
}

func ConvertIcon(ctx context.Context, image WoxImage, pluginDirectory string) (newImage WoxImage) {
	return ConvertIconWithSize(ctx, image, pluginDirectory, ResultListIconSize)
}

func ConvertIconWithSize(ctx context.Context, image WoxImage, pluginDirectory string, size int) (newImage WoxImage) {
	return convertIconWithSize(ctx, image, pluginDirectory, size, false, timetracking.IconConversionDiagnostics{})
}

// ConvertIconWithSizeWithDiagnostics converts an icon and emits query-scoped timing when diagnostic metadata is provided.
func ConvertIconWithSizeWithDiagnostics(ctx context.Context, image WoxImage, pluginDirectory string, size int, diagnostics timetracking.IconConversionDiagnostics) (newImage WoxImage) {
	return convertIconWithSize(ctx, image, pluginDirectory, size, false, diagnostics)
}

// Converted icons can be large and expensive to prepare, so this variant allows the manager to return a lazy load marker for large icons instead of blocking on conversion.
// The manager replaces the marker with the real resized icon later after it has registered the result in its cache and received the surface size from UI.
func ConvertIconWithSizeMaybeLazy(ctx context.Context, image WoxImage, pluginDirectory string, size int) (newImage WoxImage) {
	return convertIconWithSize(ctx, image, pluginDirectory, size, true, timetracking.IconConversionDiagnostics{})
}

// ConvertIconWithSizeMaybeLazyWithDiagnostics records timing for lazy-capable query result icon conversion.
func ConvertIconWithSizeMaybeLazyWithDiagnostics(ctx context.Context, image WoxImage, pluginDirectory string, size int, diagnostics timetracking.IconConversionDiagnostics) (newImage WoxImage) {
	return convertIconWithSize(ctx, image, pluginDirectory, size, true, diagnostics)
}

func logIconConversionDiagnostics(ctx context.Context, diagnostics timetracking.IconConversionDiagnostics, timing timetracking.IconConversionTiming, size int, allowLazy bool, inputType WoxImageType, inputDataLen int) {
	if !diagnostics.Enabled() {
		return
	}
	if diagnostics.Recorder != nil {
		diagnostics.Recorder(diagnostics, timing.Summary())
		return
	}
	if timing.TotalCost == 0 && timing.TotalCostUs < 100 {
		return
	}

	tracker := timetracking.New("icon_convert_done")
	if !tracker.Enabled() {
		return
	}

	tracker.SetRawString("queryId", diagnostics.QueryId)
	tracker.SetRawString("purpose", diagnostics.Purpose)
	tracker.SetRawString("plugin", diagnostics.Plugin)
	tracker.SetRawString("resultId", diagnostics.ResultId)
	tracker.SetString("title", diagnostics.ResultTitle)
	tracker.SetInt64("totalMs", timing.TotalCost)
	tracker.SetInt64("totalUs", timing.TotalCostUs)
	tracker.SetInt("size", size)
	tracker.SetBool("allowLazy", allowLazy)
	tracker.SetRawString("inputType", string(inputType))
	tracker.SetInt("inputDataLen", inputDataLen)
	tracker.SetRawString("normalizedType", string(timing.NormalizedType))
	tracker.SetRawString("outputType", string(timing.OutputType))
	tracker.SetInt("outputDataLen", timing.OutputDataLen)
	tracker.SetInt64("fileIconMs", timing.FileIconCost)
	tracker.SetInt64("fileIconUs", timing.FileIconCostUs)
	tracker.SetInt64("relativeMs", timing.RelativeCost)
	tracker.SetInt64("relativeUs", timing.RelativeCostUs)
	tracker.SetInt64("svgCheckMs", timing.SvgCheckCost)
	tracker.SetInt64("svgCheckUs", timing.SvgCheckCostUs)
	tracker.SetInt64("cacheMs", timing.CacheCost)
	tracker.SetInt64("cacheUs", timing.CacheCostUs)
	tracker.SetBool("cacheHit", timing.CacheHit)
	tracker.SetRawString("cacheSource", timing.CacheSource)
	tracker.SetInt64("lazyCheckMs", timing.LazyCheckCost)
	tracker.SetInt64("lazyCheckUs", timing.LazyCheckCostUs)
	tracker.SetBool("lazy", timing.Lazy)
	tracker.SetRawString("lazyReason", timing.LazyReason)
	tracker.SetInt("lazyWidth", timing.LazyWidth)
	tracker.SetInt("lazyHeight", timing.LazyHeight)
	tracker.SetInt64("cropMs", timing.CropCost)
	tracker.SetInt64("cropUs", timing.CropCostUs)
	tracker.SetRawString("cropResult", timing.CropTiming.Result)
	tracker.SetRawString("cropCache", timing.CropTiming.CacheSource)
	tracker.SetInt64("cropMetadataMs", timing.CropTiming.MetadataMs)
	tracker.SetInt64("cropDecodeMs", timing.CropTiming.DecodeMs)
	tracker.SetInt64("cropScanMs", timing.CropTiming.CropMs)
	tracker.SetInt64("cropSaveMs", timing.CropTiming.SaveMs)
	tracker.SetInt64("resizeMs", timing.ResizeCost)
	tracker.SetInt64("resizeUs", timing.ResizeCostUs)
	tracker.SetRawString("resizeResult", timing.ResizeTiming.Result)
	tracker.SetRawString("resizeCache", timing.ResizeTiming.CacheSource)
	tracker.SetInt64("resizeDecodeMs", timing.ResizeTiming.DecodeMs)
	tracker.SetInt64("resizeOpMs", timing.ResizeTiming.ResizeMs)
	tracker.SetInt64("resizeSaveMs", timing.ResizeTiming.SaveMs)
	tracker.SetInt("resizeSourceWidth", timing.ResizeTiming.SourceWidth)
	tracker.SetInt("resizeSourceHeight", timing.ResizeTiming.SourceHeight)
	tracker.SetInt("resizeTargetSize", timing.ResizeTiming.TargetSize)
	tracker.Log(ctx)
}

func convertIconWithSize(ctx context.Context, image WoxImage, pluginDirectory string, size int, allowLazy bool, diagnostics timetracking.IconConversionDiagnostics) (newImage WoxImage) {
	if !diagnostics.Enabled() || !util.IsDev() {
		return convertIconWithSizeFast(ctx, image, pluginDirectory, size, allowLazy)
	}

	convertStart := util.GetSystemTimestamp()
	convertTimingStart := time.Now()
	inputType := image.ImageType
	inputDataLen := len(image.ImageData)
	timing := timetracking.IconConversionTiming{
		CacheSource: "not_checked",
		LazyReason:  "not_checked",
		CropTiming: timetracking.IconCropTiming{
			CacheSource: "not_checked",
			Result:      "not_checked",
		},
		ResizeTiming: timetracking.IconResizeTiming{
			CacheSource: "not_checked",
			Result:      "not_checked",
		},
	}
	// Result icon callers can choose the surface size directly. Keep invalid
	// sizes on the normal list path so every icon cache layer shares one default.
	if size <= 0 {
		size = ResultListIconSize
	}

	fastCacheStart := util.GetSystemTimestamp()
	fastCacheTimingStart := time.Now()
	if isFinalResizeCacheImage(image, size) {
		imagecache.Touch(ctx, image.ImageData, nil)
		timing.CacheCost = util.GetSystemTimestamp() - fastCacheStart
		timing.CacheCostUs = time.Since(fastCacheTimingStart).Microseconds()
		timing.CacheHit = true
		timing.CacheSource = "already_resized"
		timing.NormalizedType = image.ImageType
		timing.TotalCost = util.GetSystemTimestamp() - convertStart
		timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
		timing.OutputType = image.ImageType
		timing.OutputDataLen = len(image.ImageData)
		logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
		return image
	}

	fileIconStart := util.GetSystemTimestamp()
	fileIconTimingStart := time.Now()
	newImage = ConvertFileIconToAbsolutePathWithSize(ctx, image, size)
	timing.FileIconCost = util.GetSystemTimestamp() - fileIconStart
	timing.FileIconCostUs = time.Since(fileIconTimingStart).Microseconds()
	relativeStart := util.GetSystemTimestamp()
	relativeTimingStart := time.Now()
	newImage = ConvertRelativePathToAbsolutePath(ctx, newImage, pluginDirectory)
	timing.RelativeCost = util.GetSystemTimestamp() - relativeStart
	timing.RelativeCostUs = time.Since(relativeTimingStart).Microseconds()
	timing.NormalizedType = newImage.ImageType

	// Keep SVG data and SVG files as-is so UI can render vectors directly.
	svgCheckStart := util.GetSystemTimestamp()
	svgCheckTimingStart := time.Now()
	if newImage.ImageType == WoxImageTypeSvg || (newImage.ImageType == WoxImageTypeAbsolutePath && isSvgFilePath(newImage.ImageData)) {
		timing.SvgCheckCost = util.GetSystemTimestamp() - svgCheckStart
		timing.SvgCheckCostUs = time.Since(svgCheckTimingStart).Microseconds()
		timing.TotalCost = util.GetSystemTimestamp() - convertStart
		timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
		timing.OutputType = newImage.ImageType
		timing.OutputDataLen = len(newImage.ImageData)
		logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
		return newImage
	}
	timing.SvgCheckCost = util.GetSystemTimestamp() - svgCheckStart
	timing.SvgCheckCostUs = time.Since(svgCheckTimingStart).Microseconds()

	cacheStart := util.GetSystemTimestamp()
	cacheTimingStart := time.Now()
	cached, ok, cacheSource := cachedResizeImageDetailed(ctx, newImage, size)
	timing.CacheCost = util.GetSystemTimestamp() - cacheStart
	timing.CacheCostUs = time.Since(cacheTimingStart).Microseconds()
	timing.CacheHit = ok
	timing.CacheSource = cacheSource
	if ok {
		timing.TotalCost = util.GetSystemTimestamp() - convertStart
		timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
		timing.OutputType = cached.ImageType
		timing.OutputDataLen = len(cached.ImageData)
		logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
		return cached
	}

	croppedCacheStart := util.GetSystemTimestamp()
	croppedCacheTimingStart := time.Now()
	croppedCached, croppedOk, croppedCacheSource := cachedCroppedResizeImageDetailed(ctx, newImage, size)
	timing.CacheCost += util.GetSystemTimestamp() - croppedCacheStart
	timing.CacheCostUs += time.Since(croppedCacheTimingStart).Microseconds()
	if croppedOk {
		timing.CacheHit = true
		timing.CacheSource = croppedCacheSource
		timing.TotalCost = util.GetSystemTimestamp() - convertStart
		timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
		timing.OutputType = croppedCached.ImageType
		timing.OutputDataLen = len(croppedCached.ImageData)
		logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
		return croppedCached
	}

	lazyCheckStart := util.GetSystemTimestamp()
	lazyCheckTimingStart := time.Now()
	lazy, lazyReason, lazyWidth, lazyHeight := shouldLazyLoadImageIconDetailed(ctx, newImage, size)
	timing.LazyCheckCost = util.GetSystemTimestamp() - lazyCheckStart
	timing.LazyCheckCostUs = time.Since(lazyCheckTimingStart).Microseconds()
	timing.Lazy = lazy
	timing.LazyReason = lazyReason
	timing.LazyWidth = lazyWidth
	timing.LazyHeight = lazyHeight
	if allowLazy && lazy {
		// Optimization: large local raster icons are expensive because the old
		// polish path decoded, optionally cropped, resized, and wrote every image
		// before the query response reached UI. Return a source-bearing marker
		// here and let the manager decide whether to register it as a token, which
		// keeps cache ownership out of common image conversion.
		lazyImage := NewWoxImageLazyLoadCandidate(newImage, size)
		timing.TotalCost = util.GetSystemTimestamp() - convertStart
		timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
		timing.OutputType = lazyImage.ImageType
		timing.OutputDataLen = len(lazyImage.ImageData)
		logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
		return lazyImage
	}

	cropStart := util.GetSystemTimestamp()
	cropTimingStart := time.Now()
	newImage, timing.CropTiming = cropPngTransparentPaddingsWithTiming(ctx, newImage)
	timing.CropCost = util.GetSystemTimestamp() - cropStart
	timing.CropCostUs = time.Since(cropTimingStart).Microseconds()
	resizeStart := util.GetSystemTimestamp()
	resizeTimingStart := time.Now()
	newImage, timing.ResizeTiming = resizeImageWithTiming(ctx, newImage, size)
	timing.ResizeCost = util.GetSystemTimestamp() - resizeStart
	timing.ResizeCostUs = time.Since(resizeTimingStart).Microseconds()
	timing.TotalCost = util.GetSystemTimestamp() - convertStart
	timing.TotalCostUs = time.Since(convertTimingStart).Microseconds()
	timing.OutputType = newImage.ImageType
	timing.OutputDataLen = len(newImage.ImageData)
	logIconConversionDiagnostics(ctx, diagnostics, timing, size, allowLazy, inputType, inputDataLen)
	return
}

// convertIconWithSizeFast keeps the normal icon path free from diagnostic timers.
func convertIconWithSizeFast(ctx context.Context, image WoxImage, pluginDirectory string, size int, allowLazy bool) (newImage WoxImage) {
	if size <= 0 {
		size = ResultListIconSize
	}

	if isFinalResizeCacheImage(image, size) {
		imagecache.Touch(ctx, image.ImageData, nil)
		return image
	}

	newImage = ConvertFileIconToAbsolutePathWithSize(ctx, image, size)
	newImage = ConvertRelativePathToAbsolutePath(ctx, newImage, pluginDirectory)
	if newImage.ImageType == WoxImageTypeSvg || (newImage.ImageType == WoxImageTypeAbsolutePath && isSvgFilePath(newImage.ImageData)) {
		return newImage
	}

	if cached, ok := cachedResizeImage(ctx, newImage, size); ok {
		return cached
	}
	if cached, ok, _ := cachedCroppedResizeImageDetailed(ctx, newImage, size); ok {
		return cached
	}
	if allowLazy && shouldLazyLoadImageIcon(ctx, newImage, size) {
		return NewWoxImageLazyLoadCandidate(newImage, size)
	}

	newImage = cropPngTransparentPaddings(ctx, newImage)
	newImage = resizeImage(ctx, newImage, size)
	return
}

func shouldLazyLoadImageIcon(ctx context.Context, woxImage WoxImage, size int) bool {
	lazy, _, _, _ := shouldLazyLoadImageIconDetailed(ctx, woxImage, size)
	return lazy
}

func shouldLazyLoadImageIconDetailed(ctx context.Context, woxImage WoxImage, size int) (bool, string, int, int) {
	if woxImage.ImageType != WoxImageTypeAbsolutePath || woxImage.IsGif() || isSvgFilePath(woxImage.ImageData) {
		return false, "not_absolute_raster", 0, 0
	}

	file, err := os.Open(woxImage.ImageData)
	if err != nil {
		return false, "open_error", 0, 0
	}
	defer file.Close()

	config, _, err := image.DecodeConfig(file)
	if err != nil {
		// Keep unknown image shapes on the existing synchronous path. This favors
		// compatibility for uncommon formats and lets the surrounding slow-query
		// logs reveal any future formats that need a dedicated lazy rule.
		util.GetLogger().Debug(ctx, fmt.Sprintf("failed to decode result icon config for lazy decision: %s", err.Error()))
		return false, "decode_error", 0, 0
	}

	if max(config.Width, config.Height) > 512 {
		return true, "large_raster", config.Width, config.Height
	}
	return false, "small_raster", config.Width, config.Height
}

func resizeImageCachePath(image WoxImage, size int) string {
	imgHash := image.Hash()
	return path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("%s%d_%s.png", resizeImageCachePrefix, size, imgHash))
}

// isFinalResizeCacheImage checks whether the image is already the final resized cache artifact for the requested surface size.
func isFinalResizeCacheImage(image WoxImage, size int) bool {
	if image.ImageType != WoxImageTypeAbsolutePath || image.ImageData == "" {
		return false
	}

	imagePath := filepath.Clean(image.ImageData)
	cacheDir := filepath.Clean(util.GetLocation().GetImageCacheDirectory())
	if !strings.EqualFold(filepath.Dir(imagePath), cacheDir) {
		return false
	}

	filename := filepath.Base(imagePath)
	return strings.HasPrefix(filename, fmt.Sprintf("%s%d_", resizeImageCachePrefix, size)) && strings.EqualFold(filepath.Ext(filename), ".png")
}

func cachedResizeImage(ctx context.Context, image WoxImage, size int) (WoxImage, bool) {
	woxImage, ok, _ := cachedResizeImageDetailed(ctx, image, size)
	return woxImage, ok
}

func cachedResizeImageDetailed(ctx context.Context, image WoxImage, size int) (WoxImage, bool, string) {
	resizeImgPath := resizeImageCachePath(image, size)
	if imagecache.IsKnownExistingDerivedPath(resizeImgPath) {
		imagecache.Touch(ctx, resizeImgPath, nil)
		return NewWoxImageAbsolutePath(resizeImgPath), true, "memory"
	}
	if info, err := os.Stat(resizeImgPath); err == nil {
		imagecache.RememberDerivedPathExists(resizeImgPath)
		imagecache.Touch(ctx, resizeImgPath, info)
		return NewWoxImageAbsolutePath(resizeImgPath), true, "stat"
	}
	return WoxImage{}, false, "miss"
}

// cachedCroppedResizeImageDetailed returns the final crop+resize cache entry
// before the lazy-size probe opens the original image. Warm app icons commonly
// have both derived files already, so this avoids repeated DecodeConfig and stat
// work on the query polish path.
func cachedCroppedResizeImageDetailed(ctx context.Context, image WoxImage, size int) (WoxImage, bool, string) {
	if image.ImageType == WoxImageTypeEmoji || image.IsGif() {
		return WoxImage{}, false, "skipped"
	}

	cropImgPath := cropPngTransparentPaddingCachePath(image)
	cropCacheSource := ""
	if imagecache.IsKnownExistingDerivedPath(cropImgPath) {
		imagecache.Touch(ctx, cropImgPath, nil)
		cropCacheSource = "memory"
	} else if info, err := os.Stat(cropImgPath); err == nil {
		imagecache.RememberDerivedPathExists(cropImgPath)
		imagecache.Touch(ctx, cropImgPath, info)
		cropCacheSource = "stat"
	} else {
		return WoxImage{}, false, "miss"
	}

	croppedImage := NewWoxImageAbsolutePath(cropImgPath)
	if resized, ok, resizeCacheSource := cachedResizeImageDetailed(ctx, croppedImage, size); ok {
		return resized, true, fmt.Sprintf("cropped_%s_%s", cropCacheSource, resizeCacheSource)
	}

	return WoxImage{}, false, fmt.Sprintf("cropped_%s_resize_miss", cropCacheSource)
}

func resizeImage(ctx context.Context, image WoxImage, size int) (newImage WoxImage) {
	newImage, _ = resizeImageWithTiming(ctx, image, size)
	return newImage
}

func resizeImageWithTiming(ctx context.Context, image WoxImage, size int) (newImage WoxImage, timing timetracking.IconResizeTiming) {
	timing.CacheSource = "not_checked"
	timing.Result = "unknown"

	// skip emoji images
	if image.ImageType == WoxImageTypeEmoji {
		timing.Result = "skipped_emoji"
		return image, timing
	}
	if image.IsGif() {
		timing.Result = "skipped_gif"
		return image, timing
	}

	newImage = image

	resizeImgPath := resizeImageCachePath(image, size)
	cacheStart := util.GetSystemTimestamp()
	if cached, ok, cacheSource := cachedResizeImageDetailed(ctx, image, size); ok {
		timing.CacheSource = cacheSource
		timing.Result = "cache_hit"
		timing.DecodeMs = util.GetSystemTimestamp() - cacheStart
		return cached, timing
	}
	timing.CacheSource = "miss"
	timing.DecodeMs = util.GetSystemTimestamp() - cacheStart

	decodeStart := util.GetSystemTimestamp()
	img, imgErr := image.ToImage()
	timing.DecodeMs = util.GetSystemTimestamp() - decodeStart
	if imgErr != nil {
		timing.Result = "decode_error"
		return image, timing
	}

	// Respect the original ratio and never enlarge the source. Upscaling a small
	// native app icon before a grid surface downsampled it made large result icons
	// visibly soft.
	sourceWidth := img.Bounds().Dx()
	sourceHeight := img.Bounds().Dy()
	targetSize := min(size, max(sourceWidth, sourceHeight))
	timing.SourceWidth = sourceWidth
	timing.SourceHeight = sourceHeight
	timing.TargetSize = targetSize
	width := targetSize
	height := targetSize
	if sourceWidth > sourceHeight {
		height = 0
	} else {
		width = 0
	}

	resizeFilter := imaging.Lanczos
	resizeStart := util.GetSystemTimestamp()
	resizeImg := imaging.Resize(img, width, height, resizeFilter)
	timing.ResizeMs = util.GetSystemTimestamp() - resizeStart
	saveStart := util.GetSystemTimestamp()
	saveErr := savePngFast(resizeImg, resizeImgPath)
	timing.SaveMs = util.GetSystemTimestamp() - saveStart
	if saveErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to save resize image: %s", saveErr.Error()))
		timing.Result = "save_error"
		return image, timing
	}

	imagecache.RememberDerivedPathExists(resizeImgPath)
	timing.Result = "saved"
	return NewWoxImageAbsolutePath(resizeImgPath), timing
}

func cropPngTransparentPaddings(ctx context.Context, woxImage WoxImage) (newImage WoxImage) {
	newImage, _ = cropPngTransparentPaddingsWithTiming(ctx, woxImage)
	return newImage
}

func cropPngTransparentPaddingsWithTiming(ctx context.Context, woxImage WoxImage) (newImage WoxImage, timing timetracking.IconCropTiming) {
	timing.CacheSource = "not_checked"
	timing.Result = "unknown"
	// skip emoji images
	if woxImage.ImageType == WoxImageTypeEmoji {
		timing.Result = "skipped_emoji"
		return woxImage, timing
	}
	if woxImage.IsGif() {
		timing.Result = "skipped_gif"
		return woxImage, timing
	}

	imgHash := woxImage.Hash()
	if isKnownTransparentPaddingBypass(imgHash) {
		timing.CacheSource = "bypass_memory"
		timing.Result = "bypassed"
		return woxImage, timing
	}

	//try load from cache first
	cropImgPath := cropPngTransparentPaddingCachePath(woxImage)
	if imagecache.IsKnownExistingDerivedPath(cropImgPath) {
		imagecache.Touch(ctx, cropImgPath, nil)
		timing.CacheSource = "memory"
		timing.Result = "cache_hit"
		return NewWoxImageAbsolutePath(cropImgPath), timing
	}
	if info, err := os.Stat(cropImgPath); err == nil {
		imagecache.RememberDerivedPathExists(cropImgPath)
		imagecache.Touch(ctx, cropImgPath, info)
		timing.CacheSource = "stat"
		timing.Result = "cache_hit"
		return NewWoxImageAbsolutePath(cropImgPath), timing
	}
	timing.CacheSource = "miss"
	metadataStart := util.GetSystemTimestamp()
	if metadata, ok := absolutePngCropMetadata(woxImage); ok {
		timing.MetadataMs = util.GetSystemTimestamp() - metadataStart
		if metadata.width > pngCropLargeDimension && metadata.height > pngCropLargeDimension {
			// Very large PNGs are content images instead of icon artwork. Cropping them can force a
			// full-size decode and transparent scan with no icon benefit, so keep the original image
			// and let the resize cache own the bounded icon output.
			rememberTransparentPaddingBypass(imgHash)
			timing.Result = "metadata_large_bypass"
			return woxImage, timing
		}
		if !metadata.mayContainTransparency {
			// RGB screenshots cannot have transparent padding, so decoding, scanning every pixel,
			// and writing an equivalent crop file only adds cold-query latency. The metadata-only
			// check keeps uncertain or alpha-capable PNGs on the existing full crop path, and the
			// bypass cache avoids repeating this metadata read on the steady warm path.
			rememberTransparentPaddingBypass(imgHash)
			timing.Result = "metadata_no_alpha_bypass"
			return woxImage, timing
		}
	} else {
		timing.MetadataMs = util.GetSystemTimestamp() - metadataStart
	}

	decodeStart := util.GetSystemTimestamp()
	pngImg, pngErr := woxImage.ToPng()
	timing.DecodeMs = util.GetSystemTimestamp() - decodeStart
	if pngErr != nil {
		if !errors.Is(pngErr, NOT_PNG_ERR) {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to convert image to png: %s", pngErr.Error()))
		}
		timing.Result = "not_png_or_decode_error"
		return woxImage, timing
	}

	cropStart := util.GetSystemTimestamp()
	cropImg := cropTransparentPaddings(pngImg)
	timing.CropMs = util.GetSystemTimestamp() - cropStart
	saveStart := util.GetSystemTimestamp()
	saveErr := savePngFast(cropImg, cropImgPath)
	timing.SaveMs = util.GetSystemTimestamp() - saveStart
	if saveErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to save crop image: %s", saveErr.Error()))
		timing.Result = "save_error"
		return woxImage, timing
	}

	imagecache.RememberDerivedPathExists(cropImgPath)
	timing.Result = "saved"
	return NewWoxImageAbsolutePath(cropImgPath), timing
}

func cropPngTransparentPaddingCachePath(woxImage WoxImage) string {
	return path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("crop_padding_%s.png", woxImage.Hash()))
}

// absolutePngCropMetadata reads only PNG metadata before IDAT. Any malformed,
// unsupported, or non-file image returns ok=false so callers keep the full decode path.
func absolutePngCropMetadata(woxImage WoxImage) (pngCropMetadata, bool) {
	if woxImage.ImageType != WoxImageTypeAbsolutePath || !strings.EqualFold(filepath.Ext(woxImage.ImageData), ".png") {
		return pngCropMetadata{}, false
	}

	file, err := os.Open(woxImage.ImageData)
	if err != nil {
		return pngCropMetadata{}, false
	}
	defer file.Close()

	var signature [8]byte
	if _, err = io.ReadFull(file, signature[:]); err != nil || signature != pngFileSignature {
		return pngCropMetadata{}, false
	}

	seenHeader := false
	metadata := pngCropMetadata{}
	for {
		var chunkHeader [8]byte
		if _, err = io.ReadFull(file, chunkHeader[:]); err != nil {
			return pngCropMetadata{}, false
		}

		chunkLength := binary.BigEndian.Uint32(chunkHeader[0:4])
		chunkType := [4]byte{chunkHeader[4], chunkHeader[5], chunkHeader[6], chunkHeader[7]}
		switch chunkType {
		case pngChunkIHDR:
			if seenHeader || chunkLength != 13 {
				return pngCropMetadata{}, false
			}

			var headerData [13]byte
			if _, err = io.ReadFull(file, headerData[:]); err != nil {
				return pngCropMetadata{}, false
			}
			if _, err = file.Seek(4, io.SeekCurrent); err != nil {
				return pngCropMetadata{}, false
			}

			colorType := headerData[9]
			metadata.width = int(binary.BigEndian.Uint32(headerData[0:4]))
			metadata.height = int(binary.BigEndian.Uint32(headerData[4:8]))
			seenHeader = true
			switch colorType {
			case pngColorTypeGrayscaleAlpha, pngColorTypeTruecolorAlpha:
				metadata.mayContainTransparency = true
				return metadata, true
			case pngColorTypeGrayscale, pngColorTypeTruecolor, pngColorTypeIndexed:
				metadata.mayContainTransparency = false
				continue
			default:
				return pngCropMetadata{}, false
			}
		case pngChunktRNS:
			if !seenHeader {
				return pngCropMetadata{}, false
			}
			metadata.mayContainTransparency = true
			return metadata, true
		case pngChunkIDAT:
			if !seenHeader {
				return pngCropMetadata{}, false
			}
			return metadata, true
		default:
			if !seenHeader {
				return pngCropMetadata{}, false
			}
			if _, err = file.Seek(int64(chunkLength)+4, io.SeekCurrent); err != nil {
				return pngCropMetadata{}, false
			}
		}
	}
}

func cropTransparentPaddings(pngImg image.Image) image.Image {
	bounds := pngImg.Bounds()
	minX, minY, maxX, maxY := bounds.Max.X, bounds.Max.Y, bounds.Min.X, bounds.Min.Y
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := pngImg.At(x, y).RGBA()
			if a != 0 {
				// not transparent
				if x < minX {
					minX = x
				}
				if x+1 > maxX { // add 1 to maxX
					maxX = x + 1
				}
				if y < minY {
					minY = y
				}
				if y+1 > maxY { // add 1 to maxY
					maxY = y + 1
				}
			}
		}
	}
	if maxX > bounds.Max.X {
		maxX = bounds.Max.X
	}
	if maxY > bounds.Max.Y {
		maxY = bounds.Max.Y
	}

	return imaging.Crop(pngImg, image.Rect(minX, minY, maxX, maxY))
}

func ConvertRelativePathToAbsolutePath(ctx context.Context, image WoxImage, pluginDirectory string) (newImage WoxImage) {
	newImage = image

	if image.ImageType == WoxImageTypeRelativePath {
		newImage.ImageType = WoxImageTypeAbsolutePath
		newImage.ImageData = path.Join(pluginDirectory, image.ImageData)
	}

	return newImage
}

func ConvertFileIconToAbsolutePath(ctx context.Context, image WoxImage) (newImage WoxImage) {
	return ConvertFileIconToAbsolutePathWithSize(ctx, image, ResultListIconSize)
}

func ConvertFileIconToAbsolutePathWithSize(ctx context.Context, image WoxImage, size int) (newImage WoxImage) {
	newImage = image

	if image.ImageType == WoxImageTypeFileIcon {
		absPath, err := fileicon.GetFileIconByPathWithSize(ctx, image.ImageData, size)
		if err == nil {
			newImage.ImageType = WoxImageTypeAbsolutePath
			newImage.ImageData = absPath
		}
	}

	return newImage
}

func SetServerPort(port int) {
	serverPort = port
}
