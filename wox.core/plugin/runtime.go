package plugin

import "strings"

type Runtime string

const (
	PLUGIN_RUNTIME_GO     Runtime = "GO"
	PLUGIN_RUNTIME_PYTHON Runtime = "PYTHON"
	PLUGIN_RUNTIME_NODEJS Runtime = "NODEJS"
	PLUGIN_RUNTIME_SCRIPT Runtime = "SCRIPT"
)

func IsSupportedRuntime(runtime string) bool {
	runtimeUpper := strings.ToUpper(runtime)
	return runtimeUpper == string(PLUGIN_RUNTIME_PYTHON) || runtimeUpper == string(PLUGIN_RUNTIME_NODEJS) || runtimeUpper == string(PLUGIN_RUNTIME_GO) || runtimeUpper == string(PLUGIN_RUNTIME_SCRIPT)
}

func ConvertToRuntime(runtime string) Runtime {
	return Runtime(strings.ToUpper(runtime))
}
