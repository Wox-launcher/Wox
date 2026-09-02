package resource

import "testing"

func TestWoxPluginCreatorSkillIsEmbedded(t *testing.T) {
	for _, filePath := range []string{
		"ai/skills/wox-plugin-creator/SKILL.md",
		"ai/skills/wox-plugin-creator/scripts/scaffold_wox_plugin.py",
		"ai/skills/wox-plugin-creator/assets/single_file_plugin_templates/template.py",
	} {
		if _, err := AIFS.ReadFile(filePath); err != nil {
			t.Errorf("embedded skill is missing %s: %v", filePath, err)
		}
	}
}
