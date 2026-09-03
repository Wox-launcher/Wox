package shell

// TerminateProcessTree kills pid and every descendant still linked through parent ids.
// Windows TerminateProcess does not touch children, which is why a uvx trampoline can leave
// uv.exe behind after the process Wox started has already exited.
func TerminateProcessTree(pid int) {
	if pid <= 0 {
		return
	}
	terminateProcessTree(pid)
}
