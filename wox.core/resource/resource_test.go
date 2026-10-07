package resource

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"wox/internal/assetfs"
)

func TestWoxPluginCreatorSkillIsEmbedded(t *testing.T) {
	for _, filePath := range []string{
		"ai/skills/wox-plugin-creator/SKILL.md",
		"ai/skills/wox-theme-creator/SKILL.md",
		"ai/skills/wox-plugin-creator/scripts/scaffold_wox_plugin.py",
		"ai/skills/wox-plugin-creator/assets/single_file_plugin_templates/template.py",
	} {
		if _, err := AIFS.ReadFile(filePath); err != nil {
			t.Errorf("embedded skill is missing %s: %v", filePath, err)
		}
	}
}

// TestEmbeddedBytesMatchSources protects paths and every byte of scripts, language
// files, icons, and native helpers, whether built with or without the release overlay.
func TestEmbeddedBytesMatchSources(t *testing.T) {
	for _, assets := range []assetfs.FS{HostFS, LangFS, ThemeFS, OthersFS, AIFS, iconFS} {
		err := fs.WalkDir(assets, ".", func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				return nil
			}
			source, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			embedded, err := assets.ReadFile(name)
			if err != nil {
				return err
			}
			if !bytes.Equal(source, embedded) {
				t.Errorf("embedded bytes differ: %s", name)
			}
			info, err := entry.Info()
			if err != nil {
				return err
			}
			if info.Size() != int64(len(source)) {
				t.Errorf("wrong original size: %s", name)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

// BenchmarkReadEmbeddedResources measures the extra work of a complete resource
// read without launching Wox or modifying the user's extracted files.
func BenchmarkReadEmbeddedResources(b *testing.B) {
	type asset struct {
		source assetfs.FS
		name   string
	}
	var files []asset
	for _, source := range []assetfs.FS{HostFS, LangFS, ThemeFS, OthersFS, AIFS, iconFS} {
		err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
			if err == nil && !entry.IsDir() {
				files = append(files, asset{source, name})
			}
			return err
		})
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, file := range files {
			if _, err := file.source.ReadFile(file.name); err != nil {
				b.Fatal(err)
			}
		}
	}
}

// TestExtractedResourcesMatchSources verifies the unchanged destination layout
// and permission policy without touching the user's Wox data directory.
func TestExtractedResourcesMatchSources(t *testing.T) {
	for _, group := range []struct {
		source assetfs.FS
		root   string
	}{
		{HostFS, "hosts"}, {OthersFS, "others"}, {AIFS, "ai"},
	} {
		destination := t.TempDir()
		if err := extractFiles(context.Background(), group.source, destination, group.root, true); err != nil {
			t.Fatal(err)
		}
		err := fs.WalkDir(group.source, group.root, func(name string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			relative, err := filepath.Rel(group.root, name)
			if err != nil {
				return err
			}
			output := filepath.Join(destination, relative)
			actual, err := os.ReadFile(output)
			if err != nil {
				return err
			}
			original, err := os.ReadFile(name)
			if err != nil {
				return err
			}
			if !bytes.Equal(actual, original) {
				t.Errorf("extracted bytes differ: %s", name)
			}
			if runtime.GOOS != "windows" {
				info, err := os.Stat(output)
				if err != nil {
					return err
				}
				executable := group.root == "others" && (relative == "recording/ffmpeg" || relative == "recording/gst-launch-1.0")
				if (info.Mode().Perm()&0111 != 0) != executable {
					t.Errorf("unexpected permissions: %s: %v", name, info.Mode())
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
