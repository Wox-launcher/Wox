package ai

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"wox/util"
)

// CloneSkillRepo clones a git repository (shallow) into the AI skills cache
// directory and returns the local path of the clone.
func CloneSkillRepo(ctx context.Context, url string) (string, error) {
	url = strings.TrimSpace(url)
	if url == "" {
		return "", fmt.Errorf("url is required")
	}

	cacheDir := util.GetLocation().GetAISkillsCacheDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(cacheDir); err != nil {
		return "", fmt.Errorf("failed to create skills cache directory: %w", err)
	}

	hash := sha1.Sum([]byte(url))
	hashText := hex.EncodeToString(hash[:])[:12]
	cloneDir := filepath.Join(cacheDir, hashText)

	// Remove existing clone so we always get fresh content.
	if util.IsDirExists(cloneDir) {
		_ = os.RemoveAll(cloneDir)
	}

	cmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", url, cloneDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git clone failed: %w, output: %s", err, string(output))
	}

	return cloneDir, nil
}

// DiscoverRemoteSkills clones the given git URL and scans it for SKILL.md
// files. It returns one skill stub per SKILL.md found. Each stub has its
// Path set to the bundle directory and SourceUrl set to the original URL.
// The caller is expected to persist these stubs so that DiscoverSkills can
// re-scan the paths on subsequent reloads.
func DiscoverRemoteSkills(ctx context.Context, url string) ([]RemoteSkillStub, error) {
	cloneDir, err := CloneSkillRepo(ctx, url)
	if err != nil {
		return nil, err
	}

	var stubs []RemoteSkillStub
	walkErr := filepath.WalkDir(cloneDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if entry.IsDir() && shouldSkipSkillDiscoveryDirectory(entry.Name()) {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.EqualFold(entry.Name(), "SKILL.md") {
			return nil
		}

		bundlePath := filepath.Dir(path)
		metadata, parseErr := readSkillManifestFrontMatter(path)
		name := strings.TrimSpace(metadata.Name)
		if name == "" {
			name = filepath.Base(bundlePath)
		}

		stub := RemoteSkillStub{
			Path:        bundlePath,
			ManifestPath: path,
			Name:        name,
			Description: strings.TrimSpace(metadata.Description),
			SourceUrl:   url,
		}
		if parseErr != nil {
			stub.Error = parseErr.Error()
		}
		stubs = append(stubs, stub)
		return nil
	})
	if walkErr != nil {
		return stubs, fmt.Errorf("failed to scan cloned repo: %w", walkErr)
	}

	if len(stubs) == 0 {
		return nil, fmt.Errorf("no SKILL.md files found in the repository")
	}

	return stubs, nil
}

// RemoteSkillStub is a lightweight description of a skill discovered in a
// freshly cloned remote repository. It carries enough information for the
// caller to construct a common.Skill entry.
type RemoteSkillStub struct {
	Path         string
	ManifestPath string
	Name         string
	Description  string
	Error        string
	SourceUrl    string
}