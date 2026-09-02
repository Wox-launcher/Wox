package ai

import (
	"path/filepath"
	"testing"

	"wox/common"
	"wox/util"
)

func TestBuiltinSkillRoot(t *testing.T) {
	root := builtinSkillRoot()
	wantPath := filepath.Join(util.GetLocation().GetAISkillsDirectory(), "wox-plugin-creator")
	if root.Path != wantPath || root.Source != "builtin" || root.SourceName != "Wox" || !root.Builtin {
		t.Fatalf("builtin skill root = %+v", root)
	}
}

func TestSanitizeUserSkillsRemovesBuiltin(t *testing.T) {
	builtinPath := builtinSkillRoot().Path
	got := SanitizeUserSkills([]common.Skill{
		{Name: "builtin flag", Builtin: true, Path: "/tmp/copy"},
		{Name: "builtin path", Path: builtinPath},
		{Name: "user", Path: "/tmp/user"},
	})
	if len(got) != 1 || got[0].Name != "user" {
		t.Fatalf("sanitized skills = %+v", got)
	}
}
