package diagnostic

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
	"wox/util"
)

func TestSupervisedChildArgsPreserveTroubleshootingMode(t *testing.T) {
	args := []string{"wox.exe", ArgSupervisor, ArgWaitParent, "1234", util.ArgNoThirdPartyPlugins, "wox://query?q=test"}
	for _, firstLaunch := range []bool{true, false} {
		want := []string{ArgChild, util.ArgNoThirdPartyPlugins}
		if firstLaunch {
			want = append(want, "wox://query?q=test")
		}
		if got := supervisedChildArgs(args, firstLaunch); !reflect.DeepEqual(got, want) {
			t.Fatalf("supervisedChildArgs(firstLaunch=%t) = %v, want %v", firstLaunch, got, want)
		}
	}
	if got := supervisedChildArgs([]string{"wox.exe", ArgSupervisor, "wox://query?q=test"}, false); !reflect.DeepEqual(got, []string{ArgChild}) {
		t.Fatalf("normal crash restart arguments = %v", got)
	}
	if got := supervisedChildArgs([]string{"wox.exe", ArgSupervisor, ArgChild}, true); !reflect.DeepEqual(got, []string{ArgChild}) {
		t.Fatalf("explicit normal restart arguments = %v", got)
	}
}

func TestForwardedProcessArgs(t *testing.T) {
	args := []string{
		"wox.exe",
		ArgSupervisor,
		ArgWaitParent,
		"1234",
		ArgChild,
		"--updated",
		"wox://query?q=test",
	}
	want := []string{"--updated", "wox://query?q=test"}
	if got := forwardedProcessArgs(args); !reflect.DeepEqual(got, want) {
		t.Fatalf("forwardedProcessArgs() = %v, want %v", got, want)
	}
}

func TestNextCrashRestartBoundsCrashLoops(t *testing.T) {
	count, restart := nextCrashRestart(time.Second, 0)
	if count != 1 || !restart {
		t.Fatalf("first crash returned count=%d restart=%t", count, restart)
	}
	count, restart = nextCrashRestart(time.Second, count)
	if count != 2 || !restart {
		t.Fatalf("second crash returned count=%d restart=%t", count, restart)
	}
	count, restart = nextCrashRestart(time.Second, count)
	if count != 3 || restart {
		t.Fatalf("third crash returned count=%d restart=%t", count, restart)
	}
}

func TestNextCrashRestartResetsAfterStableRun(t *testing.T) {
	count, restart := nextCrashRestart(crashLoopWindow, maxConsecutiveCrashRestarts)
	if count != 1 || !restart {
		t.Fatalf("stable run returned count=%d restart=%t", count, restart)
	}
}

func TestRetainNewestCrashFiles(t *testing.T) {
	directory := t.TempDir()
	baseTime := time.Now().Add(-time.Hour)
	for i := 0; i < 7; i++ {
		path := filepath.Join(directory, fmt.Sprintf("crash-%d.zip", i))
		if err := os.WriteFile(path, []byte("report"), 0644); err != nil {
			t.Fatal(err)
		}
		modifiedAt := baseTime.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(path, modifiedAt, modifiedAt); err != nil {
			t.Fatal(err)
		}
	}

	GetManager().retainNewestCrashFiles(directory, ".zip", 5)
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 5 {
		t.Fatalf("retained %d reports, want 5", len(entries))
	}
	for _, removed := range []string{"crash-0.zip", "crash-1.zip"} {
		if _, err := os.Stat(filepath.Join(directory, removed)); !os.IsNotExist(err) {
			t.Fatalf("old report %s was not removed", removed)
		}
	}
}
