//go:build windows

package processmemory

import (
	"fmt"
	"os"
	"runtime"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	processMemoryKernel32        = syscall.NewLazyDLL("kernel32.dll")
	processMemoryNtdll           = syscall.NewLazyDLL("ntdll.dll")
	processMemoryOpenProcess     = processMemoryKernel32.NewProc("OpenProcess")
	processMemoryCloseHandle     = processMemoryKernel32.NewProc("CloseHandle")
	processMemoryNtQueryProcess  = processMemoryNtdll.NewProc("NtQueryInformationProcess")
	processMemoryQueryWS         = syscall.NewLazyDLL("psapi.dll").NewProc("QueryWorkingSet")
	processMemoryVirtualQueryEx  = processMemoryKernel32.NewProc("VirtualQueryEx")
	processMemoryGetProcessHeaps = processMemoryKernel32.NewProc("GetProcessHeaps")
	processMemoryHeapLock        = processMemoryKernel32.NewProc("HeapLock")
	processMemoryHeapUnlock      = processMemoryKernel32.NewProc("HeapUnlock")
	processMemoryHeapWalk        = processMemoryKernel32.NewProc("HeapWalk")
)

const (
	processMemoryQueryLimitedInformation = 0x1000
	processMemoryQueryInformation        = 0x0400
	processMemoryVMRead                  = 0x0010
	processMemoryProcessVmCounters       = 3
	processMemoryMapped                  = 0x40000
	processMemoryPrivate                 = 0x20000
	processMemoryImage                   = 0x1000000
	processMemoryPageGuard               = 0x100

	// Go places every heap arena inside a single hint block, so the top bits of any live heap
	// address identify all arena pages of this runtime. A 1 TiB block matches the runtime's
	// arena hint stride and avoids hardcoding the hint base constant itself.
	processMemoryGoHeapBlockShift = 40

	// A heap walk of a busy low-fragmentation heap can enumerate a very large number of blocks.
	// Only the virtual allocations behind those blocks are needed, so the walk stops once it has
	// clearly seen every segment rather than paying for the whole block list.
	processMemoryHeapWalkLimit = 200000
)

// privatePageOwner labels one private allocation so every resident page of that allocation is
// attributed the same way.
type privatePageOwner uint8

const (
	privateOwnerNativeAnon privatePageOwner = iota
	privateOwnerGoHeap
	privateOwnerThreadStack
	privateOwnerNativeHeap
)

// processMemoryAddressRange is a half-open virtual address interval used to avoid re-querying
// the address space for every block inside an already resolved heap region.
type processMemoryAddressRange struct {
	start uintptr
	end   uintptr
}

// processHeapEntry mirrors PROCESS_HEAP_ENTRY. Only the leading fields are read, so the trailing
// block and region union is kept as opaque padding to preserve the structure size.
type processHeapEntry struct {
	data        uintptr
	dataSize    uint32
	overhead    uint8
	regionIndex uint8
	flags       uint16
	union       [24]byte
}

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

	// The working set buffer is far too large for a stack allocation, so it always lives in a Go
	// heap arena and its address reveals the arena block chosen by this runtime.
	goHeapBlock := uintptr(unsafe.Pointer(&entries[0])) >> processMemoryGoHeapBlockShift
	attributePrivate := pid == os.Getpid()

	var heapBases map[uintptr]struct{}
	if attributePrivate {
		heapBases = nativeHeapAllocationBases()
	}

	pageSize := uint64(os.Getpagesize())
	breakdown := PrivateWorkingSetBreakdown{Available: true, PrivateAttributed: attributePrivate}
	owners := map[uintptr]privatePageOwner{}
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
			if !attributePrivate {
				continue
			}
			owner, classified := owners[info.allocationBase]
			if !classified {
				owner = classifyPrivateAllocation(handle, info.allocationBase, goHeapBlock, heapBases)
				owners[info.allocationBase] = owner
			}
			switch owner {
			case privateOwnerGoHeap:
				breakdown.GoHeapBytes += pageSize
			case privateOwnerThreadStack:
				breakdown.ThreadStackBytes += pageSize
			case privateOwnerNativeHeap:
				breakdown.NativeHeapBytes += pageSize
			default:
				breakdown.NativeAnonBytes += pageSize
			}
		case processMemoryMapped:
			breakdown.MappedBytes += pageSize
		case processMemoryImage:
			breakdown.ImageBytes += pageSize
		}
	}
	return breakdown, nil
}

