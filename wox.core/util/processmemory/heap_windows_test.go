//go:build windows

package processmemory

import (
	"testing"
	"unsafe"
)

// TestReleaseIdleNativeHeapPreservesLiveBlocks exercises the allocator with
// interleaved live/free blocks so cache trimming cannot discard live contents.
func TestReleaseIdleNativeHeapPreservesLiveBlocks(t *testing.T) {
	heap, _, _ := processMemoryKernel32.NewProc("GetProcessHeap").Call()
	alloc := processMemoryKernel32.NewProc("HeapAlloc")
	free := processMemoryKernel32.NewProc("HeapFree")
	blocks := make([]uintptr, 128)
	defer func() {
		for _, block := range blocks {
			if block != 0 {
				free.Call(heap, 0, block)
			}
		}
	}()
	for index := range blocks {
		block, _, err := alloc.Call(heap, 0, 8192)
		if block == 0 {
			t.Fatal(err)
		}
		blocks[index] = block
		for offset := range 8192 {
			*(*byte)(unsafe.Pointer(block + uintptr(offset))) = byte(index)
		}
	}
	for index := 0; index < len(blocks); index += 2 {
		free.Call(heap, 0, blocks[index])
		blocks[index] = 0
	}
	if err := ReleaseIdleNativeHeap(); err != nil {
		t.Fatal(err)
	}
	for index, block := range blocks {
		if block == 0 {
			continue
		}
		for offset := range 8192 {
			if *(*byte)(unsafe.Pointer(block + uintptr(offset))) != byte(index) {
				t.Fatalf("live block %d changed at offset %d", index, offset)
			}
		}
	}
}
