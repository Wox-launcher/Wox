package launcher

import (
	"strings"

	launcherview "wox/ui/launcher/view"
)

const fileSearchPluginID = "979d6363-025a-4f51-88d3-0b04e9dc56bf"

func isFileSearchRootsTable(idPrefix string, definition formDefinition, pluginID string) bool {
	return idPrefix == "plugin-settings" && pluginID == fileSearchPluginID && definition.Value.Key == "roots"
}

// fileSearchServiceVolumeRows builds locked prefix rows for volumes owned by the
// Windows file index service. They are display-only and never persisted.
func fileSearchServiceVolumeRows(columns []formTableColumn, volumes []string, tooltip string) []launcherview.FormTableRow {
	rows := make([]launcherview.FormTableRow, 0, len(volumes))
	for _, volume := range volumes {
		path := formatFileSearchVolumeRoot(volume)
		if path == "" {
			continue
		}
		cells := make([]launcherview.FormTableCell, len(columns))
		for index, column := range columns {
			if column.Key == "Path" {
				cells[index] = launcherview.FormTableCell{Text: path, Tooltip: tooltip}
			}
		}
		rows = append(rows, launcherview.FormTableRow{Index: -1, ReadOnly: true, Cells: cells})
	}
	return rows
}

// formatFileSearchVolumeRoot keeps drive letters in the C:\ form shown in settings.
func formatFileSearchVolumeRoot(root string) string {
	root = strings.TrimSpace(root)
	if root == "" {
		return ""
	}
	if len(root) == 2 && root[1] == ':' {
		return root + `\`
	}
	return root
}
