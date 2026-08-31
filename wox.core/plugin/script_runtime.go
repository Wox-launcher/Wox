package plugin

import (
	"context"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
)

// ScriptInterpreterRuntime maps a script path or download URL to the host
// runtime that must execute it. Store installs use this before download so a
// Python/Node.js version problem fails the install instead of leaving a plugin
// that only crashes on the first query.
func ScriptInterpreterRuntime(pathOrURL string) Runtime {
	name := scriptFileName(pathOrURL)
	if name == "" {
		return ""
	}

	switch strings.ToLower(filepath.Ext(name)) {
	case ".py":
		return PLUGIN_RUNTIME_PYTHON
	case ".js":
		return PLUGIN_RUNTIME_NODEJS
	default:
		return ""
	}
}

// EnsureScriptInterpreterReady rejects script plugins whose interpreter is
// missing or below the same version floor used by the Python/Node.js hosts.
func EnsureScriptInterpreterReady(ctx context.Context, pathOrURL string) error {
	runtime := ScriptInterpreterRuntime(pathOrURL)
	if runtime == "" {
		return nil
	}

	status, ok := RuntimeStatusForRegisteredHost(ctx, runtime)
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", runtime)
	}

	if runtimeRefreshStillPending(status) {
		if status.StatusMessage != "" {
			return fmt.Errorf("%s", status.StatusMessage)
		}
		return fmt.Errorf("%s runtime is not ready", runtime)
	}
	return nil
}

// runtimeRefreshStillPending is the expected "check again later" outcome when
// the interpreter is still missing or too old after a settings refresh.
func runtimeRefreshStillPending(status RuntimeHostStatus) bool {
	return status.StatusCode == RuntimeHostStatusExecutableMissing || status.StatusCode == RuntimeHostStatusUnsupportedVersion
}

// RuntimeStatusForRegisteredHost reports host health without starting a process.
func RuntimeStatusForRegisteredHost(ctx context.Context, runtime Runtime) (RuntimeHostStatus, bool) {
	for _, item := range AllHosts {
		if strings.EqualFold(string(item.GetRuntime(ctx)), string(runtime)) {
			return item.RuntimeStatus(ctx), true
		}
	}
	return RuntimeHostStatus{}, false
}

func scriptFileName(pathOrURL string) string {
	trimmed := strings.TrimSpace(pathOrURL)
	if trimmed == "" {
		return ""
	}

	if parsed, err := url.Parse(trimmed); err == nil && parsed.Path != "" && (parsed.Scheme != "" || parsed.Host != "") {
		base := path.Base(parsed.Path)
		if base != "" && base != "." && base != "/" {
			return base
		}
	}

	return filepath.Base(strings.ReplaceAll(trimmed, "\\", "/"))
}
