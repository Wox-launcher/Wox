package main

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

type releaseInfo struct {
	Version   string
	Date      string
	Changelog string
}

func runRelease() {
	if err := ensureGitClean(); err != nil {
		fmt.Println("Error: git working tree is not clean.")
		fmt.Println("Please commit/stash your changes before running the release process.")
		fmt.Println()
		fmt.Println(strings.TrimSpace(err.Error()))
		os.Exit(1)
	}

	// Parse latest version from CHANGELOG.md
	info, err := parseLatestFromChangelog()
	if err != nil {
		fmt.Printf("Error parsing CHANGELOG.md: %v\n", err)
		os.Exit(1)
	}

	if !validateVersion(info.Version) {
		fmt.Printf("Error: invalid version format '%s' in CHANGELOG.md\n", info.Version)
		fmt.Println("Version must follow semver format: X.Y.Z or X.Y.Z-prerelease (e.g., 2.0.0, 2.0.0-beta.6)")
		os.Exit(1)
	}

	// Check if date contains placeholder
	if strings.Contains(info.Date, "?") {
		fmt.Printf("Error: CHANGELOG.md contains placeholder date '%s'\n", info.Date)
		fmt.Println("Please update the date before releasing.")
		os.Exit(1)
	}

	// Show review prompt
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Release Review")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("Version:   %s\n", info.Version)
	fmt.Printf("Date:      %s\n", info.Date)
	fmt.Printf("Tag:       v%s\n", info.Version)
	fmt.Println("Changelog:")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println(strings.TrimSpace(info.Changelog))
	fmt.Println(strings.Repeat("-", 40))
	fmt.Println("\nThis will:")
	fmt.Println("  1. Update wox.core/updater/version.go")
	fmt.Println("  2. Update updater.json")
	fmt.Println("  3. Update wox.ui.flutter/wox/pubspec.yaml")
	fmt.Println("  4. Update assets/mac/Info.plist")
	fmt.Println("  5. Commit changes and create tag v" + info.Version)
	fmt.Println("  6. Push to trigger release workflow")
	fmt.Println(strings.Repeat("=", 60))

	fmt.Print("\nProceed with release? (yes/no): ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(strings.ToLower(input))

	if input != "yes" && input != "y" {
		fmt.Println("Release cancelled.")
		os.Exit(0)
	}

	fmt.Println("\nStarting release process...")

	// Step 1: Update version.go
	if err := updateVersionGo(info.Version); err != nil {
		fmt.Printf("Error updating version.go: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated wox.core/updater/version.go")

	// Step 2: Update updater.json
	if err := updateUpdaterJson(info.Version); err != nil {
		fmt.Printf("Error updating updater.json: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated updater.json")

	// Step 3: Update pubspec.yaml
	if err := updatePubspecYaml(info.Version); err != nil {
		fmt.Printf("Error updating pubspec.yaml: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated wox.ui.flutter/wox/pubspec.yaml")

	// Step 4: Update Info.plist
	if err := updateInfoPlist(info.Version); err != nil {
		fmt.Printf("Error updating Info.plist: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Updated assets/mac/Info.plist")

	// Step 5: Git commit and tag
	if err := gitCommitAndTag(info.Version); err != nil {
		fmt.Printf("Error in git operations: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Created commit and tag v" + info.Version)

	// Step 6: Push
	if err := gitPush(info.Version); err != nil {
		fmt.Printf("Error pushing to remote: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✓ Pushed to remote")

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Printf("Release v%s completed successfully!\n", info.Version)
	fmt.Println("GitHub Actions will now build and publish the release.")
	fmt.Println(strings.Repeat("=", 60))
}

func ensureGitClean() error {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = ".."
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git status failed: %v\n%s", err, output)
	}
	if strings.TrimSpace(string(output)) != "" {
		return fmt.Errorf("Uncommitted changes detected:\n%s", output)
	}
	return nil
}

func parseLatestFromChangelog() (releaseInfo, error) {
	content, err := os.ReadFile("../CHANGELOG.md")
	if err != nil {
		return releaseInfo{}, fmt.Errorf("failed to read CHANGELOG.md: %w", err)
	}

	// Pattern: ## vX.Y.Z — YYYY-MM-DD or ## vX.Y.Z-prerelease — YYYY-MM-DD
	headerPattern := regexp.MustCompile(`^## v([0-9]+\.[0-9]+\.[0-9]+(?:-[a-zA-Z0-9.]+)?)\s+—\s+(\S+)`)

	lines := strings.Split(string(content), "\n")
	var info releaseInfo
	var changelogLines []string
	inChangelog := false

	for _, line := range lines {
		if matches := headerPattern.FindStringSubmatch(line); matches != nil {
			if info.Version == "" {
				// First version header found
				info.Version = matches[1]
				info.Date = matches[2]
				inChangelog = true
				continue
			} else {
				// Next version header, stop
				break
			}
		}

		if inChangelog {
			// Stop at separator
			if strings.TrimSpace(line) == "---" {
				break
			}
			changelogLines = append(changelogLines, line)
		}
	}

	if info.Version == "" {
		return releaseInfo{}, fmt.Errorf("no version found in CHANGELOG.md")
	}

	info.Changelog = strings.Join(changelogLines, "\n")
	return info, nil
}

func validateVersion(version string) bool {
	// Semver regex: X.Y.Z or X.Y.Z-prerelease
	pattern := `^[0-9]+\.[0-9]+\.[0-9]+(-[a-zA-Z0-9]+(\.[a-zA-Z0-9]+)*)?$`
	matched, _ := regexp.MatchString(pattern, version)
	return matched
}

func updateVersionGo(version string) error {
	content := fmt.Sprintf(`package updater

const CURRENT_VERSION = "%s"
`, version)
	return os.WriteFile("../wox.core/updater/version.go", []byte(content), 0644)
}

func updateUpdaterJson(version string) error {
	tag := "v" + version
	content := fmt.Sprintf(`{
  "Version": "%s",
  "MacArm64DownloadUrl": "https://github.com/Wox-launcher/Wox/releases/download/%s/wox-mac-arm64.dmg",
  "MacArm64Checksum": "",
  "WindowsDownloadUrl": "https://github.com/Wox-launcher/Wox/releases/download/%s/wox-windows-amd64.exe",
  "WindowsChecksum": "",
  "LinuxDownloadUrl": "https://github.com/Wox-launcher/Wox/releases/download/%s/wox-linux-amd64",
  "LinuxChecksum": "",
  "ReleaseNotes": ""
}
`, version, tag, tag, tag)
	return os.WriteFile("../updater.json", []byte(content), 0644)
}

func gitCommitAndTag(version string) error {
	// Add all changed files
	files := []string{
		"wox.core/updater/version.go",
		"wox.ui.flutter/wox/pubspec.yaml",
		"assets/mac/Info.plist",
		"CHANGELOG.md",
		"updater.json",
	}

	for _, file := range files {
		cmd := exec.Command("git", "add", file)
		cmd.Dir = ".."
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("git add %s failed: %v\n%s", file, err, output)
		}
	}

	// Commit
	commitMsg := fmt.Sprintf("chore: release v%s", version)
	cmd := exec.Command("git", "commit", "-m", commitMsg)
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git commit failed: %v\n%s", err, output)
	}

	// Create tag
	tag := "v" + version
	cmd = exec.Command("git", "tag", "-a", tag, "-m", fmt.Sprintf("Release %s", tag))
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git tag failed: %v\n%s", err, output)
	}

	return nil
}

func gitPush(version string) error {
	// Push commits
	cmd := exec.Command("git", "push")
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push failed: %v\n%s", err, output)
	}

	// Push tag
	tag := "v" + version
	cmd = exec.Command("git", "push", "origin", tag)
	cmd.Dir = ".."
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push tag failed: %v\n%s", err, output)
	}

	return nil
}

func updatePubspecYaml(version string) error {
	path := "../wox.ui.flutter/wox/pubspec.yaml"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read pubspec.yaml: %w", err)
	}

	lines := strings.Split(string(content), "\n")
	found := false
	for i, line := range lines {
		if strings.HasPrefix(line, "version: ") {
			lines[i] = fmt.Sprintf("version: %s+1", version)
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("version key not found in pubspec.yaml")
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}

func updateInfoPlist(version string) error {
	path := "../assets/mac/Info.plist"
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read Info.plist: %w", err)
	}

	s := string(content)

	// Replace CFBundleVersion
	// <key>CFBundleVersion</key>
	// <string>X.Y.Z</string>
	reVersion := regexp.MustCompile(`(<key>CFBundleVersion</key>\s*<string>)([^<]+)(</string>)`)
	if !reVersion.MatchString(s) {
		return fmt.Errorf("CFBundleVersion not found in Info.plist")
	}
	s = reVersion.ReplaceAllString(s, "${1}"+version+"${3}")

	// Replace CFBundleShortVersionString
	// <key>CFBundleShortVersionString</key>
	// <string>X.Y.Z</string>
	reShortVersion := regexp.MustCompile(`(<key>CFBundleShortVersionString</key>\s*<string>)([^<]+)(</string>)`)
	if !reShortVersion.MatchString(s) {
		return fmt.Errorf("CFBundleShortVersionString not found in Info.plist")
	}
	s = reShortVersion.ReplaceAllString(s, "${1}"+version+"${3}")

	return os.WriteFile(path, []byte(s), 0644)
}
