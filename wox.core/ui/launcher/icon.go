package launcher

import (
	"strings"

	"wox/common"
)

func settingNavIconSource(id string) woxImage {
	return fromCoreImage(common.UIIcon("settings." + id))
}

func settingControlIconSource(id string) woxImage {
	return fromCoreImage(common.UIIcon("control." + id))
}

func usageIconSource(id string) woxImage {
	return fromCoreImage(common.UIIcon("usage." + id))
}

func runtimeIconSource(runtime string) woxImage {
	name := strings.ToLower(runtime)
	if name != "python" && name != "nodejs" {
		name = "script"
	}
	return fromCoreImage(common.UIIcon("runtime." + name))
}

func pluginMetadataIconSource(kind string) woxImage {
	if kind == "nodejs" || kind == "python" {
		return runtimeIconSource(kind)
	}
	return fromCoreImage(common.UIIcon("plugin." + kind))
}
