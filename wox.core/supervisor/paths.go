package supervisor

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"sync"
)

const controlAddressEnv = "WOX_SUPERVISOR_CONTROL_ADDRESS"

var (
	pathMu          sync.RWMutex
	dataDirOverride string
	versionMu       sync.RWMutex
	appVersion      string
	controlMu       sync.RWMutex
	controlOverride string
)

// SetDataDirectory selects the ~/.wox root. main sets this from the running
// location so tests and custom layouts stay outside this package.
func SetDataDirectory(dir string) {
	pathMu.Lock()
	dataDirOverride = dir
	pathMu.Unlock()
}

// SetVersion records the application version stamped onto crash reports.
func SetVersion(version string) {
	versionMu.Lock()
	appVersion = version
	versionMu.Unlock()
}

func dataDirectory() string {
	pathMu.RLock()
	override := dataDirOverride
	pathMu.RUnlock()
	if override != "" {
		return override
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".wox"
	}
	return filepath.Join(home, ".wox")
}

func currentVersion() string {
	versionMu.RLock()
	defer versionMu.RUnlock()
	return appVersion
}

// SetControlAddress selects the local connection for a detached launch or a test.
func SetControlAddress(address string) {
	controlMu.Lock()
	controlOverride = address
	controlMu.Unlock()
}

func controlAddress() string {
	controlMu.RLock()
	override := controlOverride
	controlMu.RUnlock()
	if override != "" {
		return override
	}
	if address := os.Getenv(controlAddressEnv); address != "" {
		return address
	}
	return defaultControlAddress()
}

// newControlAddress isolates overlapping supervisors during an explicit restart.
func newControlAddress() string {
	return defaultControlAddress() + "-" + rand.Text()[:12]
}
