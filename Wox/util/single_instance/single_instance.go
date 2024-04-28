package single_instance

import (
	"fmt"
	"os"
	"strconv"
	"wox/util"
)

// Lock tries to lock the server port. If the port is already locked, it will return the existing server port
func Lock(serverPort int) (existingServerPort int, err error) {
	if lockErr := lock(fmt.Sprintf("%d", serverPort)); lockErr != nil {
		// If the file already exists, we read the content to get the existing server port
		contents, readErr := os.ReadFile(util.GetLocation().GetAppLockFilePath())
		if readErr != nil {
			return 0, fmt.Errorf("failed to lock: %s, and failed to read lock port: %s", lockErr.Error(), readErr.Error())
		}

		existingPort, parseErr := strconv.Atoi(string(contents))
		if parseErr != nil {
			return 0, fmt.Errorf("failed to lock: %s, and failed to parse lock port: %s", lockErr.Error(), parseErr.Error())
		}

		return existingPort, lockErr
	}

	return 0, nil
}
