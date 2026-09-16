package launcher

// formTableRowEditorSection is one ungrouped block or named collapsible group.
type formTableRowEditorSection struct {
	Group     formTableGroup
	Collapsed bool
	Fields    []formTableRowEditorField
}

// formTableRowEditorField keeps the original row-form index so focus and
// callbacks still address the same definition after groups hide fields.
type formTableRowEditorField struct {
	Index      int
	Definition formDefinition
}

func defaultFormTableCollapsedGroups(definition formDefinition) map[string]bool {
	if len(definition.Value.Groups) == 0 {
		return nil
	}
	collapsed := make(map[string]bool, len(definition.Value.Groups))
	for _, group := range definition.Value.Groups {
		if group.Key != "" && group.CollapsedByDefault {
			collapsed[group.Key] = true
		}
	}
	if len(collapsed) == 0 {
		return nil
	}
	return collapsed
}

func cloneFormTableCollapsedGroups(values map[string]bool) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	copy := make(map[string]bool, len(values))
	for key, collapsed := range values {
		copy[key] = collapsed
	}
	return copy
}

func formTableGroupByKey(definition formDefinition, key string) (formTableGroup, bool) {
	if key == "" {
		return formTableGroup{}, false
	}
	for _, group := range definition.Value.Groups {
		if group.Key == key {
			return group, true
		}
	}
	return formTableGroup{}, false
}

func formTableFieldGroupKey(definition formDefinition, field formDefinition) string {
	if _, ok := formTableGroupByKey(definition, field.Value.Group); ok {
		return field.Value.Group
	}
	return ""
}

func formTableRowFieldCollapsed(definition formDefinition, field formDefinition, collapsed map[string]bool) bool {
	key := formTableFieldGroupKey(definition, field)
	return key != "" && collapsed[key]
}

// formTableRowEditorSections puts ungrouped fields first, then declared groups
// that still have visible fields. Unknown group keys stay ungrouped so fields
// cannot disappear because of a typo.
func formTableRowEditorSections(definition formDefinition, fields []formDefinition, collapsed map[string]bool, visible func(formDefinition) bool) []formTableRowEditorSection {
	if visible == nil {
		visible = func(formDefinition) bool { return true }
	}
	ungrouped := formTableRowEditorSection{}
	grouped := make([]formTableRowEditorSection, 0, len(definition.Value.Groups))
	indexByKey := make(map[string]int, len(definition.Value.Groups))
	for _, group := range definition.Value.Groups {
		if group.Key == "" {
			continue
		}
		if _, exists := indexByKey[group.Key]; exists {
			continue
		}
		indexByKey[group.Key] = len(grouped)
		grouped = append(grouped, formTableRowEditorSection{Group: group, Collapsed: collapsed[group.Key]})
	}
	for index, field := range fields {
		if !visible(field) {
			continue
		}
		item := formTableRowEditorField{Index: index, Definition: field}
		if key := formTableFieldGroupKey(definition, field); key != "" {
			grouped[indexByKey[key]].Fields = append(grouped[indexByKey[key]].Fields, item)
			continue
		}
		ungrouped.Fields = append(ungrouped.Fields, item)
	}
	sections := make([]formTableRowEditorSection, 0, 1+len(grouped))
	if len(ungrouped.Fields) > 0 {
		sections = append(sections, ungrouped)
	}
	for _, section := range grouped {
		if len(section.Fields) == 0 {
			continue
		}
		sections = append(sections, section)
	}
	return sections
}

func expandFormTableGroupsForErrors(state *formTableEditorState) {
	if state == nil || len(state.fieldErrors) == 0 || len(state.collapsedGroups) == 0 {
		return
	}
	for _, column := range state.definition.Value.Columns {
		if column.Group == "" || state.fieldErrors[column.Key] == "" {
			continue
		}
		if _, ok := formTableGroupByKey(state.definition, column.Group); ok {
			state.collapsedGroups[column.Group] = false
		}
	}
}
