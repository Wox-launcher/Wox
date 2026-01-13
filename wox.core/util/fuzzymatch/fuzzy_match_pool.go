package fuzzymatch

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

	uint32Pool = sync.Pool{
		New: func() interface{} {
			// Initial capacity 64 uint32s
			s := make([]uint32, 0, 64)
			return &s
		},
	}

	searchStatePool = sync.Pool{
		New: func() interface{} {
			s := make([]pinyinSearchState, 0, 64)
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

func getUint32Buffer() *[]uint32 {
	ptr := uint32Pool.Get().(*[]uint32)
	*ptr = (*ptr)[:0]
	return ptr
}

func putUint32Buffer(bufPtr *[]uint32) {
	if bufPtr == nil {
		return
	}
	if cap(*bufPtr) > maxPoolBufferSize {
		return
	}
	uint32Pool.Put(bufPtr)
}

func getSearchStateBuffer() *[]pinyinSearchState {
	ptr := searchStatePool.Get().(*[]pinyinSearchState)
	*ptr = (*ptr)[:0]
	return ptr
}

func putSearchStateBuffer(bufPtr *[]pinyinSearchState) {
	if bufPtr == nil {
		return
	}
	if cap(*bufPtr) > maxPoolBufferSize {
		return
	}
	searchStatePool.Put(bufPtr)
}

var int64Pool = sync.Pool{
	New: func() interface{} {
		// Initial capacity 128 int64s (enough for typical pattern len ~8 * 12 + margin)
		s := make([]int64, 0, 128)
		return &s
	},
}

func getInt64Buffer() *[]int64 {
	ptr := int64Pool.Get().(*[]int64)
	*ptr = (*ptr)[:0]
	return ptr
}

func putInt64Buffer(bufPtr *[]int64) {
	if bufPtr == nil {
		return
	}
	if cap(*bufPtr) > maxPoolBufferSize {
		return
	}
	int64Pool.Put(bufPtr)
}
