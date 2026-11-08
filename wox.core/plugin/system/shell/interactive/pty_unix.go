//go:build darwin || linux

package interactive

import (
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

// openPTY starts the shell as leader of its own session and assigns the slave as its controlling terminal.
func openPTY(options Options, size Size) (transport, error) {
	cmd := exec.Command(options.Interpreter, "-i")
	cmd.Dir = options.Directory
	cmd.Env = append(os.Environ(), "TERM=xterm-256color", "COLORTERM=truecolor")
	original, slave, err := pty.Open()
	if err != nil {
		return transport{}, err
	}
	defer original.Close()
	defer slave.Close()
	if err := unix.IoctlSetWinsize(int(original.Fd()), unix.TIOCSWINSZ, unixPTYSize(size)); err != nil {
		return transport{}, err
	}
	// Give Go's poller a nonblocking master so shutdown can interrupt a blocked read or write.
	fd, err := unix.FcntlInt(original.Fd(), unix.F_DUPFD_CLOEXEC, 0)
	if err != nil {
		return transport{}, err
	}
	if err := unix.SetNonblock(fd, true); err != nil {
		_ = unix.Close(fd)
		return transport{}, err
	}
	master := os.NewFile(uintptr(fd), "shell-pty")
	control, err := master.SyscallConn()
	if err != nil {
		_ = master.Close()
		return transport{}, err
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = slave, slave, slave
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true, Setctty: true}
	if err := cmd.Start(); err != nil {
		_ = master.Close()
		return transport{}, err
	}
	return transport{
		output: master,
		input:  master,
		resize: func(size Size) error {
			var resizeErr error
			if err := control.Control(func(fd uintptr) { resizeErr = unix.IoctlSetWinsize(int(fd), unix.TIOCSWINSZ, unixPTYSize(size)) }); err != nil {
				return err
			}
			return resizeErr
		},
		wait: cmd.Wait,
		stop: func(canceled bool) {
			if canceled {
				// Give the interactive shell time to forward hangup to its own jobs.
				_ = cmd.Process.Signal(unix.SIGHUP)
				time.Sleep(150 * time.Millisecond)
				_ = cmd.Process.Kill()
			}
			for _, pid := range unixProcessIDs() {
				if sid, err := unix.Getsid(pid); err == nil && sid == cmd.Process.Pid {
					_ = unix.Kill(pid, unix.SIGKILL)
				}
			}
			// A detached process may still have inherited the terminal FD.
			_ = master.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
			_ = master.SetWriteDeadline(time.Now())
		},
	}, nil
}

func unixPTYSize(size Size) *unix.Winsize {
	return &unix.Winsize{
		Col: uint16(size.Columns), Row: uint16(size.Rows),
		Xpixel: uint16(size.Columns * size.CellWidthPixels), Ypixel: uint16(size.Rows * size.CellHeightPixels),
	}
}
