package util

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/saracen/fastzip"
)

// Unzip accepts Windows-authored entry separators on every supported platform.
func Unzip(source, destination string) error {
	e, err := fastzip.NewExtractor(source, destination)
	if err != nil {
		return err
	}
	defer e.Close()

	// Some plugin packagers store backslashes instead of ZIP's forward slashes.
	// Normalize before extraction so assets and dependencies retain their hierarchy,
	// and validate the resulting paths before any files are written.
	for _, file := range e.Files() {
		name := strings.ReplaceAll(file.Name, "\\", "/")
		cleanName := path.Clean(name)
		if path.IsAbs(name) || strings.Contains(name, ":") || cleanName == ".." || strings.HasPrefix(cleanName, "../") {
			return fmt.Errorf("invalid archive entry path: %q", file.Name)
		}
		file.Name = name
	}

	if err = e.Extract(context.Background()); err != nil {
		return err
	}

	return nil
}
