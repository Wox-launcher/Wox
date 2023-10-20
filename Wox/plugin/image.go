package plugin

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"path"
	"wox/util"
)

var localImageMap util.HashMap[string, string]

type WoxImageType = string

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

func convertLocalImageToUrl(ctx context.Context, image WoxImage, pluginInstance *Instance) (newImage WoxImage) {
	newImage = image

	if image.ImageType == WoxImageTypeAbsolutePath {
		id := uuid.NewString()
		newImage.ImageType = WoxImageTypeUrl
		newImage.ImageData = fmt.Sprintf("http://localhost:%d/image?id=%s", GetPluginManager().GetUI().GetServerPort(ctx), id)
		localImageMap.Store(id, image.ImageData)
	}
	if image.ImageType == WoxImageTypeRelativePath {
		id := uuid.NewString()
		newImage.ImageType = WoxImageTypeUrl
		newImage.ImageData = fmt.Sprintf("http://localhost:%d/image?id=%s", GetPluginManager().GetUI().GetServerPort(ctx), id)
		localImageMap.Store(id, path.Join(pluginInstance.PluginDirectory, image.ImageData))
	}

	return newImage
}

func GetLocalImageMap(id string) (string, bool) {
	return localImageMap.Load(id)
}
