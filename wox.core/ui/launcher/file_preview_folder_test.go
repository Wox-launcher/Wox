package launcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestInspectPreviewFileBuildsFolderPeek(t *testing.T) {
	root := t.TempDir()
	folder := filepath.Join(root, "Droppy")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"zeta.txt", "alpha.txt", "readme.md"} {
		if err := os.WriteFile(filepath.Join(folder, name), []byte(name), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(folder, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(folder, "Projects"), 0o700); err != nil {
		t.Fatal(err)
	}

	content := inspectPreviewFile(folder, "", false)
	if content.Kind != "folder" {
		t.Fatalf("kind = %q, want folder", content.Kind)
	}
	if len(content.Tags) != 0 {
		t.Fatalf("tags = %#v, want no FILE/size chips on the inspected folder", content.Tags)
	}
	if content.Folder.Name != "Droppy" || content.Folder.Path != folder {
		t.Fatalf("identity = %+v", content.Folder)
	}
	if !content.Folder.CountedAll || content.Folder.FolderCount != 2 || content.Folder.FileCount != 3 {
		t.Fatalf("counts = %+v", content.Folder)
	}
	if len(content.Folder.Entries) != 5 {
		t.Fatalf("entries = %#v, want folders first then files", namesOfFolderPreview(content.Folder.Entries))
	}
	if !content.Folder.Entries[0].IsDir || !content.Folder.Entries[1].IsDir {
		t.Fatalf("entries = %#v, want folders first", content.Folder.Entries)
	}
	if content.Folder.Entries[0].Name != "Projects" || content.Folder.Entries[1].Name != "src" {
		t.Fatalf("folder order = %#v", namesOfFolderPreview(content.Folder.Entries))
	}
	if content.Folder.Entries[2].Name != "alpha.txt" || !content.Folder.Entries[2].HasSize {
		t.Fatalf("file peek = %#v", content.Folder.Entries[2])
	}
	if folderPreviewMoreCount(content.Folder) != 0 {
		t.Fatalf("more = %d, want 0", folderPreviewMoreCount(content.Folder))
	}
}

func TestInspectPreviewFileCapsFolderPeek(t *testing.T) {
	folder := t.TempDir()
	if err := os.Mkdir(filepath.Join(folder, "keep"), 0o700); err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 15; index++ {
		name := filepath.Join(folder, string(rune('a'+index))+"-file.txt")
		if err := os.WriteFile(name, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	content := inspectPreviewFile(folder, "", false)
	if content.Kind != "folder" || content.Folder.FolderCount != 1 || content.Folder.FileCount != 15 {
		t.Fatalf("folder peek = %+v", content.Folder)
	}
	if len(content.Folder.Entries) != maxFolderPreviewEntries {
		t.Fatalf("entries = %d, want %d", len(content.Folder.Entries), maxFolderPreviewEntries)
	}
	if !content.Folder.Entries[0].IsDir || content.Folder.Entries[0].Name != "keep" {
		t.Fatalf("first entry = %#v, want the only folder", content.Folder.Entries[0])
	}
	if folderPreviewMoreCount(content.Folder) != 4 {
		t.Fatalf("more = %d, want 4 hidden files", folderPreviewMoreCount(content.Folder))
	}
}

func TestInspectPreviewFileEmptyFolderHasNoInodeSizeTag(t *testing.T) {
	folder := t.TempDir()
	content := inspectPreviewFile(folder, "", false)
	if content.Kind != "folder" || content.Folder.Error != "" || content.Folder.FolderCount != 0 || content.Folder.FileCount != 0 {
		t.Fatalf("empty folder = %+v", content)
	}
	if len(content.Tags) != 0 {
		t.Fatalf("tags = %#v, want no 4 KB directory-size chip", content.Tags)
	}
}

func TestFolderPreviewDisplayNameUsesVolumeRoot(t *testing.T) {
	if name := folderPreviewDisplayName(`C:\`); name == `\` || name == "" {
		t.Fatalf("volume root name = %q", name)
	}
}

// TestFolderPreviewTagsUseSeparateCounts covers complete, bounded, empty, and failed scans.
func TestFolderPreviewTagsUseSeparateCounts(t *testing.T) {
	app := &App{translations: map[string]string{
		"ui_file_preview_folder_folders_count": "{count} folders",
		"ui_file_preview_folder_files_count":   "{count} files",
	}}
	for _, test := range []struct {
		folder         folderPreviewContent
		folders, files string
	}{
		{folderPreviewContent{FolderCount: 5, FileCount: 8, CountedAll: true}, "5 folders", "8 files"},
		{folderPreviewContent{FolderCount: 2, FileCount: 510}, "2+ folders", "510+ files"},
		{folderPreviewContent{CountedAll: true}, "0 folders", "0 files"},
		{folderPreviewContent{FileCount: 3, CountedAll: true}, "0 folders", "3 files"},
	} {
		tags := app.folderPreviewTags(test.folder)
		if len(tags) != 2 || tags[0].Label != test.folders || tags[1].Label != test.files {
			t.Fatalf("tags = %#v, want %q and %q", tags, test.folders, test.files)
		}
	}
	if tags := app.folderPreviewTags(folderPreviewContent{Error: "permission denied"}); len(tags) != 0 {
		t.Fatalf("unreadable folder must not show counts: %#v", tags)
	}
}

func namesOfFolderPreview(entries []folderPreviewEntry) []string {
	names := make([]string, len(entries))
	for index, entry := range entries {
		names[index] = entry.Name
	}
	return names
}
