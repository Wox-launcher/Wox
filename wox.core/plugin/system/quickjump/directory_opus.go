package quickjump

import (
	"encoding/xml"
	"strconv"
	"strings"
)

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
