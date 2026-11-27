//go:build windows

package windowhook

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const mockHookDLLSource = `#include <windows.h>
#include <stdio.h>

typedef struct {
    DWORD stage;
    DWORD win32Error;
    LONG hresult;
    DWORD targetPid;
    DWORD targetThread;
    DWORD shellViewFound;
    DWORD hookInstalled;
    DWORD callbackEntered;
    DWORD waitResult;
} WoxWindowHookDiagnostic;

static void writeLog(const char *line) {
    const char *path = getenv("WOX_MOCK_HOOK_LOG");
    if (path == NULL || *path == '\0') return;
    FILE *f = fopen(path, "ab");
    if (f == NULL) return;
    fputs(line, f);
    fputc('\n', f);
    fclose(f);
}

__declspec(dllexport) void *WINAPI WoxWindowHookAttachSticky(HWND target, DWORD expectedPid, HWND overlay, WoxWindowHookDiagnostic *diag) {
    char buf[256];
    snprintf(buf, sizeof(buf), "attach:%llu:%lu:%llu", (unsigned long long)(uintptr_t)target, (unsigned long)expectedPid, (unsigned long long)(uintptr_t)overlay);
    writeLog(buf);
    if (expectedPid == 99999) {
        if (diag != NULL) {
            diag->stage = 6;
            diag->win32Error = 5;
            diag->hresult = (LONG)0x80070005L;
            diag->targetPid = expectedPid;
        }
        return NULL;
    }
    return (void *)(uintptr_t)0x1234;
}

__declspec(dllexport) BOOL WINAPI WoxWindowHookDetachSticky(void *handle) {
    char buf[128];
    snprintf(buf, sizeof(buf), "detach:%llu", (unsigned long long)(uintptr_t)handle);
    writeLog(buf);
    return TRUE;
}

static BOOL handleDialog(const char *op, HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diag) {
    char utf8[1024];
    int n = WideCharToMultiByte(CP_UTF8, 0, targetPath, -1, utf8, sizeof(utf8), NULL, NULL);
    if (n <= 0) utf8[0] = '\0';
    char buf[1280];
    snprintf(buf, sizeof(buf), "%s:%llu:%lu:%s", op, (unsigned long long)(uintptr_t)dialog, (unsigned long)expectedPid, utf8);
    writeLog(buf);
    if (expectedPid == 88888) {
        if (diag != NULL) {
            diag->stage = 14;
            diag->win32Error = 2;
            diag->hresult = (LONG)0x80070002L;
            diag->targetPid = expectedPid;
            diag->hookInstalled = 1;
        }
        return FALSE;
    }
    return TRUE;
}

__declspec(dllexport) BOOL WINAPI WoxWindowHookNavigateDialog(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diag) {
    return handleDialog("navigate", dialog, expectedPid, targetPath, diag);
}

__declspec(dllexport) BOOL WINAPI WoxWindowHookSelectDialogItem(HWND dialog, DWORD expectedPid, const WCHAR *targetPath, WoxWindowHookDiagnostic *diag) {
    return handleDialog("select", dialog, expectedPid, targetPath, diag);
}
`

func compileCHelperForTest(t *testing.T) (helperExe string, mockDLL string) {
	t.Helper()
	gccPath, err := exec.LookPath("gcc")
	if err != nil {
		t.Skip("gcc not available on PATH")
	}

	dir := t.TempDir()
	helperSrc, err := filepath.Abs(filepath.Join("hook", "window_hook_helper.c"))
	if err != nil {
		t.Fatal(err)
	}
	helperExe = filepath.Join(dir, "wox-window-hook-helper-test.exe")
	out, err := exec.Command(gccPath, "-O2", "-s", "-static-libgcc", "-Wall", "-Wextra", "-Werror", "-o", helperExe, helperSrc, "-lshell32").CombinedOutput()
	if err != nil {
		t.Fatalf("compile window_hook_helper.c failed: %v\n%s", err, out)
	}

	mockSrc := filepath.Join(dir, "mock_hook.c")
	if err := os.WriteFile(mockSrc, []byte(mockHookDLLSource), 0o644); err != nil {
		t.Fatal(err)
	}
	mockDLL = filepath.Join(dir, "mock_hook.dll")
	out, err = exec.Command(gccPath, "-shared", "-O2", "-s", "-static-libgcc", "-Wall", "-Wextra", "-o", mockDLL, mockSrc).CombinedOutput()
	if err != nil {
		t.Fatalf("compile mock_hook.dll failed: %v\n%s", err, out)
	}
	return helperExe, mockDLL
}

