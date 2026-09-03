//go:build windows

package processmemory

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

var (
	processMemoryKernel32       = syscall.NewLazyDLL("kernel32.dll")
	processMemoryNtdll          = syscall.NewLazyDLL("ntdll.dll")
	processMemoryOpenProcess    = processMemoryKernel32.NewProc("OpenProcess")
	processMemoryCloseHandle    = processMemoryKernel32.NewProc("CloseHandle")
	processMemoryNtQueryProcess = processMemoryNtdll.NewProc("NtQueryInformationProcess")
	processMemoryQueryWS        = syscall.NewLazyDLL("psapi.dll").NewProc("QueryWorkingSet")
	processMemoryVirtualQueryEx = processMemoryKernel32.NewProc("VirtualQueryEx")
)

const (
	processMemoryQueryLimitedInformation = 0x1000
	processMemoryQueryInformation        = 0x0400
	processMemoryVMRead                  = 0x0010
	processMemoryProcessVmCounters       = 3
	processMemoryMapped                  = 0x40000
	processMemoryPrivate                 = 0x20000
	processMemoryImage                   = 0x1000000
)

type processMemoryBasicInformation struct {
	baseAddress       uintptr
	allocationBase    uintptr
	allocationProtect uint32
	_                 uint32
	regionSize        uintptr
	state             uint32
	protect           uint32
	typeValue         uint32
	_                 uint32
}

type processMemoryVMCountersEx struct {
	peakVirtualSize            uintptr
	virtualSize                uintptr
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

type processMemoryVMCountersEx2 struct {
	countersEx            processMemoryVMCountersEx
	privateWorkingSetSize uintptr
	sharedCommitUsage     uintptr
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

	privateWorkingSet, err := getTaskManagerPrivateWorkingSetBytes(handle)
	if err != nil {
		return 0, fmt.Errorf("failed to read Task Manager memory for pid %d: %w", pid, err)
	}
	if privateWorkingSet == 0 {
		return 0, fmt.Errorf("empty private working set for pid %d", pid)
	}
	return uint64(privateWorkingSet), nil
}

func openProcessForMemory(pid int) (uintptr, error) {
	handle, _, _ := processMemoryOpenProcess.Call(
		uintptr(processMemoryQueryLimitedInformation),
		0,
		uintptr(pid),
	)
	if handle != 0 {
		return handle, nil
	}
	return 0, fmt.Errorf("OpenProcess failed for pid %d", pid)
}

func getPrivateWorkingSetBreakdown(pid int) (PrivateWorkingSetBreakdown, error) {
	handle, _, callErr := processMemoryOpenProcess.Call(processMemoryQueryInformation|processMemoryVMRead, 0, uintptr(pid))
	if handle == 0 {
		return PrivateWorkingSetBreakdown{}, fmt.Errorf("OpenProcess failed for pid %d: %v", pid, callErr)
	}
	defer processMemoryCloseHandle.Call(handle)

	var entries []uintptr
	for capacity := 65536; capacity <= 1048576; capacity *= 2 {
		entries = make([]uintptr, capacity+1)
		ok, _, _ := processMemoryQueryWS.Call(handle, uintptr(unsafe.Pointer(&entries[0])), uintptr(len(entries))*unsafe.Sizeof(entries[0]))
		if ok != 0 {
			break
		}
		entries = nil
	}
	if entries == nil {
		return PrivateWorkingSetBreakdown{}, fmt.Errorf("QueryWorkingSet failed for pid %d", pid)
	}
	if entries[0] >= uintptr(len(entries)) {
		return PrivateWorkingSetBreakdown{}, fmt.Errorf("invalid working set entry count for pid %d: %d", pid, entries[0])
	}

	pageSize := uint64(os.Getpagesize())
	breakdown := PrivateWorkingSetBreakdown{Available: true}
	var info processMemoryBasicInformation
	for _, flags := range entries[1 : 1+entries[0]] {
		if flags&(1<<8) != 0 {
			continue
		}
		address := (flags >> 12) * uintptr(pageSize)
		if address < info.baseAddress || address-info.baseAddress >= info.regionSize {
			result, _, _ := processMemoryVirtualQueryEx.Call(handle, address, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
			if result == 0 {
				continue
			}
		}
		switch info.typeValue {
		case processMemoryPrivate:
			breakdown.PrivateBytes += pageSize
		case processMemoryMapped:
			breakdown.MappedBytes += pageSize
		case processMemoryImage:
			breakdown.ImageBytes += pageSize
		}
	}
	return breakdown, nil
}

// getTaskManagerPrivateWorkingSetBytes uses the VM counter queried by Windows Task Manager.
func getTaskManagerPrivateWorkingSetBytes(handle uintptr) (uintptr, error) {
	var counters processMemoryVMCountersEx2
	status, _, _ := processMemoryNtQueryProcess.Call(
		handle,
		processMemoryProcessVmCounters,
		uintptr(unsafe.Pointer(&counters)),
		unsafe.Sizeof(counters),
		0,
	)
	if int32(status) != 0 {
		return 0, fmt.Errorf("NtQueryInformationProcess failed with status 0x%x", uint32(status))
	}
	return counters.privateWorkingSetSize, nil
}
