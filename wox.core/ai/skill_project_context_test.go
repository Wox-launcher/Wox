package ai

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/common"
)

func TestProjectSkillsAreScopedAndPrimaryRootWins(t *testing.T) {
	first, second, other := t.TempDir(), t.TempDir(), t.TempDir()
	for _, root := range []string{first, second, other} {
		dir := filepath.Join(root, ".agents", "skills", "review")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: project-review\ndescription: Review changes\n---\nInstructions"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	skills, err := DiscoverProjectSkills([]string{first, second})
	if err != nil || len(skills) != 1 || skills[0].Source != "project" || skills[0].Path != filepath.Join(first, ".agents", "skills", "review") {
		t.Fatalf("project discovery = %+v, %v", skills, err)
	}
	otherSkills, err := DiscoverProjectSkills([]string{other})
	if err != nil || len(otherSkills) != 1 {
		t.Fatalf("other project discovery = %+v, %v", otherSkills, err)
	}
	ref := SkillRefFromSkill(skills[0])
	if _, ok := ResolveSkillRefForContext(WithProjectSkills(context.Background(), skills), ref); !ok {
		t.Fatal("project reference not resolved")
	}
	if _, ok := ResolveSkillRefForContext(WithProjectSkills(context.Background(), otherSkills), ref); ok {
		t.Fatal("project reference leaked across projects")
	}
	if _, ok := ResolveSkillRef(ref); ok {
		t.Fatal("project reference leaked to global registry")
	}
	// An explicit global reference continues to use the existing registry.
	if _, ok := ResolveSkillRefForContext(context.Background(), common.AISkillRef{Source: "project", Name: skills[0].Name}); ok {
		t.Fatal("unidentified project reference resolved by name")
	}
}
