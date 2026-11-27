//go:build !windows

package manager

import (
	"context"
	"fmt"

	"wox/util/filesearchservice"
)

func GetStatus() filesearchservice.Status {
	return filesearchservice.Status{State: filesearchservice.StateUnavailable, EmbeddedVersion: filesearchservice.EmbeddedVersion}
}

func Execute(context.Context, string) error {
	return fmt.Errorf("file index service is only available on Windows")
}
