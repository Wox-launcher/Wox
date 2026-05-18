//go:build windows

package util

import (
	"fmt"
	"syscall"
	"unsafe"
)

var (
	processMemoryKernel32             = syscall.NewLazyDLL("kernel32.dll")
	processMemoryPSAPI                = syscall.NewLazyDLL("psapi.dll")
	processMemoryOpenProcess          = processMemoryKernel32.NewProc("OpenProcess")
	processMemoryCloseHandle          = processMemoryKernel32.NewProc("CloseHandle")
	processMemoryGetProcessMemoryInfo = processMemoryPSAPI.NewProc("GetProcessMemoryInfo")
	processMemoryQueryWorkingSet      = processMemoryPSAPI.NewProc("QueryWorkingSet")
)

const (
	processMemoryQueryInformation        = 0x0400
	processMemoryQueryLimitedInformation = 0x1000
	processMemoryVMRead                  = 0x0010
)

type processMemoryCountersEx struct {
	cb                         uint32
	pageFaultCount             uint32
	peakWorkingSetSize         uintptr
	workingSetSize             uintptr
	quotaPeakPagedPoolUsage    uintptr
	quotaPagedPoolUsage        uintptr
	quotaPeakNonPagedPoolUsage uintptr
	quotaNonPagedPoolUsage     uintptr
	pagefileUsage              uintptr
	peakPagefileUsage          uintptr
	privateUsage               uintptr
}

type processMemoryWorkingSetBlock struct {
	flags uintptr
}

type processMemoryWorkingSetInformation struct {
	numberOfEntries uintptr
	workingSetInfo  [1]processMemoryWorkingSetBlock
}

func getProcessMemoryBytes(pid int) (uint64, error) {
	if pid <= 0 {
		return 0, fmt.Errorf("invalid pid: %d", pid)
	}

	handle, err := openProcessForMemory(pid)
	if err != nil {
		return 0, err
	}
	defer processMemoryCloseHandle.Call(handle)

	var counters processMemoryCountersEx
	counters.cb = uint32(unsafe.Sizeof(counters))
	ret, _, callErr := processMemoryGetProcessMemoryInfo.Call(
		handle,
		uintptr(unsafe.Pointer(&counters)),
		uintptr(counters.cb),
	)
	if ret == 0 {
		return 0, fmt.Errorf("GetProcessMemoryInfo failed for pid %d: %w", pid, callErr)
	}
	if privateWorkingSet, privateErr := getPrivateWorkingSetBytes(handle); privateErr == nil && privateWorkingSet > 0 {
		// Bug fix: Task Manager's default Processes > Memory column is closer to
		// private working set than total working set. The previous Windows fix
		// returned WorkingSetSize, which included shared DLL and mapped pages and
		// made Flutter look much larger than Task Manager for the same PID.
		return uint64(privateWorkingSet), nil
	}
	if counters.pagefileUsage > 0 {
		// Windows may deny QueryWorkingSet for some processes. PagefileUsage is
		// the existing app-plugin fallback and keeps Wox Memory visible instead
		// of dropping the Glance item when private working set is unavailable.
		return uint64(counters.pagefileUsage), nil
	}

	return 0, fmt.Errorf("empty process memory counters for pid %d", pid)
}

func openProcessForMemory(pid int) (uintptr, error) {
	handle, _, _ := processMemoryOpenProcess.Call(
		uintptr(processMemoryQueryInformation|processMemoryVMRead),
		0,
		uintptr(pid),
	)
	if handle != 0 {
		return handle, nil
	}

	// Some processes deny VM_READ even though their memory counters are still
	// queryable. Limited information keeps the diagnostic useful for the Flutter
	// process without broadening the rest of the Glance flow.
	handle, _, callErr := processMemoryOpenProcess.Call(
		uintptr(processMemoryQueryLimitedInformation),
		0,
		uintptr(pid),
	)
	if handle == 0 {
		return 0, fmt.Errorf("OpenProcess failed for pid %d: %w", pid, callErr)
	}
	return handle, nil
}

func getPrivateWorkingSetBytes(handle uintptr) (uintptr, error) {
	var info processMemoryWorkingSetInformation
	processMemoryQueryWorkingSet.Call(
		handle,
		uintptr(unsafe.Pointer(&info)),
		unsafe.Sizeof(info),
	)
	if info.numberOfEntries == 0 || info.numberOfEntries > 1000000 {
		return 0, fmt.Errorf("invalid working set entries: %d", info.numberOfEntries)
	}

	bufferSize := unsafe.Sizeof(uintptr(0)) + info.numberOfEntries*unsafe.Sizeof(processMemoryWorkingSetBlock{})
	buffer := make([]byte, bufferSize)
	ret, _, callErr := processMemoryQueryWorkingSet.Call(
		handle,
		uintptr(unsafe.Pointer(&buffer[0])),
		bufferSize,
	)
	if ret == 0 {
		return 0, fmt.Errorf("QueryWorkingSet failed: %w", callErr)
	}

	actualEntries := *(*uintptr)(unsafe.Pointer(&buffer[0]))
	pageSize := uintptr(4096)
	var privateBytes uintptr
	offset := unsafe.Sizeof(uintptr(0))
	for i := uintptr(0); i < actualEntries; i++ {
		flags := *(*uintptr)(unsafe.Pointer(&buffer[offset+i*unsafe.Sizeof(uintptr(0))]))
		// QueryWorkingSet marks shared pages in bit 8. Counting only pages where
		// that bit is clear mirrors the private working-set number Task Manager
		// uses for its default process memory column.
		isShared := (flags & (1 << 8)) != 0
		if !isShared {
			privateBytes += pageSize
		}
	}
	return privateBytes, nil
}
