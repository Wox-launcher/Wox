package shell

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"unsafe"
	"wox/util"

	"golang.org/x/sys/windows"
)

const processJobCreateSuspended = 0x00000004

var (
	processJobNtdll         = windows.NewLazySystemDLL("ntdll.dll")
	processJobResumeProcess = processJobNtdll.NewProc("NtResumeProcess")

	lifetimeJobOnce   sync.Once
	lifetimeJobHandle windows.Handle
)

// lifetimeJob returns a job object that kills every process inside it as soon as the last handle
// to the job is closed. Wox holds the only handle, so the kernel performs the cleanup even when
// Wox is killed without running any shutdown code. This is what a Wox-side kill loop cannot offer.
func lifetimeJob(ctx context.Context) windows.Handle {
	lifetimeJobOnce.Do(func() {
		handle, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to create the process lifetime job: %s", err))
			return
		}
		limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		if _, err = windows.SetInformationJobObject(handle, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to configure the process lifetime job: %s", err))
			windows.CloseHandle(handle)
			return
		}
		lifetimeJobHandle = handle
	})
	return lifetimeJobHandle
}

func prepareLifetimeBoundCmd(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= processJobCreateSuspended
}

func adoptLifetimeBoundCmd(ctx context.Context, cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return fmt.Errorf("cannot adopt a command that has not been started")
	}

	access := uint32(windows.PROCESS_SET_QUOTA | windows.PROCESS_TERMINATE | windows.PROCESS_SUSPEND_RESUME)
	handle, err := windows.OpenProcess(access, false, uint32(cmd.Process.Pid))
	if err != nil {
		// The child is still suspended and now unreachable, so kill it rather than leaving a
		// process that holds the transport pipes open but never runs.
		_ = cmd.Process.Kill()
		return fmt.Errorf("failed to open process %d for adoption: %w", cmd.Process.Pid, err)
	}
	defer windows.CloseHandle(handle)

	if job := lifetimeJob(ctx); job != 0 {
		if err = windows.AssignProcessToJobObject(job, handle); err != nil {
			util.GetLogger().Error(ctx, fmt.Sprintf("failed to bind process %d to the lifetime job, its tree may outlive Wox: %s", cmd.Process.Pid, err))
		}
	}

	// The child was created suspended, so it has to be resumed whatever happened above.
	if status, _, _ := processJobResumeProcess.Call(uintptr(handle)); status != 0 {
		_ = windows.TerminateProcess(handle, 1)
		return fmt.Errorf("failed to resume process %d, NtResumeProcess status 0x%x", cmd.Process.Pid, uint32(status))
	}
	return nil
}
