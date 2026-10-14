package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"wox/util"

	"gorm.io/gorm"
)

// OCR rejects a sidecar whose size or mtime no longer matches, and edit archives are addressed by the image path.
const (
	screenshotOCRSuffix  = ".ocr.json"
	screenshotEditSuffix = ".edit.zip"
)

func init() {
	Register(&moveScreenshotsMigration{})
}

type moveScreenshotsMigration struct{}

func (m *moveScreenshotsMigration) ID() string {
	return "20261014_move_screenshots"
}

func (m *moveScreenshotsMigration) Description() string {
	return "Move saved screenshots from the Wox data directory into the user data directory."
}

// Up moves ~/.wox/screenshots to the user data directory, next to notes and chats.
// The filesystem change cannot roll back with the migration transaction, so every step is safe to repeat.
// It runs before the migration record is stored: AfterCommit only logs failures and would not retry a partial move.
func (m *moveScreenshotsMigration) Up(ctx context.Context, _ *gorm.DB) error {
	location := util.GetLocation()
	return moveScreenshotDirectory(ctx, location.GetLegacyScreenshotDirectory(), location.GetScreenshotDirectory())
}

// moveScreenshotDirectory relocates legacy captures onto dest.
// An empty destination is replaced in one rename so file mtimes, which OCR sidecars trust, stay unchanged.
func moveScreenshotDirectory(ctx context.Context, legacy, dest string) error {
	legacy = filepath.Clean(legacy)
	dest = filepath.Clean(dest)
	if screenshotDirectoriesOverlap(legacy, dest) {
		util.GetLogger().Info(ctx, "screenshot directory migration skipped because the legacy and user-data paths overlap")
		return nil
	}

	legacyInfo, err := os.Lstat(legacy)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat legacy screenshot directory: %w", err)
	}
	if !legacyInfo.IsDir() {
		return fmt.Errorf("legacy screenshot path is not a directory: %s", legacy)
	}

	replaced, err := replaceScreenshotDirectory(ctx, legacy, dest)
	if err != nil {
		return err
	}
	if replaced {
		util.GetLogger().Info(ctx, "moved legacy screenshot directory into user data")
		return nil
	}

	moved, err := moveDirectoryContents(legacy, dest)
	if err != nil {
		return err
	}
	if err := os.Remove(legacy); err != nil {
		return fmt.Errorf("remove legacy screenshot directory: %w", err)
	}
	util.GetLogger().Info(ctx, fmt.Sprintf("moved %d legacy screenshot entries into user data", moved))
	return nil
}

// replaceScreenshotDirectory renames legacy onto dest when dest is missing or empty.
// It returns false when dest already has files, or when the rename cannot be done in place.
func replaceScreenshotDirectory(ctx context.Context, legacy, dest string) (bool, error) {
	destInfo, err := os.Lstat(dest)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return false, fmt.Errorf("create screenshot parent directory: %w", err)
		}
		return renameScreenshotDirectory(ctx, legacy, dest)
	}
	if err != nil {
		return false, fmt.Errorf("stat screenshot directory: %w", err)
	}
	if !destInfo.IsDir() {
		return false, fmt.Errorf("screenshot path is not a directory: %s", dest)
	}

	empty, err := directoryIsEmpty(dest)
	if err != nil {
		return false, err
	}
	if !empty {
		return false, nil
	}
	if err := os.Remove(dest); err != nil {
		return false, fmt.Errorf("remove empty screenshot directory: %w", err)
	}
	return renameScreenshotDirectory(ctx, legacy, dest)
}

// renameScreenshotDirectory returns false when the caller should copy entries instead.
// A locked file can reject renaming the whole directory on Windows; copying still moves the files that are free.
func renameScreenshotDirectory(ctx context.Context, legacy, dest string) (bool, error) {
	err := os.Rename(legacy, dest)
	if err == nil {
		return true, nil
	}
	if mkErr := os.MkdirAll(dest, 0o755); mkErr != nil {
		return false, fmt.Errorf("create screenshot directory: %w", mkErr)
	}
	if !isCrossDeviceRename(err) {
		util.GetLogger().Warn(ctx, "screenshot directory rename failed, moving files individually: "+err.Error())
	}
	return false, nil
}

