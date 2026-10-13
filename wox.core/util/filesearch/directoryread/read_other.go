//go:build !darwin || !cgo

package directoryread

import (
	"context"
	"os"
)

// ReadDir supplies the portable scanner's sorted directory-entry contract.
func ReadDir(ctx context.Context, path string) ([]os.DirEntry, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path)
	if err == nil {
		err = ctx.Err()
	}
	return entries, err
}
