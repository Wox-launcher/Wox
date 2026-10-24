package app

import (
	"strings"
	"wox/util/shell"
)

func supportsAppNewWindow(info appInfo) bool {
	return strings.HasSuffix(strings.ToLower(info.Path), ".app")
}

// macNewWindowArguments asks Launch Services for another launch and supplies explicit window semantics for known apps.
func macNewWindowArguments(info appInfo) []string {
	args := []string{"-n", "-a", info.Path}
	var executable string
	switch strings.ToLower(info.Identity) {
	case "org.mozilla.firefox", "org.mozilla.firefoxdeveloperedition", "org.mozilla.nightly":
		executable = "firefox"
	case "com.google.chrome", "com.google.chrome.canary", "org.chromium.chromium", "com.microsoft.edgemac", "com.brave.browser", "com.vivaldi.vivaldi":
		executable = "chrome"
	case "com.microsoft.vscode", "com.microsoft.vscodeinsiders":
		executable = "code"
	}
	if appArgs := appNewWindowArguments(executable); len(appArgs) > 0 {
		args = append(args, "--args")
		args = append(args, appArgs...)
	}
	return args
}

// openAppNewWindow uses the system's new-instance option rather than the ordinary activation path.
func openAppNewWindow(info appInfo) error {
	if !supportsAppNewWindow(info) {
		return errNewWindowUnsupported
	}
	_, err := shell.Run("/usr/bin/open", macNewWindowArguments(info)...)
	return err
}
