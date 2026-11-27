package util

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// Unzip accepts Windows-authored entry separators on every supported platform.
func Unzip(source, destination string) error {
	reader, err := zip.OpenReader(source)
	if err != nil {
		return err
	}
	defer reader.Close()

	absDest, err := filepath.Abs(destination)
	if err != nil {
		return err
	}

	type validatedEntry struct {
		file       *zip.File
		targetPath string
		isDir      bool
	}
	entries := make([]validatedEntry, 0, len(reader.File))

	// Validate every archive entry before creating destination or any extracted file.
	for _, file := range reader.File {
		normalized := strings.ReplaceAll(file.Name, "\\", "/")
		if strings.HasPrefix(file.Name, "\\") || path.IsAbs(normalized) || strings.Contains(normalized, ":") {
			return fmt.Errorf("invalid archive entry path: %q", file.Name)
		}
		cleanName := path.Clean(normalized)
		if cleanName == ".." || strings.HasPrefix(cleanName, "../") {
			return fmt.Errorf("invalid archive entry path: %q", file.Name)
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsupported symlink archive entry: %q", file.Name)
		}

		targetPath := absDest
		if cleanName != "." {
			targetPath = filepath.Join(absDest, filepath.FromSlash(cleanName))
			rel, relErr := filepath.Rel(absDest, targetPath)
			if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				return fmt.Errorf("invalid archive entry path: %q", file.Name)
			}
		}

		entries = append(entries, validatedEntry{
			file:       file,
			targetPath: targetPath,
			isDir:      file.FileInfo().IsDir() || strings.HasSuffix(normalized, "/"),
		})
	}

	if err := os.MkdirAll(absDest, 0o755); err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.isDir {
			if err := os.MkdirAll(entry.targetPath, 0o755); err != nil {
				return err
			}
			continue
		}

		if err := extractZipFile(entry.file, entry.targetPath); err != nil {
			return err
		}
	}

	return nil
}

// extractZipFile writes one regular archive entry to targetPath.
func extractZipFile(file *zip.File, targetPath string) error {
	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return err
	}

	src, err := file.Open()
	if err != nil {
		return err
	}

	perm := file.Mode().Perm()
	if perm == 0 {
		perm = 0o644
	}
	dst, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		_ = src.Close()
		return err
	}

	_, copyErr := io.Copy(dst, src)
	closeDstErr := dst.Close()
	closeSrcErr := src.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeDstErr != nil {
		return closeDstErr
	}
	return closeSrcErr
}
