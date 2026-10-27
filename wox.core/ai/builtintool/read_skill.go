package tool

import (
	"context"
	"fmt"
	"strings"

	"wox/ai"
	"wox/common"

	"github.com/tmc/langchaingo/jsonschema"
)

func init() {
	ai.GetToolRegistry().Register(ReadSkillTool())
}

// ReadSkillTool loads full SKILL.md instructions on demand.
func ReadSkillTool() common.Tool {
	return common.Tool{
		Name:        ai.ReadSkillToolName,
		Description: "Load the full instructions for a discovered local skill by id or exact name. Prefer id when available from the available_skills context.",
		Parameters: jsonschema.Definition{
			Type: jsonschema.Object,
			Properties: map[string]jsonschema.Definition{
				"id":   {Type: jsonschema.String, Description: "Skill id from the available_skills context"},
				"name": {Type: jsonschema.String, Description: "Exact skill name if the id is not known"},
			},
		},
		Source:   common.ToolSourceBuiltin,
		Callback: readSkillCallback,
	}
}

func readSkillCallback(ctx context.Context, args map[string]any) (common.ToolResult, error) {
	id, _ := args["id"].(string)
	name, _ := args["name"].(string)
	skill, err := resolveSkillToolInput(ctx, id, name)
	if err != nil {
		return common.ToolResult{}, err
	}

	text, err := ai.FormatSkillInvocation(skill)
	if err != nil {
		return common.ToolResult{}, err
	}
	return common.ToolResult{Text: text}, nil
}

// resolveSkillToolInput resolves either an exact id or a unique exact skill name.
func resolveSkillToolInput(ctx context.Context, id string, name string) (common.Skill, error) {
	skills := ai.SkillsForContext(ctx)
	if id = strings.TrimSpace(id); id != "" {
		for _, skill := range skills {
			if skill.Id == id {
				if !skill.Enabled {
					return common.Skill{}, fmt.Errorf("skill is disabled: %s", id)
				}
				return skill, nil
			}
		}
		return common.Skill{}, fmt.Errorf("skill id not found: %s", id)
	}

	if name = strings.TrimSpace(name); name == "" {
		return common.Skill{}, fmt.Errorf("id or name is required")
	}

	var matches []common.Skill
	for _, skill := range skills {
		if strings.EqualFold(skill.Name, name) {
			matches = append(matches, skill)
		}
	}
	if len(matches) == 0 {
		return common.Skill{}, fmt.Errorf("skill name not found: %s", name)
	}
	enabled := make([]common.Skill, 0, len(matches))
	for _, skill := range matches {
		if skill.Enabled {
			enabled = append(enabled, skill)
		}
	}
	if len(enabled) == 0 {
		return common.Skill{}, fmt.Errorf("skill is disabled: %s", name)
	}
	if len(enabled) > 1 {
		ids := make([]string, 0, len(enabled))
		for _, skill := range enabled {
			ids = append(ids, skill.Id)
		}
		return common.Skill{}, fmt.Errorf("multiple skills named %s; retry with one id: %s", name, strings.Join(ids, ", "))
	}
	return enabled[0], nil
}
