package resource

import (
	"context"
	"embed"
	"fmt"
	"os"
	"path"
	"wox/util"
)

//go:embed hosts
var HostFS embed.FS

func ExtractHosts(ctx context.Context) error {
	dir, err := HostFS.ReadDir("hosts")
	if err != nil {
		return err
	}
	if len(dir) == 0 {
		return fmt.Errorf("no host file found")
	}

	for _, entry := range dir {
		util.GetLogger().Info(ctx, fmt.Sprintf("extracting host file: %s", entry.Name()))
		hostData, readErr := HostFS.ReadFile("hosts/" + entry.Name())
		if readErr != nil {
			return readErr
		}

		var hostFilePath = path.Join(util.GetLocation().GetHostDirectory(), entry.Name())
		writeErr := os.WriteFile(hostFilePath, hostData, 0644)
		if writeErr != nil {
			return writeErr
		}
	}

	return nil
}
