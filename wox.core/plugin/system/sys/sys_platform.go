package sys

import (
	"os/exec"
	"sync"
)

// ponytail: Command availability is stable during a Wox run; refresh this cache if runtime PATH changes must take effect without restarting.
var commandAvailabilityCache sync.Map

func commandExists(command string) bool {
	if cached, ok := commandAvailabilityCache.Load(command); ok {
		return cached.(bool)
	}
	_, err := exec.LookPath(command)
	available := err == nil
	commandAvailabilityCache.Store(command, available)
	return available
}
