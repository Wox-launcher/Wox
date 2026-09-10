package launcher

import (
	"strings"

	"wox/common/icons"
)

func settingNavIconSource(id string) woxImage {
	return fromCoreImage(icons.Get("settings." + id))
}

func settingControlIconSource(id string) woxImage {
	return fromCoreImage(icons.Get("control." + id))
}

func usageIconSource(id string) woxImage {
	return fromCoreImage(icons.Get("usage." + id))
}

func runtimeIconSource(runtime string) woxImage {
	name := strings.ToLower(runtime)
	if name != "python" && name != "nodejs" {
		name = "script"
	}
	return fromCoreImage(icons.Get("runtime." + name))
}

func pluginMetadataIconSource(kind string) woxImage {
	if kind == "nodejs" || kind == "python" {
		return runtimeIconSource(kind)
	}
	return fromCoreImage(icons.Get("plugin." + kind))
}
