//go:build windows

package single_instance

import (
	"os"
	"syscall"
	"unsafe"
	"wox/util"
)

const (
	LOCKFILE_EXCLUSIVE_LOCK = 2
)

var (
	kernel32       = syscall.MustLoadDLL("kernel32.dll")
	procLockFileEx = kernel32.MustFindProc("LockFileEx")
)

func LockFileEx(hFile syscall.Handle, dwFlags, dwReserved, nNumberOfBytesToLockLow, nNumberOfBytesToLockHigh uint32, lpOverlapped *syscall.Overlapped) (err error) {
	r1, _, e1 := syscall.Syscall6(procLockFileEx.Addr(), 6, uintptr(hFile), uintptr(dwFlags), uintptr(dwReserved), uintptr(nNumberOfBytesToLockLow), uintptr(nNumberOfBytesToLockHigh), uintptr(unsafe.Pointer(lpOverlapped)))
	if r1 == 0 {
		if e1 != 0 {
			err = error(e1)
		} else {
			err = syscall.EINVAL
		}
	}
	return
}

func lock(content string) error {
	filename := util.GetLocation().GetAppLockFilePath()
	file, err := os.OpenFile(filename, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()

	var overlapped syscall.Overlapped
	err = LockFileEx(syscall.Handle(file.Fd()), LOCKFILE_EXCLUSIVE_LOCK, 0, 1, 0, &overlapped)
	if err != nil {
		return err
	}

	if _, err := file.WriteString(content); err != nil {
		return err
	}

	return nil
}
