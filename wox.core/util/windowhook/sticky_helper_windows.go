//go:build windows

package windowhook

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"wox/util"

	"golang.org/x/sys/windows"
)

// HelperDLLPath returns the hook DLL built for the given bitness.
func HelperDLLPath(is32Bit bool) string {
	name := "WoxWindowHook64.dll"
	if is32Bit {
		name = "WoxWindowHook32.dll"
	}
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "window_hook", name)
}

// HelperPath returns the 32-bit helper that installs the hook in 32-bit targets.
func HelperPath() string {
	return filepath.Join(util.GetLocation().GetOthersDirectory(), "window_hook", "wox-window-hook-helper32.exe")
}

// helperAttachTimeout bounds the wait for the helper's first line. The attach itself
// posts a message to the target thread and waits on an event, so a wedged dialog would
// otherwise hold the overlay's setup open indefinitely.
const helperAttachTimeout = 3 * time.Second

// helperCommandTimeout bounds one navigation or selection run through the helper. The DLL
// already caps its own wait on the target thread; this only guards the process around it.
const helperCommandTimeout = 3 * time.Second

// HelperHook owns a helper process that holds an injected subclass in a target whose
// bitness differs from Wox's. It exposes the same shape as StickyHook so the overlay
// runtime does not branch on which mechanism attached.
type HelperHook struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	target uintptr
	once   sync.Once
}

// TargetIsWow64 reports whether the process runs under WOW64, which on a 64-bit Windows
// means it is a 32-bit process.
func TargetIsWow64(pid int) (bool, error) {
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return false, err
	}
	defer windows.CloseHandle(handle)

	var wow64 bool
	if err := windows.IsWow64Process(handle, &wow64); err != nil {
		return false, err
	}
	return wow64, nil
}

// hostIsWow64 reports whether Wox itself is a 32-bit process on a 64-bit Windows. Both
// sides have to be compared: only a difference forces the helper.
func hostIsWow64() bool {
	var wow64 bool
	if err := windows.IsWow64Process(windows.CurrentProcess(), &wow64); err != nil {
		return false
	}
	return wow64
}

// NeedsBitnessHelper reports whether the target can only be hooked from a helper process.
func NeedsBitnessHelper(pid int) bool {
	targetWow64, err := TargetIsWow64(pid)
	if err != nil {
		return false
	}
	return targetWow64 != hostIsWow64()
}

// RunDialogCommandViaHelper performs one dialog command in a target whose bitness differs
// from Wox's.
//
// Running it in-process instead does not fail fast: the hook installs and the message is
// posted, but the target can never load a DLL of the wrong bitness, so the call sits out
// the DLL's full timeout before reporting failure. That dead second is what separates a
// jump that feels instant from one that visibly stalls.
func RunDialogCommandViaHelper(ctx context.Context, command string, target uintptr, pid int, targetPath string) bool {
	if target == 0 || pid <= 0 || strings.TrimSpace(targetPath) == "" {
		return false
	}

	runCtx, cancel := context.WithTimeout(ctx, helperCommandTimeout)
	defer cancel()

	helper := HelperPath()
	cmd := exec.CommandContext(runCtx, helper,
		fmt.Sprintf("-dll=%s", HelperDLLPath(true)),
		fmt.Sprintf("-command=%s", command),
		fmt.Sprintf("-target=%d", target),
		fmt.Sprintf("-pid=%d", pid),
	)
	// The helper is a console binary; without this a window flashes on every command.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	// The path travels on stdin so it stays out of the helper's command line.
	cmd.Stdin = strings.NewReader(targetPath)

	output, err := cmd.Output()
	line := strings.TrimSpace(string(output))
	if err != nil || line != "ok" {
		util.GetLogger().Debug(ctx, fmt.Sprintf("dialog helper %s refused: pid=%d target=0x%X detail=%q err=%v", command, pid, target, line, err))
		return false
	}
	return true
}

// AttachStickyViaHelper installs the subclass through a helper built for the target's
// bitness and keeps that helper alive for as long as the overlay is attached.
func AttachStickyViaHelper(target uintptr, pid int, overlayHWND uintptr) *HelperHook {
	ctx := context.Background()
	if target == 0 || pid <= 0 || overlayHWND == 0 {
		return nil
	}

	helper := HelperPath()
	dll := HelperDLLPath(true)
	cmd := exec.Command(helper,
		fmt.Sprintf("-dll=%s", dll),
		fmt.Sprintf("-target=%d", target),
		fmt.Sprintf("-pid=%d", pid),
		fmt.Sprintf("-overlay=%d", overlayHWND),
	)
	// The helper is a console binary; without this a window flashes on every attach.
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("sticky helper stdin failed: err=%v", err))
		return nil
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("sticky helper stdout failed: err=%v", err))
		return nil
	}
	if err := cmd.Start(); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("sticky helper start failed: helper=%q err=%v", helper, err))
		return nil
	}

	result := make(chan string, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		line, _ := reader.ReadString('\n')
		result <- strings.TrimSpace(line)
		// Drain so the helper never blocks writing, and so its exit is not masked.
		_, _ = io.Copy(io.Discard, reader)
	}()

	var line string
	select {
	case line = <-result:
	case <-time.After(helperAttachTimeout):
		util.GetLogger().Error(ctx, fmt.Sprintf("sticky helper timed out: pid=%d target=0x%X", pid, target))
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return nil
	}

	if line != "ok" {
		util.GetLogger().Error(ctx, fmt.Sprintf("sticky helper refused: pid=%d target=0x%X detail=%q", pid, target, line))
		_ = stdin.Close()
		_ = cmd.Process.Kill()
		return nil
	}

	return &HelperHook{cmd: cmd, stdin: stdin, target: target}
}

// PublishStickyOffset republishes the offset the injected subclass reads on every move.
func (hook *HelperHook) PublishStickyOffset(overlayHWND uintptr) {
	if hook == nil {
		return
	}
	publishStickyOffset(hook.target, overlayHWND)
}

// Detach closes the helper's stdin, which is its signal to remove the subclass and exit.
func (hook *HelperHook) Detach() bool {
	if hook == nil {
		return true
	}
	hook.once.Do(func() {
		_ = hook.stdin.Close()
		done := make(chan struct{})
		go func() {
			_ = hook.cmd.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(helperAttachTimeout):
			// The subclass is removed by the target on WM_NCDESTROY anyway, so a stuck
			// helper must not keep Wox's overlay teardown waiting.
			_ = hook.cmd.Process.Kill()
			<-done
		}
		clearStickyOffsetProps(hook.target)
		hook.target = 0
	})
	return true
}
