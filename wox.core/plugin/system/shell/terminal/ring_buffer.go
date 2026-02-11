package terminal

import (
	"bytes"
	"strings"
	"sync"
)

type RingBuffer struct {
	mu        sync.RWMutex
	data      []byte
	lineCount int

	startCursor int64
	endCursor   int64
	maxBytes    int
	maxLines    int
}

func NewRingBuffer(maxBytes int, maxLines int) *RingBuffer {
	return &RingBuffer{
		maxBytes: maxBytes,
		maxLines: maxLines,
	}
}

func (r *RingBuffer) Append(content string) (start int64, end int64, truncated bool) {
	if content == "" {
		r.mu.RLock()
		defer r.mu.RUnlock()
		return r.endCursor, r.endCursor, false
	}

	b := []byte(content)
	newlineCount := bytes.Count(b, []byte{'\n'})

	r.mu.Lock()
	defer r.mu.Unlock()

	start = r.endCursor
	r.data = append(r.data, b...)
	r.endCursor += int64(len(b))
	r.lineCount += newlineCount
	end = r.endCursor

	for (r.maxBytes > 0 && len(r.data) > r.maxBytes) || (r.maxLines > 0 && r.lineCount > r.maxLines) {
		remove := r.nextTrimSize()
		if remove <= 0 || remove > len(r.data) {
			break
		}
		removed := r.data[:remove]
		r.data = r.data[remove:]
		r.startCursor += int64(remove)
		r.lineCount -= bytes.Count(removed, []byte{'\n'})
		truncated = true
	}

	return start, end, truncated
}

func (r *RingBuffer) nextTrimSize() int {
	if len(r.data) == 0 {
		return 0
	}

	idx := bytes.IndexByte(r.data, '\n')
	if idx >= 0 {
		return idx + 1
	}

	if r.maxBytes > 0 && len(r.data) > r.maxBytes {
		return len(r.data) - r.maxBytes
	}

	return len(r.data)
}

func (r *RingBuffer) SliceFrom(cursor int64, maxBytes int) (chunk string, nextCursor int64, truncated bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if maxBytes <= 0 {
		maxBytes = len(r.data)
	}

	if cursor < r.startCursor {
		cursor = r.startCursor
		truncated = true
	}
	if cursor > r.endCursor {
		cursor = r.endCursor
	}

	offset := int(cursor - r.startCursor)
	if offset < 0 {
		offset = 0
	}
	if offset > len(r.data) {
		offset = len(r.data)
	}

	endOffset := offset + maxBytes
	if endOffset > len(r.data) {
		endOffset = len(r.data)
	}

	chunk = string(r.data[offset:endOffset])
	nextCursor = r.startCursor + int64(endOffset)
	return chunk, nextCursor, truncated
}

func (r *RingBuffer) SnapshotTail(maxBytes int) string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if maxBytes <= 0 || maxBytes >= len(r.data) {
		return string(r.data)
	}
	return string(r.data[len(r.data)-maxBytes:])
}

func (r *RingBuffer) StartCursor() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.startCursor
}

func (r *RingBuffer) EndCursor() int64 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.endCursor
}

func (r *RingBuffer) Search(pattern string, cursor int64, backward bool, caseSensitive bool) (matchStart int64, matchEnd int64, nextCursor int64, found bool) {
	if pattern == "" {
		return 0, 0, cursor, false
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if len(r.data) == 0 {
		return 0, 0, cursor, false
	}

	bufferStart := r.startCursor
	bufferEnd := r.endCursor

	if cursor < bufferStart {
		cursor = bufferStart
	}
	if cursor > bufferEnd {
		cursor = bufferEnd
	}

	source := string(r.data)
	query := pattern
	if !caseSensitive {
		source = strings.ToLower(source)
		query = strings.ToLower(pattern)
	}

	offset := int(cursor - bufferStart)
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}

	if backward {
		idx := strings.LastIndex(source[:offset], query)
		if idx < 0 {
			return 0, 0, cursor, false
		}
		matchStart = bufferStart + int64(idx)
		matchEnd = matchStart + int64(len(pattern))
		nextCursor = matchStart
		return matchStart, matchEnd, nextCursor, true
	}

	idx := strings.Index(source[offset:], query)
	if idx < 0 {
		return 0, 0, cursor, false
	}
	matchStart = bufferStart + int64(offset+idx)
	matchEnd = matchStart + int64(len(pattern))
	nextCursor = matchEnd
	return matchStart, matchEnd, nextCursor, true
}
