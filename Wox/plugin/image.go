package plugin

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/disintegration/imaging"
	"image"
	"image/png"
	"os"
	"path"
	"strings"
	"wox/util"
)

var localImageMap = util.NewHashMap[string, string]()

type WoxImageType = string

var notPngErr = errors.New("image is not png")

const (
	WoxImageTypeAbsolutePath = "absolute"
	WoxImageTypeRelativePath = "relative"
	WoxImageTypeBase64       = "base64"
	WoxImageTypeSvg          = "svg"
	WoxImageTypeUrl          = "url"
)

type WoxImage struct {
	ImageType WoxImageType
	ImageData string
}

func (w *WoxImage) ToPng() (image.Image, error) {
	if w.ImageType == WoxImageTypeBase64 {
		if !strings.HasPrefix(w.ImageData, "data:image/png;") {
			return nil, notPngErr
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
			return nil, notPngErr
		}

		imgReader, openErr := os.Open(w.ImageData)
		if openErr != nil {
			return nil, openErr
		}
		defer imgReader.Close()
		return png.Decode(imgReader)
	}

	if w.ImageType == WoxImageTypeSvg {
		//TODO: convert svg to png
	}

	return nil, notPngErr
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

	return nil, fmt.Errorf("unsupported image type: %s", w.ImageType)
}

func (w *WoxImage) Hash() string {
	return util.Md5([]byte(w.ImageType + w.ImageData))
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

func ConvertIcon(ctx context.Context, image WoxImage, pluginDirectory string) (newImage WoxImage) {
	newImage = convertRelativePathToAbsolutePath(ctx, image, pluginDirectory)
	newImage = cropPngTransparentPaddings(ctx, newImage)
	newImage = resizeImage(ctx, newImage, 40)
	newImage = convertLocalImageToUrl(ctx, newImage)
	return
}

func resizeImage(ctx context.Context, image WoxImage, size int) (newImage WoxImage) {
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
		logger.Error(ctx, fmt.Sprintf("failed to save resize image: %s", saveErr.Error()))
		return image
	} else {
		logger.Info(ctx, fmt.Sprintf("saved resize image: %s, cost %d ms", resizeImgPath, util.GetSystemTimestamp()-start))
	}

	return NewWoxImageAbsolutePath(resizeImgPath)
}

func cropPngTransparentPaddings(ctx context.Context, woxImage WoxImage) (newImage WoxImage) {
	//try load from cache first
	imgHash := woxImage.Hash()
	cropImgPath := path.Join(util.GetLocation().GetImageCacheDirectory(), fmt.Sprintf("crop_padding_%s.png", imgHash))
	if _, err := os.Stat(cropImgPath); err == nil {
		return NewWoxImageAbsolutePath(cropImgPath)
	}

	pngImg, pngErr := woxImage.ToPng()
	if pngErr != nil {
		if !errors.Is(pngErr, notPngErr) {
			logger.Error(ctx, fmt.Sprintf("failed to convert image to png: %s", pngErr.Error()))
		}
		return woxImage
	}

	start := util.GetSystemTimestamp()
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
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	cropImg := imaging.Crop(pngImg, image.Rect(minX, minY, maxX, maxY))
	saveErr := imaging.Save(cropImg, cropImgPath)
	if saveErr != nil {
		logger.Error(ctx, fmt.Sprintf("failed to save crop image: %s", saveErr.Error()))
		return woxImage
	} else {
		logger.Info(ctx, fmt.Sprintf("saved crop image: %s, cost %d ms", cropImgPath, util.GetSystemTimestamp()-start))
	}

	return NewWoxImageAbsolutePath(cropImgPath)
}

func convertRelativePathToAbsolutePath(ctx context.Context, image WoxImage, pluginDirectory string) (newImage WoxImage) {
	newImage = image

	if image.ImageType == WoxImageTypeRelativePath {
		newImage.ImageType = WoxImageTypeAbsolutePath
		newImage.ImageData = path.Join(pluginDirectory, image.ImageData)
	}

	return newImage
}

func convertLocalImageToUrl(ctx context.Context, image WoxImage) (newImage WoxImage) {
	newImage = image

	if image.ImageType == WoxImageTypeAbsolutePath {
		imgHash := image.Hash()
		newImage.ImageType = WoxImageTypeUrl
		newImage.ImageData = fmt.Sprintf("http://localhost:%d/image?id=%s", GetPluginManager().GetUI().GetServerPort(ctx), imgHash)
		localImageMap.Store(imgHash, image.ImageData)
	}

	return newImage
}

func GetLocalImageMap(id string) (string, bool) {
	return localImageMap.Load(id)
}
