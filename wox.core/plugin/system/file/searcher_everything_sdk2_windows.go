// Search Everything 1.4 SDK : https://www.voidtools.com/support/everything/sdk/
package file

import (
	"syscall"
	"time"
	"unsafe"
)

const (
	everything2ErrorIPC = 2
)

const (
	everything2RequestFileName   = 0x00000001
	everything2RequestPath       = 0x00000002
	everything2RequestSize       = 0x00000010
	everything2RequestDateModify = 0x00000040
)

var (
	everything2SetSearch        *syscall.LazyProc
	everything2SetMatchPath     *syscall.LazyProc
	everything2SetMatchCase     *syscall.LazyProc
	everything2SetMatchWhole    *syscall.LazyProc
	everything2SetRegex         *syscall.LazyProc
	everything2SetRequestFlags  *syscall.LazyProc
	everything2Query            *syscall.LazyProc
	everything2GetLastError     *syscall.LazyProc
	everything2GetNumResults    *syscall.LazyProc
	everything2GetResultFull    *syscall.LazyProc
	everything2GetResultSize    *syscall.LazyProc
	everything2GetResultModTime *syscall.LazyProc
	everything2IsFolderResult   *syscall.LazyProc
	everything2Reset            *syscall.LazyProc
)

func initEverything2DLL(dllPath string) {
	dll := syscall.NewLazyDLL(dllPath)
	if dll == nil {
		return
	}
	everything2SetSearch = dll.NewProc("Everything_SetSearchW")
	everything2SetMatchPath = dll.NewProc("Everything_SetMatchPath")
	everything2SetMatchCase = dll.NewProc("Everything_SetMatchCase")
	everything2SetMatchWhole = dll.NewProc("Everything_SetMatchWholeWord")
	everything2SetRegex = dll.NewProc("Everything_SetRegex")
	everything2SetRequestFlags = dll.NewProc("Everything_SetRequestFlags")
	everything2Query = dll.NewProc("Everything_QueryW")
	everything2GetLastError = dll.NewProc("Everything_GetLastError")
	everything2GetNumResults = dll.NewProc("Everything_GetNumResults")
	everything2GetResultFull = dll.NewProc("Everything_GetResultFullPathNameW")
	everything2GetResultSize = dll.NewProc("Everything_GetResultSize")
	everything2GetResultModTime = dll.NewProc("Everything_GetResultDateModified")
	everything2IsFolderResult = dll.NewProc("Everything_IsFolderResult")
	everything2Reset = dll.NewProc("Everything_Reset")
}

func walkEverything2(root string, maxCount int, walkFn WalkFunc) error {
	if everything2SetSearch == nil || everything2Query == nil {
		return EverythingNotRunningError
	}

	setSearchBool2(everything2SetMatchPath, false)
	setSearchBool2(everything2SetMatchCase, false)
	setSearchBool2(everything2SetMatchWhole, false)
	setSearchBool2(everything2SetRegex, false)

	everything2SetSearch.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(root))))
	everything2SetRequestFlags.Call(uintptr(everything2RequestFileName | everything2RequestPath | everything2RequestSize | everything2RequestDateModify))

	ok, _, _ := everything2Query.Call(1)
	if ok == 0 {
		if getEverything2LastError() == everything2ErrorIPC {
			return EverythingNotRunningError
		}
		return EverythingNotRunningError
	}

	num := getEverything2NumResults()
	if maxCount > 0 && num > maxCount {
		num = maxCount
	}
	for i := 0; i < num; i++ {
		var fi FileInfo
		fi.name = getEverything2FullPathName(i)
		fi.size = getEverything2ResultSize(i)
		fi.modTime = getEverything2ResultDateModified(i)
		fi.isDir = isEverything2FolderResult(i)
		if err := walkFn(fi.name, fi, nil); err != nil {
			return err
		}
	}
	return nil
}

func setSearchBool2(proc *syscall.LazyProc, enabled bool) {
	if proc == nil {
		return
	}
	var value uintptr
	if enabled {
		value = 1
	}
	proc.Call(value)
}

func getEverything2LastError() int {
	if everything2GetLastError == nil {
		return 0
	}
	r, _, _ := everything2GetLastError.Call()
	return int(r)
}

func getEverything2NumResults() int {
	if everything2GetNumResults == nil {
		return 0
	}
	r, _, _ := everything2GetNumResults.Call()
	return int(r)
}

func getEverything2FullPathName(index int) string {
	if everything2GetResultFull == nil {
		return ""
	}
	pathbuf := make([]uint16, 32768)
	everything2GetResultFull.Call(
		uintptr(index),
		uintptr(unsafe.Pointer(&pathbuf[0])),
		uintptr(len(pathbuf)),
	)
	return syscall.UTF16ToString(pathbuf)
}

func getEverything2ResultSize(index int) int64 {
	if everything2GetResultSize == nil {
		return 0
	}
	var size int64
	everything2GetResultSize.Call(uintptr(index), uintptr(unsafe.Pointer(&size)))
	return size
}

func getEverything2ResultDateModified(index int) time.Time {
	if everything2GetResultModTime == nil {
		return time.Time{}
	}
	var ft syscall.Filetime
	everything2GetResultModTime.Call(uintptr(index), uintptr(unsafe.Pointer(&ft)))
	return time.Unix(0, ft.Nanoseconds())
}

func isEverything2FolderResult(index int) bool {
	if everything2IsFolderResult == nil {
		return false
	}
	r, _, _ := everything2IsFolderResult.Call(uintptr(index))
	return r != 0
}