func moveDirectoryContents(src, dst string) (int, error) {
	entries, err := os.ReadDir(src)
	if err != nil {
		return 0, fmt.Errorf("read legacy screenshot directory: %w", err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		return 0, fmt.Errorf("create screenshot directory: %w", err)
	}
	for _, entry := range entries {
		if err := moveScreenshotEntry(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return 0, err
		}
	}
	return len(entries), nil
}

func moveScreenshotEntry(src, dst string) error {
	info, err := os.Lstat(src)
	if errors.Is(err, os.ErrNotExist) {
		// A colliding image moves its sidecar immediately, so this name may already be gone.
		return nil
	}
	if err != nil {
		return fmt.Errorf("stat screenshot entry %s: %w", src, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return moveScreenshotSymlink(src, dst)
	}
	if info.IsDir() {
		return moveScreenshotSubdirectory(src, dst)
	}
	return moveScreenshotFile(src, dst, info)
}

func moveScreenshotSubdirectory(src, dst string) error {
	info, err := os.Lstat(dst)
	switch {
	case errors.Is(err, os.ErrNotExist):
		renameErr := os.Rename(src, dst)
		if renameErr == nil {
			return nil
		}
		if !isCrossDeviceRename(renameErr) {
			return fmt.Errorf("move screenshot directory %s: %w", src, renameErr)
		}
	case err != nil:
		return fmt.Errorf("stat screenshot directory %s: %w", dst, err)
	case !info.IsDir():
		return moveScreenshotSubdirectory(src, uniqueSiblingPath(dst))
	}

	if _, err := moveDirectoryContents(src, dst); err != nil {
		return err
	}
	if err := os.Remove(src); err != nil {
		return fmt.Errorf("remove legacy screenshot directory %s: %w", src, err)
	}
	return nil
}

func moveScreenshotFile(src, dst string, info os.FileInfo) error {
	target := dst
	existing, err := os.Lstat(dst)
	if err == nil {
		if existing.IsDir() {
			target = uniqueSiblingPath(dst)
		} else {
			same, sameErr := sameFileContents(src, dst)
			if sameErr != nil {
				return sameErr
			}
			if same {
				return removeScreenshotFile(src)
			}
			target = uniqueSiblingPath(dst)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat screenshot file %s: %w", dst, err)
	}

	if err := moveOneScreenshotFile(src, target, info); err != nil {
		return err
	}
	if target != dst {
		return moveScreenshotCompanions(src, target)
	}
	return nil
}

// moveScreenshotCompanions keeps OCR and edit files attached when a name collision renames the image.
func moveScreenshotCompanions(srcImage, dstImage string) error {
	for _, suffix := range []string{screenshotOCRSuffix, screenshotEditSuffix} {
		src := srcImage + suffix
		info, err := os.Lstat(src)
		if errors.Is(err, os.ErrNotExist) || (err == nil && info.IsDir()) {
			continue
		}
		if err != nil {
			return fmt.Errorf("stat screenshot companion %s: %w", src, err)
		}
		dst := dstImage + suffix
		if _, statErr := os.Lstat(dst); statErr == nil {
			dst = uniqueSiblingPath(dst)
		} else if !errors.Is(statErr, os.ErrNotExist) {
			return fmt.Errorf("stat screenshot companion %s: %w", dst, statErr)
		}
		if err := moveOneScreenshotFile(src, dst, info); err != nil {
			return err
		}
	}
	return nil
}

func moveOneScreenshotFile(src, dst string, info os.FileInfo) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	} else if !isCrossDeviceRename(err) {
		return fmt.Errorf("move screenshot file %s: %w", src, err)
	}
	if err := copyScreenshotFile(src, dst, info); err != nil {
		return err
	}
	return removeScreenshotFile(src)
}

func moveScreenshotSymlink(src, dst string) error {
	if _, err := os.Lstat(dst); err == nil {
		dst = uniqueSiblingPath(dst)
	} else if !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("stat screenshot link %s: %w", dst, err)
	}
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("move screenshot link %s: %w", src, err)
	}
	return nil
}

func copyScreenshotFile(src, dst string, info os.FileInfo) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open screenshot %s: %w", src, err)
	}
	defer in.Close()

	tmp := dst + ".migrating"
	mode := info.Mode().Perm()
	if mode == 0 {
		mode = 0o644
	}
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return fmt.Errorf("create screenshot %s: %w", tmp, err)
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("copy screenshot %s: %w", src, copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("close screenshot %s: %w", tmp, closeErr)
	}
	// History order and OCR sidecars both compare the image mtime, so a copied file must keep it.
	if err := os.Chtimes(tmp, info.ModTime(), info.ModTime()); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("preserve screenshot time %s: %w", dst, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("commit screenshot %s: %w", dst, err)
	}
	return nil
}

func removeScreenshotFile(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove migrated screenshot %s: %w", path, err)
	}
	return nil
}

func sameFileContents(left, right string) (bool, error) {
	leftInfo, err := os.Stat(left)
	if err != nil {
		return false, fmt.Errorf("stat screenshot %s: %w", left, err)
	}
	rightInfo, err := os.Stat(right)
	if err != nil {
		return false, fmt.Errorf("stat screenshot %s: %w", right, err)
	}
	if leftInfo.Size() != rightInfo.Size() {
		return false, nil
	}
	leftHash, err := hashFile(left)
	if err != nil {
		return false, err
	}
	rightHash, err := hashFile(right)
	if err != nil {
		return false, err
	}
	return leftHash == rightHash, nil
}

func hashFile(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open screenshot %s: %w", path, err)
	}
	defer file.Close()
	sum := sha256.New()
	if _, err := io.Copy(sum, file); err != nil {
		return "", fmt.Errorf("hash screenshot %s: %w", path, err)
	}
	return hex.EncodeToString(sum.Sum(nil)), nil
}

func uniqueSiblingPath(path string) string {
	dir := filepath.Dir(path)
	name := filepath.Base(path)
	ext := filepath.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for i := 1; i < 1000; i++ {
		candidate := filepath.Join(dir, fmt.Sprintf("%s_legacy_%02d%s", base, i, ext))
		if _, err := os.Lstat(candidate); errors.Is(err, os.ErrNotExist) {
			return candidate
		}
	}
	return filepath.Join(dir, base+"_legacy"+ext)
}

func directoryIsEmpty(path string) (bool, error) {
	entries, err := os.ReadDir(path)
	if err != nil {
		return false, fmt.Errorf("read screenshot directory: %w", err)
	}
	return len(entries) == 0, nil
}

func screenshotDirectoriesOverlap(left, right string) bool {
	left = normalizeScreenshotPath(left)
	right = normalizeScreenshotPath(right)
	if left == right {
		return true
	}
	return screenshotPathInside(left, right) || screenshotPathInside(right, left)
}

func normalizeScreenshotPath(path string) string {
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

func screenshotPathInside(parent, child string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

// isCrossDeviceRename reports a rename that failed because the paths are on different volumes.
// Windows MoveFileEx returns ERROR_NOT_SAME_DEVICE (17), which is not syscall.EXDEV.
func isCrossDeviceRename(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	if runtime.GOOS != "windows" {
		return false
	}
	var errno syscall.Errno
	return errors.As(err, &errno) && errno == 17
}
