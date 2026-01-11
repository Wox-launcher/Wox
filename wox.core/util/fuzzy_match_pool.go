package util

import "sync"

// Large request protection: don't pool huge buffers to avoid memory leaks
const maxPoolBufferSize = 1024 * 64 // 64KB

var (
	runePool = sync.Pool{
		New: func() interface{} {
			// Initial capacity 256 runes
			s := make([]rune, 0, 256)
			return &s
		},
	}

	intPool = sync.Pool{
		New: func() interface{} {
			// Initial capacity 64 ints
			s := make([]int, 0, 64)
			return &s
		},
	}
)

func getRuneBuffer() *[]rune {
	ptr := runePool.Get().(*[]rune)
	*ptr = (*ptr)[:0]
	return ptr
}

func putRuneBuffer(bufPtr *[]rune) {
	if bufPtr == nil {
		return
	}
	// Don't put back huge buffers
	if cap(*bufPtr) > maxPoolBufferSize {
		return
	}
	runePool.Put(bufPtr)
}

func getIntBuffer() *[]int {
	ptr := intPool.Get().(*[]int)
	*ptr = (*ptr)[:0]
	return ptr
}

func putIntBuffer(bufPtr *[]int) {
	if bufPtr == nil {
		return
	}
	if cap(*bufPtr) > maxPoolBufferSize {
		return
	}
	intPool.Put(bufPtr)
}
