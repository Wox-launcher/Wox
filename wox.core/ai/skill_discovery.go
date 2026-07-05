package ai

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"

	"wox/common"
	"wox/util"

	"gopkg.in/yaml.v3"
)

type skillDiscoveryRoot struct {
	Path       string
	Source     string
	SourceName string
	Builtin    bool
}

type skillManifestFrontMatter struct {
	Name                        string `yaml:"name"`
	Description                 string `yaml:"description"`
	DisableModelInvocation      bool   `yaml:"disableModelInvocation"`
	DisableModelInvocationAlias bool   `yaml:"disable-model-invocation"`
}

// DiscoverSkills scans known local skill directories for SKILL.md bundles.
func DiscoverSkills(ctx context.Context) ([]common.Skill, error) {
	_ = ctx

	roots := discoverSkillRoots()
	skillsByManifestPath := map[string]common.Skill{}
	var scanErrors []string

	for _, root := range roots {
		if strings.TrimSpace(root.Path) == "" {
			continue
		}

		rootPath, absErr := filepath.Abs(root.Path)
		if absErr == nil {
			root.Path = rootPath
		}
		if !util.IsDirExists(root.Path) {
			continue
		}

		walkErr := filepath.WalkDir(root.Path, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				scanErrors = append(scanErrors, fmt.Sprintf("%s: %s", path, err.Error()))
				return nil
			}

			if entry.IsDir() && shouldSkipSkillDiscoveryDirectory(entry.Name()) {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.EqualFold(entry.Name(), "SKILL.md") {
				return nil
			}

			skill := loadDiscoveredSkill(path, root)
			skillsByManifestPath[skill.ManifestPath] = skill
			return nil
		})
		if walkErr != nil {
			scanErrors = append(scanErrors, fmt.Sprintf("%s: %s", root.Path, walkErr.Error()))
		}
	}

	skills := make([]common.Skill, 0, len(skillsByManifestPath))
	for _, skill := range skillsByManifestPath {
		skills = append(skills, skill)
	}
	sort.SliceStable(skills, func(i, j int) bool {
		if skills[i].SourceName != skills[j].SourceName {
			return skills[i].SourceName < skills[j].SourceName
		}
		if skills[i].Name != skills[j].Name {
			return skills[i].Name < skills[j].Name
		}
		return skills[i].Path < skills[j].Path
	})

	if len(scanErrors) > 0 {
		return skills, fmt.Errorf("%s", strings.Join(scanErrors, "; "))
	}
	return skills, nil
}

func discoverSkillRoots() []skillDiscoveryRoot {
	roots := []skillDiscoveryRoot{
		{Path: util.GetLocation().GetAISkillsDirectory(), Source: "wox", SourceName: "Wox"},
	}

	if cwd, err := os.Getwd(); err == nil {
		roots = append(roots,
			skillDiscoveryRoot{Path: filepath.Join(cwd, ".agents", "skills"), Source: "wox-builtin", SourceName: "Wox Built-in", Builtin: true},
			skillDiscoveryRoot{Path: filepath.Join(cwd, "..", ".agents", "skills"), Source: "wox-builtin", SourceName: "Wox Built-in", Builtin: true},
		)
	}

	if home, err := os.UserHomeDir(); err == nil {
		roots = append(roots,
			skillDiscoveryRoot{Path: filepath.Join(home, ".codex", "skills"), Source: "codex", SourceName: "Codex"},
			skillDiscoveryRoot{Path: filepath.Join(home, ".codex", "plugins", "cache"), Source: "codex-plugin", SourceName: "Codex Plugin"},
			skillDiscoveryRoot{Path: filepath.Join(home, ".claude", "skills"), Source: "claude", SourceName: "Claude"},
		)

		if runtime.GOOS == "darwin" {
			roots = append(roots, skillDiscoveryRoot{
				Path:       filepath.Join(home, "Library", "Application Support", "Claude", "skills"),
				Source:     "claude",
				SourceName: "Claude",
			})
		}
		if runtime.GOOS == "linux" {
			roots = append(roots, skillDiscoveryRoot{
				Path:       filepath.Join(home, ".config", "claude", "skills"),
				Source:     "claude",
				SourceName: "Claude",
			})
		}
		if runtime.GOOS == "windows" {
			if appData := strings.TrimSpace(os.Getenv("APPDATA")); appData != "" {
				roots = append(roots, skillDiscoveryRoot{
					Path:       filepath.Join(appData, "Claude", "skills"),
					Source:     "claude",
					SourceName: "Claude",
				})
			}
		}
	}

	return dedupeSkillRoots(roots)
}

func dedupeSkillRoots(roots []skillDiscoveryRoot) []skillDiscoveryRoot {
	seen := map[string]bool{}
	var deduped []skillDiscoveryRoot

	for _, root := range roots {
		path := strings.TrimSpace(root.Path)
		if path == "" {
			continue
		}
		if absPath, err := filepath.Abs(path); err == nil {
			path = absPath
		}
		if seen[path] {
			continue
		}
		root.Path = path
		seen[path] = true
		deduped = append(deduped, root)
	}

	return deduped
}

func shouldSkipSkillDiscoveryDirectory(name string) bool {
	switch name {
	case ".git", ".dart_tool", "node_modules", "vendor", "build", "dist", "target":
		return true
	default:
		return false
	}
}

func loadDiscoveredSkill(manifestPath string, root skillDiscoveryRoot) common.Skill {
	bundlePath := filepath.Dir(manifestPath)
	metadata, err := readSkillManifestFrontMatter(manifestPath)
	name := strings.TrimSpace(metadata.Name)
	if name == "" {
		name = filepath.Base(bundlePath)
	}

	skill := common.Skill{
		Id:                     stableSkillId(root.Source, name, bundlePath),
		Name:                   name,
		Description:            strings.TrimSpace(metadata.Description),
		Path:                   bundlePath,
		ManifestPath:           manifestPath,
		Source:                 root.Source,
		SourceName:             root.SourceName,
		Builtin:                root.Builtin,
		ReadOnly:               true,
		Enabled:                true,
		DisableModelInvocation: metadata.DisableModelInvocation || metadata.DisableModelInvocationAlias,
	}
	if err != nil {
		skill.Enabled = false
		skill.Error = err.Error()
	}

	return skill
}

func readSkillManifestFrontMatter(manifestPath string) (skillManifestFrontMatter, error) {
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return skillManifestFrontMatter{}, err
	}

	content := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return skillManifestFrontMatter{}, nil
	}

	var frontMatterLines []string
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			var metadata skillManifestFrontMatter
			if unmarshalErr := yaml.Unmarshal([]byte(strings.Join(frontMatterLines, "\n")), &metadata); unmarshalErr != nil {
				return skillManifestFrontMatter{}, unmarshalErr
			}
			return metadata, nil
		}
		frontMatterLines = append(frontMatterLines, lines[i])
	}

	return skillManifestFrontMatter{}, fmt.Errorf("missing closing front matter delimiter")
}

func stableSkillId(source string, name string, bundlePath string) string {
	hash := sha1.Sum([]byte(bundlePath))
	hashText := hex.EncodeToString(hash[:])[:10]
	return fmt.Sprintf("%s:%s:%s", source, slugSkillIdPart(name), hashText)
}

func slugSkillIdPart(value string) string {
	var builder strings.Builder
	lastDash := false

	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsNumber(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			builder.WriteRune('-')
			lastDash = true
		}
	}

	slug := strings.Trim(builder.String(), "-")
	if slug == "" {
		return "skill"
	}
	return slug
}
