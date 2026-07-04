package ai

import (
	"fmt"
	"html"
	"os"
	"path/filepath"
	"strings"

	"wox/common"
)

const (
	availableSkillsPromptMaxChars = 12000
)

// SkillRefFromSkill converts a discovered skill into the message-level reference saved in chat history.
func SkillRefFromSkill(skill common.Skill) common.AISkillRef {
	return common.AISkillRef{
		Id:     skill.Id,
		Name:   skill.Name,
		Path:   skill.ManifestPath,
		Source: skill.Source,
	}
}

// ResolveSkillRef resolves a persisted message-level skill reference against the current registry.
func ResolveSkillRef(ref common.AISkillRef) (common.Skill, bool) {
	if id := strings.TrimSpace(ref.Id); id != "" {
		if skill, ok := GetSkillRegistry().Get(id); ok {
			return skill, true
		}
	}

	refPath := strings.TrimSpace(ref.Path)
	if refPath != "" {
		for _, skill := range GetSkillRegistry().List() {
			if skill.ManifestPath == refPath || skill.Path == refPath {
				return skill, true
			}
		}
	}

	if name := strings.TrimSpace(ref.Name); name != "" {
		matches := GetSkillRegistry().FindByName(name)
		if len(matches) > 0 {
			return matches[0], true
		}
	}

	return common.Skill{}, false
}

// FormatAvailableSkillsPrompt returns a lightweight runtime summary for automatic skill selection.
func FormatAvailableSkillsPrompt(skills []common.Skill) string {
	var builder strings.Builder
	builder.WriteString("Local skills are available through the read_skill tool. Use read_skill with the skill id before following a skill. Do not assume full instructions from this summary.\n")
	builder.WriteString("<available_skills>\n")

	count := 0
	for _, skill := range skills {
		if !skill.Enabled || skill.DisableModelInvocation || strings.TrimSpace(skill.ManifestPath) == "" {
			continue
		}

		entry := formatAvailableSkillEntry(skill)
		if builder.Len()+len(entry)+len("</available_skills>") > availableSkillsPromptMaxChars {
			builder.WriteString("  <truncated>true</truncated>\n")
			break
		}

		builder.WriteString(entry)
		count++
	}
	builder.WriteString("</available_skills>")

	if count == 0 {
		return ""
	}
	return builder.String()
}

// FormatSkillInvocation loads the full SKILL.md body for explicit use in the current model request.
func FormatSkillInvocation(skill common.Skill) (string, error) {
	content, manifestPath, err := readSkillManifestBody(skill)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	builder.WriteString(`<skill id="`)
	builder.WriteString(html.EscapeString(skill.Id))
	builder.WriteString(`" name="`)
	builder.WriteString(html.EscapeString(skill.Name))
	builder.WriteString(`" source="`)
	builder.WriteString(html.EscapeString(skill.Source))
	builder.WriteString(`">`)
	builder.WriteString("\nManifest path: ")
	builder.WriteString(manifestPath)
	builder.WriteString("\nBundle path: ")
	builder.WriteString(filepath.Dir(manifestPath))
	builder.WriteString("\n\n")
	builder.WriteString(content)
	builder.WriteString("\n</skill>")
	return builder.String(), nil
}

// formatAvailableSkillEntry keeps the model-facing directory entry compact.
func formatAvailableSkillEntry(skill common.Skill) string {
	source := skill.SourceName
	if strings.TrimSpace(source) == "" {
		source = skill.Source
	}

	var builder strings.Builder
	builder.WriteString(`  <skill id="`)
	builder.WriteString(html.EscapeString(skill.Id))
	builder.WriteString(`" name="`)
	builder.WriteString(html.EscapeString(skill.Name))
	builder.WriteString(`" source="`)
	builder.WriteString(html.EscapeString(source))
	builder.WriteString(`" path="`)
	builder.WriteString(html.EscapeString(skill.ManifestPath))
	builder.WriteString(`">`)
	if description := compactSkillDescription(skill.Description, 360); description != "" {
		builder.WriteString("\n    <description>")
		builder.WriteString(html.EscapeString(description))
		builder.WriteString("</description>\n  ")
	}
	builder.WriteString("</skill>\n")
	return builder.String()
}

// compactSkillDescription normalizes long YAML descriptions for the runtime summary.
func compactSkillDescription(value string, maxLen int) string {
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= maxLen {
		return value
	}
	return strings.TrimSpace(value[:maxLen]) + "..."
}

// readSkillManifestBody reads SKILL.md and returns only the instruction body.
func readSkillManifestBody(skill common.Skill) (string, string, error) {
	manifestPath := strings.TrimSpace(skill.ManifestPath)
	if manifestPath == "" && strings.TrimSpace(skill.Path) != "" {
		manifestPath = filepath.Join(skill.Path, "SKILL.md")
	}
	if manifestPath == "" {
		return "", "", fmt.Errorf("skill manifest path is empty: %s", skill.Id)
	}

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return "", "", err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	return stripSkillFrontMatter(content), manifestPath, nil
}

// stripSkillFrontMatter removes YAML metadata that is already represented in Skill.
func stripSkillFrontMatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return strings.TrimSpace(content)
	}

	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.TrimSpace(strings.Join(lines[i+1:], "\n"))
		}
	}
	return strings.TrimSpace(content)
}
