package shell

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"syscall"
	"wox/util"
)

// DesktopLaunch retains shortcut arguments, working directory, elevation and show mode.
// Application-specific launch policies belong to callers, not the Shell adapter.
type DesktopLaunch struct {
	File       string
	Parameters string
	Directory  string
	Verb       string
	Show       int32
}

// ResolveDesktopLaunch reads explicit executable launch metadata, allowing app grouping properties.
// Opaque activation properties still require the original Shell path.
func ResolveDesktopLaunch(path string) (DesktopLaunch, bool) {
	req := shellExecuteRequest{File: path, Verb: "open", Show: shellExecuteShowNormal}
	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		var err error
		req, err = readShortcutLaunchRequest(path, "open", shortcutExplicitExecutable)
		if err != nil {
			// Log only explicit launch attempts; indexing and deduplication also inspect links.
			util.GetLogger().Debug(context.Background(), fmt.Sprintf("shortcut requires Shell activation %q: %s", path, err))
			return DesktopLaunch{}, false
		}
	} else if !filepath.IsAbs(path) || !strings.EqualFold(filepath.Ext(path), ".exe") {
		return DesktopLaunch{}, false
	}
	return DesktopLaunch{File: req.File, Parameters: req.Parameters, Directory: req.Directory, Verb: req.Verb, Show: req.Show}, true
}

// requestWithArguments appends individually quoted arguments without reparsing the shortcut command line.
func (launch DesktopLaunch) requestWithArguments(args []string) shellExecuteRequest {
	parameters := launch.Parameters
	for _, arg := range args {
		if parameters != "" {
			parameters += " "
		}
		parameters += syscall.EscapeArg(arg)
	}
	return shellExecuteRequest{File: launch.File, Parameters: parameters, Directory: launch.Directory, Verb: launch.Verb, Show: launch.Show}
}

// OpenWithArguments launches once; an error must not trigger a second attempt or another UAC prompt.
func (launch DesktopLaunch) OpenWithArguments(args []string) error {
	_, err := shellExecute(launch.requestWithArguments(args))
	return err
}
