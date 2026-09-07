package launcher

import (
	"strings"
	"wox/plugin/system/websearch"
)

// validateWebSearchTableRow surfaces Title/URL template errors before a keyword can fail to register.
func (a *App) validateWebSearchTableRow(definition formDefinition, fields *formFieldsState) map[string]string {
	if definition.Value.Key != "webSearches" || fields == nil {
		return nil
	}
	urls := strings.Split(strings.ReplaceAll(fields.values["Urls"], "\r\n", "\n"), "\n")
	items := make([]string, 0, len(urls))
	for _, line := range urls {
		if line != "" {
			items = append(items, line)
		}
	}
	errors := websearch.ValidateSettingFields(fields.values["Title"], items)
	if len(errors) == 0 {
		return nil
	}
	return a.translateFormTableFieldErrors(errors)
}
