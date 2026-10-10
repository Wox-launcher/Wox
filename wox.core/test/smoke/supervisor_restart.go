package smoke

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/test/automationdriver"
	"wox/ui/automation"
)

var startupPIDPattern = regexp.MustCompile(`startup pid: (\d+)`)

// WaitForSupervisorReplacement waits until the supervisor replaces the current Wox process.
// previous is the endpoint that died, previousPID is that process, and logMarker is the supervisor.log line that explains why it was replaced.
func WaitForSupervisorReplacement(t *testing.T, ctx context.Context, previous automation.Info, previousPID int, logMarker string) (*automationdriver.Client, int) {
	t.Helper()
	infoFile := strings.TrimSpace(os.Getenv(automationdriver.SharedInfoFileEnvironment))
	dataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedDataDirectoryEnvironment))
	if infoFile == "" || dataDirectory == "" {
		t.Fatal("supervisor restart requires the shared automation endpoint and data directory")
	}
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	var lastPID int
	var lastAddress string
	for {
		logText, _ := os.ReadFile(filepath.Join(dataDirectory, "log", "supervisor.log"))
		pid := latestStartupPID(filepath.Join(dataDirectory, "log", "wox.log"))
		info := readPublishedEndpoint(infoFile)
		lastPID = pid
		lastAddress = info.Address
		if strings.Contains(string(logText), logMarker) && pid > 0 && pid != previousPID && info.Address != "" && info.Address != previous.Address && info.Token != "" {
			client, err := automationdriver.NewClient(info)
			if err == nil {
				probe, probeCancel := context.WithTimeout(ctx, time.Second)
				_, probeErr := client.Snapshot(probe)
				probeCancel()
				if probeErr == nil {
					return client, pid
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatalf("wait for supervisor replacement %q: pid %d -> %d, address %q -> %q: %v", logMarker, previousPID, lastPID, previous.Address, lastAddress, ctx.Err())
		case <-ticker.C:
		}
	}
}

// LatestStartupPID reads the newest Wox process id from the current log.
func LatestStartupPID(t *testing.T) int {
	t.Helper()
	dataDirectory := strings.TrimSpace(os.Getenv(automationdriver.SharedDataDirectoryEnvironment))
	pid := latestStartupPID(filepath.Join(dataDirectory, "log", "wox.log"))
	if pid <= 0 {
		t.Fatalf("startup pid was not found in %s", filepath.Join(dataDirectory, "log", "wox.log"))
	}
	return pid
}

func latestStartupPID(path string) int {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	matches := startupPIDPattern.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return 0
	}
	pid, _ := strconv.Atoi(matches[len(matches)-1][1])
	return pid
}

func readPublishedEndpoint(path string) automation.Info {
	data, err := os.ReadFile(path)
	if err != nil {
		return automation.Info{}
	}
	var info automation.Info
	if err := json.Unmarshal(data, &info); err != nil {
		return automation.Info{}
	}
	return info
}
