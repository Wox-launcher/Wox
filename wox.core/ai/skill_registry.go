package ai

import (
	"sort"
	"sync"

	"wox/common"
)

// SkillRegistry holds discovered skills that agents can reference by Id.
type SkillRegistry struct {
	mu     sync.RWMutex
	skills map[string]common.Skill
}

var globalSkillRegistry = &SkillRegistry{skills: make(map[string]common.Skill)}

// GetSkillRegistry returns the process-wide skill registry.
func GetSkillRegistry() *SkillRegistry { return globalSkillRegistry }

// Register adds or replaces a skill keyed by Id. Thread-safe.
func (r *SkillRegistry) Register(skill common.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills[skill.Id] = skill
}

// ReplaceAll atomically replaces the discovered skill set.
func (r *SkillRegistry) ReplaceAll(skills []common.Skill) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.skills = make(map[string]common.Skill, len(skills))
	for _, skill := range skills {
		r.skills[skill.Id] = skill
	}
}

// Unregister removes a skill by Id. Thread-safe.
func (r *SkillRegistry) Unregister(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.skills, id)
}

// Get looks up a skill by Id.
func (r *SkillRegistry) Get(id string) (common.Skill, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.skills[id]
	return s, ok
}

// List returns a snapshot of all registered skills.
func (r *SkillRegistry) List() []common.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]common.Skill, 0, len(r.skills))
	for _, s := range r.skills {
		out = append(out, s)
	}
	sortSkills(out)
	return out
}

// ListEnabled returns only enabled skills.
func (r *SkillRegistry) ListEnabled() []common.Skill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]common.Skill, 0, len(r.skills))
	for _, s := range r.skills {
		if s.Enabled {
			out = append(out, s)
		}
	}
	sortSkills(out)
	return out
}

func sortSkills(skills []common.Skill) {
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].SourceName != skills[j].SourceName {
			return skills[i].SourceName < skills[j].SourceName
		}
		if skills[i].Name != skills[j].Name {
			return skills[i].Name < skills[j].Name
		}
		return skills[i].Path < skills[j].Path
	})
}
