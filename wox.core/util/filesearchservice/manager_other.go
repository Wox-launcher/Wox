//go:build !windows

package filesearchservice

import (
	"context"
	"fmt"
)

func indexedVolumeRoots() []string { return nil }

func GetStatus() Status { return Status{State: StateUnavailable, EmbeddedVersion: EmbeddedVersion} }

func Execute(context.Context, string) error {
	return fmt.Errorf("file index service is only available on Windows")
}
