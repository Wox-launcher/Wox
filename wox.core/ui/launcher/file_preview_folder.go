package launcher

import (
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	maxFolderPreviewScan    = 512
	maxFolderPreviewEntries = 12
)

// inspectFolderPreview builds a shallow directory peek without recursive size or inode tags.
func inspectFolderPreview(path string, info os.FileInfo) filePreviewContent {
	folder := inspectFolderPreviewContents(path)
	folder.Name = folderPreviewDisplayName(path)
	folder.Path = path
	folder.Modified = info.ModTime()
	return filePreviewContent{
		Kind:      "folder",
		Path:      path,
		Modified:  info.ModTime(),
		TypeLabel: "Folder",
		Folder:    folder,
	}
}

// inspectFolderPreviewContents lists one directory level and keeps only a folders-first peek.
func inspectFolderPreviewContents(path string) folderPreviewContent {
	file, err := os.Open(path)
	if err != nil {
		return folderPreviewContent{Error: err.Error()}
	}
	defer file.Close()

	entries, err := file.ReadDir(maxFolderPreviewScan + 1)
	if err != nil && err != io.EOF {
		return folderPreviewContent{Error: err.Error()}
	}
	countedAll := len(entries) <= maxFolderPreviewScan
	if !countedAll {
		entries = entries[:maxFolderPreviewScan]
	}

	folders := make([]folderPreviewEntry, 0, maxFolderPreviewEntries)
	files := make([]folderPreviewEntry, 0, maxFolderPreviewEntries)
	folderCount := 0
	fileCount := 0
	for _, entry := range entries {
		name := entry.Name()
		if name == "." || name == ".." {
			continue
		}
		if entry.IsDir() {
			folderCount++
			if len(folders) < maxFolderPreviewEntries {
				folders = append(folders, folderPreviewEntry{Name: name, IsDir: true})
			}
			continue
		}
		fileCount++
		if len(files) >= maxFolderPreviewEntries {
			continue
		}
		item := folderPreviewEntry{Name: name}
		if info, infoErr := entry.Info(); infoErr == nil {
			item.Size = info.Size()
			item.HasSize = true
		}
		files = append(files, item)
	}

	sort.SliceStable(folders, func(i, j int) bool { return folderPreviewNameLess(folders[i].Name, folders[j].Name) })
	sort.SliceStable(files, func(i, j int) bool { return folderPreviewNameLess(files[i].Name, files[j].Name) })
	peek := append(folders, files...)
	if len(peek) > maxFolderPreviewEntries {
		peek = peek[:maxFolderPreviewEntries]
	}
	return folderPreviewContent{
		FolderCount: folderCount,
		FileCount:   fileCount,
		CountedAll:  countedAll,
		Entries:     peek,
	}
}

// folderPreviewDisplayName prefers the leaf name and keeps volume roots readable.
func folderPreviewDisplayName(path string) string {
	clean := filepath.Clean(strings.TrimSpace(path))
	name := filepath.Base(clean)
	if name == "" || name == "." || name == string(os.PathSeparator) {
		return clean
	}
	return name
}

// folderPreviewNameLess matches Folder plugin ordering so the peek and result list agree.
func folderPreviewNameLess(left, right string) bool {
	leftName := strings.ToLower(left)
	rightName := strings.ToLower(right)
	if leftName == rightName {
		return left < right
	}
	return leftName < rightName
}

// folderPreviewMoreCount is the remaining shallow entries after the visible peek.
func folderPreviewMoreCount(folder folderPreviewContent) int {
	remaining := folder.FolderCount + folder.FileCount - len(folder.Entries)
	if remaining < 0 {
		return 0
	}
	return remaining
}

// formatFolderPreviewTime uses the same DateTime layout as large-file preview details.
func formatFolderPreviewTime(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.DateTime)
}
