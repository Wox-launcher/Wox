package common

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path"
	"strings"
	"wox/util"

	"github.com/disintegration/imaging"
	"github.com/forPelevin/gomoji"
	"github.com/srwiley/oksvg"
	"github.com/srwiley/rasterx"
)

type WoxImageType = string

var NOT_PNG_ERR = errors.New("image is not png")

var serverPort int

const (
	WoxImageTypeAbsolutePath = "absolute"
	WoxImageTypeRelativePath = "relative"
	WoxImageTypeBase64       = "base64"
	WoxImageTypeSvg          = "svg"
	WoxImageTypeLottie       = "lottie" // only support lottie json data
	WoxImageTypeEmoji        = "emoji"
	WoxImageTypeUrl          = "url"
	WoxImageTypeTheme        = "theme"
)

type WoxImage struct {
	ImageType WoxImageType
	ImageData string
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
		if !strings.HasSuffix(w.ImageData, ".png") {
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

		buf := new(bytes.Buffer)
		encodeErr := png.Encode(buf, img)
		if encodeErr != nil {
			return nil, encodeErr
		}
	}

	return nil, NOT_PNG_ERR
}

func (w *WoxImage) ToImage() (image.Image, error) {
	if w.ImageType == WoxImageTypeAbsolutePath {
		return imaging.Open(w.ImageData)
	}
	if w.ImageType == WoxImageTypeBase64 {
		data := strings.Split(w.ImageData, ",")[1]
		decodedData, base64DecodeErr := base64.StdEncoding.DecodeString(data)
		if base64DecodeErr != nil {
			return nil, base64DecodeErr
		}

		imgReader := bytes.NewReader(decodedData)
		return png.Decode(imgReader)
	}
	if w.ImageType == WoxImageTypeSvg {
		width, height := 32, 32
		icon, err := oksvg.ReadIconStream(strings.NewReader(w.ImageData), oksvg.WarnErrorMode)
		if err != nil {
			return nil, err
		}
		icon.SetTarget(0, 0, float64(width), float64(height))

		rgba := image.NewRGBA(image.Rect(0, 0, width, height))
		icon.Draw(rasterx.NewDasher(width, height, rasterx.NewScannerGV(width, height, rgba, rgba.Bounds())), 1)
		//finalImg := cropTransparentPaddings(rgba)
		return rgba, nil
	}
	if w.ImageType == WoxImageTypeEmoji {
		emojiInfo, getErr := gomoji.GetInfo(w.ImageData)
		if getErr != nil {
			return nil, getErr
		}

		// load from cache first
		codePoint := strings.ToLower(emojiInfo.CodePoint)
		emojiPath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("emoji_%s.png", codePoint))
		if _, err := os.Stat(emojiPath); err == nil {
			return imaging.Open(emojiPath)
		}

		//download emoji image and cache it
		url := fmt.Sprintf("https://cdn.jsdelivr.net/gh/twitter/twemoji@v11.0.0/36x36/%s.png", codePoint)
		err := util.HttpDownload(util.NewTraceContext(), url, emojiPath)
		if err != nil {
			return nil, err
		}

		return imaging.Open(emojiPath)
	}

	return nil, fmt.Errorf("unsupported image type: %s", w.ImageType)
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

func NewWoxImageBase64(data string) WoxImage {
	return WoxImage{
		ImageType: WoxImageTypeBase64,
		ImageData: data,
	}
}

func NewWoxImage(image image.Image) (WoxImage, error) {
	buf := new(bytes.Buffer)
	encodeErr := png.Encode(buf, image)
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

	return WoxImage{}, fmt.Errorf("unsupported image type: %s", imageType)
}

func ConvertIcon(ctx context.Context, image WoxImage, pluginDirectory string) (newImage WoxImage) {
	newImage = ConvertRelativePathToAbsolutePath(ctx, image, pluginDirectory)
	newImage = cropPngTransparentPaddings(ctx, newImage)
	newImage = resizeImage(ctx, newImage, 40)
	return
}

func resizeImage(ctx context.Context, image WoxImage, size int) (newImage WoxImage) {
	// skip emoji images
	if image.ImageType == WoxImageTypeEmoji {
		return image
	}
	if image.IsGif() {
		return image
	}

	newImage = image

	imgHash := image.Hash()
	resizeImgPath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("resize_%d_%s.png", size, imgHash))
	if _, err := os.Stat(resizeImgPath); err == nil {
		return NewWoxImageAbsolutePath(resizeImgPath)
	}

	img, imgErr := image.ToImage()
	if imgErr != nil {
		return image
	}

	// respect ratio, remain longer side
	width := size
	height := size
	if img.Bounds().Dx() > img.Bounds().Dy() {
		height = 0
	} else {
		width = 0
	}

	start := util.GetSystemTimestamp()
	resizeImg := imaging.Resize(img, width, height, imaging.Lanczos)
	saveErr := imaging.Save(resizeImg, resizeImgPath)
	if saveErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to save resize image: %s", saveErr.Error()))
		return image
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("saved resize image: %s, cost %d ms", resizeImgPath, util.GetSystemTimestamp()-start))
	}

	return NewWoxImageAbsolutePath(resizeImgPath)
}

func cropPngTransparentPaddings(ctx context.Context, woxImage WoxImage) (newImage WoxImage) {
	// skip emoji images
	if woxImage.ImageType == WoxImageTypeEmoji {
		return woxImage
	}
	if woxImage.IsGif() {
		return woxImage
	}

	//try load from cache first
	imgHash := woxImage.Hash()
	cropImgPath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("crop_padding_%s.png", imgHash))
	if _, err := os.Stat(cropImgPath); err == nil {
		return NewWoxImageAbsolutePath(cropImgPath)
	}

	pngImg, pngErr := woxImage.ToPng()
	if pngErr != nil {
		if !errors.Is(pngErr, NOT_PNG_ERR) {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to convert image to png: %s", pngErr.Error()))
		}
		return woxImage
	}

	start := util.GetSystemTimestamp()
	cropImg := cropTransparentPaddings(pngImg)
	saveErr := imaging.Save(cropImg, cropImgPath)
	if saveErr != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("failed to save crop image: %s", saveErr.Error()))
		return woxImage
	} else {
		util.GetLogger().Info(ctx, fmt.Sprintf("saved crop image: %s, cost %d ms", cropImgPath, util.GetSystemTimestamp()-start))
	}

	return NewWoxImageAbsolutePath(cropImgPath)
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

func SetServerPort(port int) {
	serverPort = port
}
