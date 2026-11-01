package quickjump

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// directoryOpusLister is one Directory Opus lister window plus the dopusrt.exe that controls it.
// Callers resolve it once per action with findDirectoryOpusLister and use its methods.
type directoryOpusLister struct {
	pid         int
	hwnd        uintptr
	dopusrtPath string
}

// directoryOpusPathsResult is the XML written by `dopusrt /info <file>,paths`.
// Each <path> is one tab: lister and tab are HWNDs in hex, the element text is the
// real filesystem path, and display_path may be an Opus alias that does not exist on
// disk. Only the visible tab of each pane carries tab_state: 1 for the source (focused)
// pane, 2 for the destination pane. Virtual folders such as "This PC" are omitted.
type directoryOpusPathsResult struct {
	Result string                 `xml:"result,attr"`
	Paths  []directoryOpusTabPath `xml:"path"`
}

type directoryOpusTabPath struct {
	Lister   string `xml:"lister,attr"`
	TabState string `xml:"tab_state,attr"`
	Path     string `xml:",chardata"`
}

// selectDirectoryOpusSourcePath returns the real path of the source tab in the given lister.
// It never falls back to the destination pane, because that is not the folder the user is viewing.
func selectDirectoryOpusSourcePath(data []byte, listerHwnd uintptr) string {
	var result directoryOpusPathsResult
	if err := xml.Unmarshal(data, &result); err != nil || result.Result != "1" {
		return ""
	}

	for _, tab := range result.Paths {
		lister, err := strconv.ParseUint(strings.TrimPrefix(strings.ToLower(strings.TrimSpace(tab.Lister)), "0x"), 16, 64)
		if err != nil || uintptr(lister) != listerHwnd || strings.TrimSpace(tab.TabState) != "1" {
			continue
		}
		return strings.TrimSpace(tab.Path)
	}
	return ""
}

// buildDirectoryOpusGoArgs returns the dopusrt arguments that run Opus' Go command on targetPath
// in the active lister. Opus parses the text after /acmd itself and does not treat backslashes
// as escapes, so a root such as D:\ can stay quoted as-is.
func buildDirectoryOpusGoArgs(targetPath string) string {
	return fmt.Sprintf(`/acmd Go PATH="%s"`, strings.TrimSpace(targetPath))
}
