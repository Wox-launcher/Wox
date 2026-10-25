package plugin

import (
	"strings"
	"wox/common"
	"wox/util/pathquery"
)

// FolderBrowseQuery opens path with Folder's current concrete trigger keyword.
// The keyword is resolved on each call so a rename or deletion is used immediately.
// With only the global keyword left, the path stays a global query.
func FolderBrowseQuery(path string) common.PlainQuery {
	text := pathquery.EnsureTrailingSeparator(path)
	if keyword := folderBrowseTriggerKeyword(); keyword != "" {
		text = keyword + " " + text
	}
	return common.PlainQuery{QueryType: QueryTypeInput, QueryText: text}
}

// folderBrowseTriggerKeyword returns Folder's first non-global trigger keyword.
func folderBrowseTriggerKeyword() string {
	instance := GetPluginManager().GetPluginInstanceById(common.FolderPluginID)
	if instance == nil {
		return ""
	}
	return strings.TrimSpace(instance.PrimaryTriggerKeyword())
}
