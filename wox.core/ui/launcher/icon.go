package launcher

import (
	"context"
	"strings"

	"wox/common/icons"
	woxplugin "wox/plugin"
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
	if icon, ok := woxplugin.HostRuntimeIcon(context.Background(), runtime); ok {
		return fromCoreImage(icon)
	}
	name := strings.ToLower(runtime)
	if name != "python" && name != "nodejs" {
		name = "script"
	}
	return fromCoreImage(icons.Get("runtime." + name))
}

func pluginMetadataIconSource(kind string) woxImage {
	if icon, ok := woxplugin.HostRuntimeIcon(context.Background(), kind); ok {
		return fromCoreImage(icon)
	}
	if kind == "go" {
		return fromCoreImage(icons.Get(icons.ControlCode))
	}
	if kind == "nodejs" || kind == "python" {
		return runtimeIconSource(kind)
	}
	return fromCoreImage(icons.Get("plugin." + kind))
}
