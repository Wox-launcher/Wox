package dotnet

import (
	"os"
	"path/filepath"
	"strings"

	"wox/util"
)

const hostDLLName = "Wox.Flow.DotNetHost.dll"

// hostDirectoryOverride points a test at a locally published loader.
var hostDirectoryOverride string

// prepareHostDirectory returns the flow loader directory.
// resource.Extract copies resource/hosts/flow into the host directory.
func prepareHostDirectory() (string, string) {
	dir := strings.TrimSpace(hostDirectoryOverride)
	if dir == "" {
		dir = filepath.Join(util.GetLocation().GetHostDirectory(), "flow")
	}
	if _, err := os.Stat(filepath.Join(dir, hostDLLName)); err != nil {
		return "", ".NET flow host files were not published"
	}
	return dir, ""
}
