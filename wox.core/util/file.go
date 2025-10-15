package util

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/samber/lo"
)

// IsFileExecAny returns true if the file mode indicates that the file is executable by any user.
func IsFileExecAny(mode os.FileMode) bool {
	return mode&0111 != 0
}

func IsFileExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && !stat.IsDir()
}

func IsDirExists(path string) bool {
	stat, err := os.Stat(path)
	return err == nil && stat.IsDir()
}

func ListDir(path string) ([]string, error) {
	dir, err := os.ReadDir(path)
	if err != nil {
		return nil, err
	}

	var files []string
	for _, file := range dir {
		files = append(files, file.Name())
	}

	return files, nil
}

func IsImageFile(path string) bool {
	currentExt := strings.ToLower(filepath.Ext(path))
	imageSuffixList := []string{".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp", ".tiff", ".svg"}
	return lo.Contains(imageSuffixList, currentExt)
}

func GetFileModifiedAt(path string) string {
	stat, err := os.Stat(path)
	if err != nil {
		return "-"
	}

	return FormatTime(stat.ModTime())
}

func GetFileSize(path string) string {
	stat, err := os.Stat(path)
	if err != nil {
		return "-"
	}

	//if size is less than 1KB, show bytes
	//if size is less than 1MB, show KB
	//if size is less than 1GB, show MB
	//if size is less than 1TB, show GB
	//if size is less than 1PB, show TB

	size := stat.Size()
	if size < 1024 {
		return strconv.FormatInt(size, 10) + " B"
	}
	if size < 1024*1024 {
		return strconv.FormatInt(size/1024, 10) + " KB"
	}
	if size < 1024*1024*1024 {
		return strconv.FormatInt(size/1024/1024, 10) + " MB"
	}
	if size < 1024*1024*1024*1024 {
		return strconv.FormatInt(size/1024/1024/1024, 10) + " GB"
	}
	if size < 1024*1024*1024*1024*1024 {
		return strconv.FormatInt(size/1024/1024/1024/1024, 10) + " TB"
	}

	return "-"
}

// CollectExecutables returns candidate executable paths under base for the provided binary names.
func CollectExecutables(base string, binaries []string, filter func(string) bool) []string {
	if base == "" {
		return nil
	}

	var results []string
	for _, binary := range binaries {
		results = append(results, filepath.Join(base, binary))
	}

	if !IsDirExists(base) {
		return results
	}

	entries, err := ListDir(base)
	if err != nil {
		return results
	}

	for _, entry := range entries {
		if filter != nil && !filter(entry) {
			continue
		}
		for _, binary := range binaries {
			results = append(results, filepath.Join(base, entry, binary))
		}
	}

	return results
}
