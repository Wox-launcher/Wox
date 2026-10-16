//go:build windows

package processmemory

import (
	"fmt"
	"unsafe"
)

// ReleaseIdleNativeHeap asks the Windows heap manager to decommit unused LFH
// caches. Live allocations stay valid; this does not evict the working set.
func ReleaseIdleNativeHeap() error {
	info := struct{ Version, Flags uint32 }{Version: 1}
	proc := processMemoryKernel32.NewProc("HeapSetInformation")
	if err := proc.Find(); err != nil {
		return err
	}
	ok, _, err := proc.Call(0, 3, uintptr(unsafe.Pointer(&info)), unsafe.Sizeof(info))
	if ok == 0 {
		return fmt.Errorf("optimize native heap caches: %w", err)
	}
	return nil
}
