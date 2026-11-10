//go:build darwin || linux

package interactive

import (
	_ "embed"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/sys/unix"
)

//go:embed integration.bash
var bashIntegration string

//go:embed integration.zsh
var zshIntegration string

// openPTY starts the shell as leader of its own session and assigns the slave as its controlling terminal.
func openPTY(options Options, size Size, token string) (result transport, err error) {
	args, env, cleanup, err := unixShellIntegration(options.Interpreter, token)
	if err != nil {
		return transport{}, err
	}
	defer func() {
		if err != nil {
			cleanup()
		}
	}()
	cmd := exec.Command(options.Interpreter, args...)
	cmd.Dir = options.Directory
	cmd.Env = append(env, "TERM=xterm-256color", "COLORTERM=truecolor", "WOX_SHELL_SCROLLBACK="+options.scrollbackFile)
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
		wait: func() error {
			defer cleanup()
			return cmd.Wait()
		},
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

// unixShellIntegration stages startup hooks without editing the user's shell configuration.
func unixShellIntegration(interpreter, token string) ([]string, []string, func(), error) {
	env := os.Environ()
	if interpreter != "bash" && interpreter != "zsh" {
		return []string{"-i"}, env, func() {}, nil
	}
	dir, err := os.MkdirTemp("", "wox-shell-")
	if err != nil {
		return nil, nil, nil, err
	}
	file, script := "bashrc", bashIntegration
	if interpreter == "zsh" {
		file, script = ".zshenv", zshIntegration
		original, set := os.LookupEnv("ZDOTDIR")
		flag := "0"
		if set {
			flag = "1"
		}
		env = append(env, "ZDOTDIR="+dir, "WOX_ZDOTDIR_SET="+flag, "WOX_ORIGINAL_ZDOTDIR="+original)
	}
	path := filepath.Join(dir, file)
	cleanup := func() {
		_ = os.Remove(path)
		_ = os.Remove(dir)
	}
	if err := os.WriteFile(path, []byte(strings.ReplaceAll(script, "@WOX_TOKEN@", token)), 0600); err != nil {
		cleanup()
		return nil, nil, nil, err
	}
	if interpreter == "bash" {
		return []string{"--rcfile", path, "-i"}, env, cleanup, nil
	}
	return []string{"-i"}, env, cleanup, nil
}

func unixPTYSize(size Size) *unix.Winsize {
	return &unix.Winsize{
		Col: uint16(size.Columns), Row: uint16(size.Rows),
		Xpixel: uint16(size.Columns * size.CellWidthPixels), Ypixel: uint16(size.Rows * size.CellHeightPixels),
	}
}
