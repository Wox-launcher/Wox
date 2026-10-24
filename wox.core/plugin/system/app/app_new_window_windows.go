package app

import (
	"path/filepath"
	"wox/util/shell"

	"golang.org/x/sys/windows"
)

// windowsNewWindowLaunch preserves the complete shortcut launch contract before adding app arguments.
func windowsNewWindowLaunch(info appInfo) (shell.DesktopLaunch, []string, bool) {
	if info.Type != AppTypeDesktop && info.Type != AppTypeAppsFolder {
		return shell.DesktopLaunch{}, nil, false
	}
	path := info.Path
	if info.Type == AppTypeAppsFolder {
		path = windowsAppExecutablePath(info)
	}
	launch, ok := shell.ResolveDesktopLaunch(path)
	if !ok {
		return shell.DesktopLaunch{}, nil, false
	}
	args := appNewWindowArguments(filepath.Base(launch.File))
	if len(args) == 0 {
		return shell.DesktopLaunch{}, nil, false
	}
	// Prefix a dummy argv[0] so Windows parses the shortcut arguments with normal argument quoting.
	existing, err := windows.DecomposeCommandLine("wox " + launch.Parameters)
	if err != nil || len(existing) == 0 || !supportsNewWindowArguments(existing[1:]) {
		return shell.DesktopLaunch{}, nil, false
	}
	return launch, args, true
}

func supportsAppNewWindow(info appInfo) bool {
	_, _, ok := windowsNewWindowLaunch(info)
	return ok
}

// openAppNewWindow reports unavailable strategies so the action can reuse normal Open.
func openAppNewWindow(info appInfo) error {
	launch, args, ok := windowsNewWindowLaunch(info)
	if !ok {
		return errNewWindowUnsupported
	}
	return launch.OpenWithArguments(args)
}
