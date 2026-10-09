package plugin

import (
	"context"
	"strings"

	"wox/common"
)

// RuntimeIcon is implemented by a host that brings its own settings mark.
// Plugin details and runtime settings ask for it before the shared catalog,
// so a compatibility layer can show its brand without a UI special case.
type RuntimeIcon interface {
	Icon(ctx context.Context) common.WoxImage
}

// HostRuntimeIcon returns the mark supplied by the host registered for runtime.
func HostRuntimeIcon(ctx context.Context, runtime string) (common.WoxImage, bool) {
	runtime = strings.TrimSpace(runtime)
	if runtime == "" {
		return common.WoxImage{}, false
	}
	for _, host := range AllHosts {
		if host == nil || !strings.EqualFold(string(host.GetRuntime(ctx)), runtime) {
			continue
		}
		source, ok := host.(RuntimeIcon)
		if !ok {
			return common.WoxImage{}, false
		}
		icon := source.Icon(ctx)
		if icon.IsEmpty() {
			return common.WoxImage{}, false
		}
		return icon, true
	}
	return common.WoxImage{}, false
}
