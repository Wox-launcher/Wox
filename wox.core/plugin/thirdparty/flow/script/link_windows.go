//go:build windows

package script

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"wox/util/shell"
)

// ensureDirectoryLink makes link a junction to target. An existing junction to
// the same directory is left in place. A real directory is not replaced.
func ensureDirectoryLink(link, target string) error {
	target, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	link, err = filepath.Abs(link)
	if err != nil {
		return err
	}
	same, err := existingDirectoryLink(link, target)
	if err != nil {
		return err
	}
	if same {
		return nil
	}
	if _, err := os.Lstat(link); err == nil {
		if err := os.Remove(link); err != nil {
			return err
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(link), 0o755); err != nil {
		return err
	}
	cmd := shell.BuildCommand("cmd", nil, "/c", "mklink", "/J", link, target)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("create directory link: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

func existingDirectoryLink(link, target string) (bool, error) {
	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	dest, readErr := os.Readlink(link)
	if readErr != nil {
		if info.IsDir() {
			return false, fmt.Errorf("directory link %s already exists and is not a link", link)
		}
		return false, readErr
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(filepath.Dir(link), dest)
	}
	return sameDirectoryPath(dest, target), nil
}