// nativeHeapAllocationBases collects the virtual allocations backing every Win32 heap of this
// process so pages served by malloc and HeapAlloc can be told apart from memory a library
// reserved from the OS directly. Both heap segments and the oversized blocks the heap manager
// allocates outside its segments are walked, because a single large native buffer such as a
// database page cache lands in the latter and would otherwise look like a direct reservation.
//
// Returning an empty set is a valid outcome: the caller then reports those pages as direct
// reservations, which only makes the attribution coarser rather than wrong.
func nativeHeapAllocationBases() map[uintptr]struct{} {
	// Lazy symbols must be resolved before any heap is locked. Resolving one loads its module,
	// and the loader allocates from the process heap, which would deadlock against our own lock.
	for _, proc := range []*syscall.LazyProc{processMemoryGetProcessHeaps, processMemoryHeapLock, processMemoryHeapUnlock, processMemoryHeapWalk, processMemoryVirtualQueryEx} {
		if proc.Find() != nil {
			return nil
		}
	}

	count, _, _ := processMemoryGetProcessHeaps.Call(0, 0)
	if count == 0 {
		return nil
	}
	handles := make([]uintptr, count)
	found, _, _ := processMemoryGetProcessHeaps.Call(count, uintptr(unsafe.Pointer(&handles[0])))
	if found == 0 || found > count {
		return nil
	}

	// HeapUnlock has to run on the thread that took the lock, so the walk pins itself for the
	// whole duration instead of risking a heap left locked on a different thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// The pseudo handle for the current process avoids a real handle just to query our own maps.
	const currentProcess = ^uintptr(0)
	bases := map[uintptr]struct{}{}
	var walkedRegions []processMemoryAddressRange
	var info processMemoryBasicInformation
	for _, heap := range handles[:found] {
		locked, _, _ := processMemoryHeapLock.Call(heap)
		entry := processHeapEntry{}
		for walked := 0; walked < processMemoryHeapWalkLimit; walked++ {
			if ok, _, _ := processMemoryHeapWalk.Call(heap, uintptr(unsafe.Pointer(&entry))); ok == 0 {
				break
			}
			if entry.data == 0 {
				continue
			}
			// Most blocks sit inside a region already resolved, so the region cache keeps the
			// number of address space queries proportional to segments instead of blocks.
			resolved := false
			for _, region := range walkedRegions {
				if entry.data >= region.start && entry.data < region.end {
					resolved = true
					break
				}
			}
			if resolved {
				continue
			}
			if result, _, _ := processMemoryVirtualQueryEx.Call(currentProcess, entry.data, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info)); result == 0 {
				continue
			}
			bases[info.allocationBase] = struct{}{}
			walkedRegions = append(walkedRegions, processMemoryAddressRange{start: info.baseAddress, end: info.baseAddress + info.regionSize})
		}
		if locked != 0 {
			processMemoryHeapUnlock.Call(heap)
		}
	}
	return bases
}

// classifyPrivateAllocation labels one private allocation by scanning its sub-regions once.
// Thread stacks are the only private allocations Windows keeps a PAGE_GUARD sub-region for, so
// that flag separates committed stack pages from ordinary heap and VirtualAlloc memory without
// enumerating threads and reading their TEBs.
func classifyPrivateAllocation(handle, allocationBase, goHeapBlock uintptr, heapBases map[uintptr]struct{}) privatePageOwner {
	if allocationBase>>processMemoryGoHeapBlockShift == goHeapBlock {
		return privateOwnerGoHeap
	}
	if _, isHeap := heapBases[allocationBase]; isHeap {
		return privateOwnerNativeHeap
	}
	var info processMemoryBasicInformation
	for address := allocationBase; ; address += info.regionSize {
		result, _, _ := processMemoryVirtualQueryEx.Call(handle, address, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
		if result == 0 || info.allocationBase != allocationBase || info.regionSize == 0 {
			return privateOwnerNativeAnon
		}
		if info.protect&processMemoryPageGuard != 0 {
			return privateOwnerThreadStack
		}
	}
}

// listDescendantProcesses snapshots the process table once and walks it breadth-first, because a
// component like WebView2 parents its renderer and GPU processes under its own browser process
// rather than directly under Wox.
func listDescendantProcesses(pid int) ([]DescendantProcess, error) {
	snapshot, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, fmt.Errorf("failed to snapshot processes: %w", err)
	}
	defer windows.CloseHandle(snapshot)

	children := map[uint32][]DescendantProcess{}
	entry := windows.ProcessEntry32{Size: uint32(unsafe.Sizeof(windows.ProcessEntry32{}))}
	for err = windows.Process32First(snapshot, &entry); err == nil; err = windows.Process32Next(snapshot, &entry) {
		children[entry.ParentProcessID] = append(children[entry.ParentProcessID], DescendantProcess{
			ProcessID:       int(entry.ProcessID),
			ParentProcessID: int(entry.ParentProcessID),
			Name:            windows.UTF16ToString(entry.ExeFile[:]),
		})
	}

	var descendants []DescendantProcess
	pending := []uint32{uint32(pid)}
	for len(pending) > 0 {
		parent := pending[0]
		pending = pending[1:]
		for _, child := range children[parent] {
			// A recycled parent id can point back into the walked set, so never revisit an id.
			if child.ProcessID == pid {
				continue
			}
			descendants = append(descendants, child)
			pending = append(pending, uint32(child.ProcessID))
		}
		delete(children, parent)
	}
	return descendants, nil
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