func runHelperCapture(t *testing.T, exe string, logPath string, stdin string, args ...string) (int, string) {
	t.Helper()
	cmd := exec.Command(exe, args...)
	cmd.Env = append(os.Environ(), "WOX_MOCK_HOOK_LOG="+logPath)
	cmd.Stdin = strings.NewReader(stdin)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			t.Fatalf("run helper failed unexpectedly: %v", err)
		}
	}
	return code, strings.TrimSpace(buf.String())
}

func TestWindowHookHelperCArgumentAndErrorHandling(t *testing.T) {
	helperExe, mockDLL := compileCHelperForTest(t)
	logPath := filepath.Join(t.TempDir(), "calls.log")

	if code, out := runHelperCapture(t, helperExe, logPath, "", "-target=1"); code != 2 || out != "fail reason=invalid-arguments" {
		t.Fatalf("missing args: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, "", "-dll="+mockDLL, "-command=bogus", "-target=1", "-pid=2"); code != 2 || !strings.Contains(out, "fail reason=unknown-command") {
		t.Fatalf("unknown cmd: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, "", "-dll="+filepath.Join(t.TempDir(), "missing.dll"), "-target=1", "-pid=2", "-overlay=3"); code != 3 || !strings.HasPrefix(out, "fail reason=load-dll") {
		t.Fatalf("missing dll: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, "", "-dll="+mockDLL, "-command=sticky", "-target=100", "-pid=99999", "-overlay=200"); code != 5 || !strings.Contains(out, "fail reason=attach stage=6") {
		t.Fatalf("attach failure: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, "\r\n", "-dll="+mockDLL, "-command=navigate", "-target=100", "-pid=200"); code != 2 || out != "fail reason=empty-path" {
		t.Fatalf("empty path: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, `C:\test`, "-dll="+mockDLL, "-command=navigate", "-target=100", "-pid=88888"); code != 5 || !strings.Contains(out, "fail reason=command stage=14") {
		t.Fatalf("command failure: code=%d out=%q", code, out)
	}
}

func TestWindowHookHelperCStickyAndDialogCommands(t *testing.T) {
	helperExe, mockDLL := compileCHelperForTest(t)
	logPath := filepath.Join(t.TempDir(), "calls.log")

	unicodePath := `C:\测试目录\示例 文件.txt`
	if code, out := runHelperCapture(t, helperExe, logPath, unicodePath+"\r\n", "-dll="+mockDLL, "-command=navigate", "-target=111", "-pid=222"); code != 0 || out != "ok" {
		t.Fatalf("navigate failed: code=%d out=%q", code, out)
	}
	if code, out := runHelperCapture(t, helperExe, logPath, unicodePath+"\n", "-dll="+mockDLL, "-command=select", "-target=333", "-pid=444"); code != 0 || out != "ok" {
		t.Fatalf("select failed: code=%d out=%q", code, out)
	}

	// Verify sticky waits on stdin and detaches cleanly on EOF.
	cmd := exec.Command(helperExe, fmt.Sprintf("-dll=%s", mockDLL), "-command=sticky", "-target=555", "-pid=666", "-overlay=777")
	cmd.Env = append(os.Environ(), "WOX_MOCK_HOOK_LOG="+logPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	lineCh := make(chan string, 1)
	go func() {
		line, _ := bufio.NewReader(stdout).ReadString('\n')
		lineCh <- strings.TrimSpace(line)
		_, _ = io.Copy(io.Discard, stdout)
	}()

	select {
	case line := <-lineCh:
		if line != "ok" {
			t.Fatalf("sticky first line = %q, want ok", line)
		}
	case <-time.After(3 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatal("timed out waiting for sticky ok")
	}

	_ = stdin.Close()
	if err := cmd.Wait(); err != nil {
		t.Fatalf("sticky exit err = %v", err)
	}

	loggedBytes, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatal(err)
	}
	logged := string(loggedBytes)
	for _, want := range []string{
		"navigate:111:222:" + unicodePath,
		"select:333:444:" + unicodePath,
		"attach:555:666:777",
		fmt.Sprintf("detach:%d", 0x1234),
	} {
		if !strings.Contains(logged, want) {
			t.Fatalf("mock hook log missing %q:\n%s", want, logged)
		}
	}
}
