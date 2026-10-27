package ai

import (
	"context"
	"strings"

	"wox/common"
)

type projectSkillsContextKey struct{}

// WithProjectSkills freezes project skills for one request without changing the global registry.
func WithProjectSkills(ctx context.Context, skills []common.Skill) context.Context {
	return context.WithValue(ctx, projectSkillsContextKey{}, append([]common.Skill(nil), skills...))
}

// SkillsForContext returns project skills first, followed by global skills with distinct names.
func SkillsForContext(ctx context.Context) []common.Skill {
	projectSkills, _ := ctx.Value(projectSkillsContextKey{}).([]common.Skill)
	result := append([]common.Skill(nil), projectSkills...)
	for _, skill := range GetSkillRegistry().List() {
		duplicate := false
		for _, projectSkill := range projectSkills {
			if projectSkill.Enabled && strings.EqualFold(projectSkill.Name, skill.Name) {
				duplicate = true
				break
			}
		}
		if !duplicate {
			result = append(result, skill)
		}
	}
	return result
}
