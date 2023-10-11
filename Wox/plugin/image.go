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

func convertLocalImageToUrl(ctx context.Context, image WoxImage, pluginInstance *Instance) WoxImage {
	if image.ImageType == WoxImageTypeAbsolutePath {
		id := uuid.NewString()
		image.ImageType = WoxImageTypeUrl
		image.ImageData = fmt.Sprintf("http://localhost:%d/image?id=%s", GetPluginManager().GetUI().GetServerPort(ctx), id)
		localImageMap.Store(id, image.ImageData)
	}
	if image.ImageType == WoxImageTypeRelativePath {
		id := uuid.NewString()
		image.ImageType = WoxImageTypeUrl
		image.ImageData = fmt.Sprintf("http://localhost:%d/image?id=%s", GetPluginManager().GetUI().GetServerPort(ctx), id)
		localImageMap.Store(id, path.Join(pluginInstance.PluginDirectory, image.ImageData))
	}

	return image
}

func GetLocalImageMap(id string) (string, bool) {
	return localImageMap.Load(id)
}
