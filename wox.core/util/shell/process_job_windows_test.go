package shell

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
	"unsafe"
	"wox/util/processmemory"

	"golang.org/x/sys/windows"
)

const lifetimeJobHelperEnv = "WOX_TEST_LIFETIME_JOB_HELPER"

// stillActiveExitCode is the exit code Windows reports for a process that has not exited.
const stillActiveExitCode = 259

// TestLifetimeBoundCmdRunsAfterAdoption covers the risk that comes with creating the child
// suspended: if adoption fails to resume it, the process would exist but never execute and the
// caller would wait on it forever.
func TestLifetimeBoundCmdRunsAfterAdoption(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "echo", "adopted")
	var output bytes.Buffer
	cmd.Stdout = &output

	PrepareLifetimeBoundCmd(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := AdoptLifetimeBoundCmd(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output.String(), "adopted") {
		t.Fatalf("adopted command produced %q, want it to have actually run", output.String())
	}
}

// TestLifetimeBoundCmdJoinsTheKillOnCloseJob checks the property the cleanup relies on. Every
// descendant of a job member joins the same job, so membership of the first process is what makes
// the whole tree die with Wox.
func TestLifetimeBoundCmdJoinsTheKillOnCloseJob(t *testing.T) {
	job := lifetimeJob(context.Background())
	if job == 0 {
		t.Skip("the lifetime job is unavailable on this system")
	}

	var limits windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	if err := windows.QueryInformationJobObject(job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits)), nil); err != nil {
		t.Fatal(err)
	}
	if limits.BasicLimitInformation.LimitFlags&windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE == 0 {
		t.Fatal("the lifetime job must kill its members once Wox drops the last handle")
	}

	cmd := exec.Command("cmd", "/c", "echo", "bound")
	cmd.Stdout = &bytes.Buffer{}
	PrepareLifetimeBoundCmd(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer cmd.Wait()

	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_INFORMATION, false, uint32(cmd.Process.Pid))
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(handle)

	if err = AdoptLifetimeBoundCmd(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	var inJob int32
	isProcessInJob := windows.NewLazySystemDLL("kernel32.dll").NewProc("IsProcessInJob")
	if ok, _, callErr := isProcessInJob.Call(uintptr(handle), uintptr(job), uintptr(unsafe.Pointer(&inJob))); ok == 0 {
		t.Fatalf("IsProcessInJob failed: %v", callErr)
	}
	if inJob == 0 {
		t.Fatal("an adopted process must be a member of the lifetime job")
	}
}

// TestLifetimeBoundTreeDiesWithAnAbruptExit covers the guarantee a Wox-side kill loop cannot
// give: the tree has to disappear even when the parent is terminated without running any cleanup.
// The test re-runs this binary as a helper, lets it start a small process tree inside the job,
// kills the helper outright, and then requires every descendant to be gone.
func TestLifetimeBoundTreeDiesWithAnAbruptExit(t *testing.T) {
	if os.Getenv(lifetimeJobHelperEnv) != "" {
		runLifetimeJobHelper(t)
		return
	}

	helper := exec.Command(os.Args[0], "-test.run=TestLifetimeBoundTreeDiesWithAnAbruptExit")
	helper.Env = append(os.Environ(), lifetimeJobHelperEnv+"=1")
	stdout, err := helper.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err = helper.Start(); err != nil {
		t.Fatal(err)
	}
	defer helper.Wait()

	line, err := bufio.NewReader(stdout).ReadString('\n')
	if err != nil || !strings.HasPrefix(line, "tree ") {
		t.Fatalf("helper did not report a started tree, output %q, err %v", line, err)
	}

	descendants, err := processmemory.ListDescendantProcesses(helper.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if len(descendants) < 2 {
		t.Fatalf("the helper must have a live tree of its own before it is killed, got %#v", descendants)
	}

	if err = helper.Process.Kill(); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(15 * time.Second)
	for {
		alive := livePids(descendants)
		if len(alive) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("%v survived the parent, so the lifetime job did not kill the tree", alive)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// runLifetimeJobHelper starts a tree that would otherwise outlive its parent, then blocks so the
// test can kill it while the tree is still running.
func runLifetimeJobHelper(t *testing.T) {
	cmd := exec.Command("cmd", "/c", "ping -n 200 127.0.0.1")
	PrepareLifetimeBoundCmd(cmd)
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := AdoptLifetimeBoundCmd(context.Background(), cmd); err != nil {
		t.Fatal(err)
	}
	// The grandchild only exists once cmd.exe has spawned ping, and the whole point of the test
	// is to kill the parent while that grandchild is running.
	time.Sleep(2 * time.Second)
	os.Stdout.WriteString("tree " + strconv.Itoa(cmd.Process.Pid) + "\n")
	time.Sleep(2 * time.Minute)
}

// livePids reports which of the given processes still exist.
func livePids(processes []processmemory.DescendantProcess) []int {
	var alive []int
	for _, process := range processes {
		handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(process.ProcessID))
		if err != nil {
			continue
		}
		var exitCode uint32
		if err = windows.GetExitCodeProcess(handle, &exitCode); err == nil && exitCode == stillActiveExitCode {
			alive = append(alive, process.ProcessID)
		}
		windows.CloseHandle(handle)
	}
	return alive
}
