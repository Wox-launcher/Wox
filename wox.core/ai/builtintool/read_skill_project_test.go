package tool

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wox/ai"
)

func TestReadSkillUsesOnlyCurrentProject(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, ".agents", "skills", "local")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte("---\nname: isolated-project-skill\ndescription: Local instructions\n---\nProject-specific instructions"), 0644); err != nil {
		t.Fatal(err)
	}
	skills, err := ai.DiscoverProjectSkills([]string{root})
	if err != nil || len(skills) != 1 {
		t.Fatalf("discovery: %+v, %v", skills, err)
	}
	ctx := ai.WithProjectSkills(context.Background(), skills)
	result, err := readSkillCallback(ctx, map[string]any{"id": skills[0].Id})
	if err != nil || !strings.Contains(result.Text, "Project-specific instructions") {
		t.Fatalf("project read: %+v, %v", result, err)
	}
	if _, err := readSkillCallback(context.Background(), map[string]any{"id": skills[0].Id}); err == nil {
		t.Fatal("project skill was readable without its project")
	}
}
