package util

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// CopyOptions customizes directory and file copy behavior for CopyPath.
type CopyOptions struct {
	SkipSymlinks bool
	Skip         func(info os.FileInfo, srcPath, destPath string) (bool, error)
	OnError      func(srcPath, destPath string, err error) error
}

// CopyPath copies a file or directory tree from src to dest without following symlinks or junctions.
func CopyPath(src, dest string, opts ...CopyOptions) error {
	var opt CopyOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	return copyPathEntry(src, dest, opt)
}

func copyPathEntry(src, dest string, opt CopyOptions) error {
	handleErr := func(err error) error {
		if err == nil {
			return nil
		}
		if opt.OnError != nil {
			return opt.OnError(src, dest, err)
		}
		return err
	}

	info, err := os.Lstat(src)
	if err != nil {
		return handleErr(err)
	}

	if opt.Skip != nil {
		skip, skipErr := opt.Skip(info, src, dest)
		if skipErr != nil {
			return handleErr(skipErr)
		}
		if skip {
			return nil
		}
	}

	if info.Mode()&os.ModeSymlink != 0 {
		if opt.SkipSymlinks {
			return nil
		}
		target, readErr := os.Readlink(src)
		if readErr != nil {
			return handleErr(readErr)
		}
		if mkErr := os.MkdirAll(filepath.Dir(dest), 0o755); mkErr != nil {
			return handleErr(mkErr)
		}
		return handleErr(os.Symlink(target, dest))
	}

	if !info.Mode().IsRegular() && !info.IsDir() {
		if opt.SkipSymlinks {
			return nil
		}
		return handleErr(fmt.Errorf("unsupported non-regular file %s (%s)", src, info.Mode()))
	}

	if info.IsDir() {
		dirPerm := info.Mode().Perm() | 0o700
		if err := os.MkdirAll(dest, dirPerm); err != nil {
			return handleErr(err)
		}
		entries, err := os.ReadDir(src)
		if err != nil {
			return handleErr(err)
		}
		for _, entry := range entries {
			if err := copyPathEntry(filepath.Join(src, entry.Name()), filepath.Join(dest, entry.Name()), opt); err != nil {
				return err
			}
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return handleErr(err)
	}
	srcFile, err := os.Open(src)
	if err != nil {
		return handleErr(err)
	}
	filePerm := info.Mode().Perm()
	if filePerm == 0 {
		filePerm = 0o644
	}
	dstFile, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, filePerm)
	if err != nil {
		_ = srcFile.Close()
		return handleErr(err)
	}
	_, copyErr := io.Copy(dstFile, srcFile)
	closeDstErr := dstFile.Close()
	closeSrcErr := srcFile.Close()
	if copyErr != nil {
		return handleErr(copyErr)
	}
	if closeDstErr != nil {
		return handleErr(closeDstErr)
	}
	return handleErr(closeSrcErr)
}
