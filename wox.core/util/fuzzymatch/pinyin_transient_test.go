package fuzzymatch

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// TestPinyinTransientMatchesCachedSegments compares borrowed segmentation with
// the independent owning builder, including growth past scratch's initial size,
// unknown Han, polyphonic characters, literal blocks and invalid UTF-8 input.
func TestPinyinTransientMatchesCachedSegments(t *testing.T) {
	ReleaseIdleCaches()
	defer ReleaseIdleCaches()
	for _, text := range []string{"QQ音乐", "Wox测试2026.txt", "重庆银行文件夹", "café测试Ω文档", "𠀀测试", "a\xff\xfe测试", strings.Repeat("重A庆B行C", 30)} {
		for _, raw := range []string{"qqyy", "cs2026", "chongqing", "yinhang", "cqyh", "cscs", "a", "xiangmu", strings.Repeat("c", 40)} {
			pattern := PreparePattern(raw)
			got := matchPinyinText(text, pattern.normalizedRunes)
			want := matchPinyinStrict(getPinYin(text), pattern.normalizedRunes)
			if got != want {
				t.Fatalf("text=%q pattern=%q: transient=%+v cached=%+v", text, raw, got, want)
			}
		}
	}
}

// TestPinyinTransientDoesNotPopulateNameCache guards cold-search allocation and
// mmap ownership: scoring distinct borrowed names must not retain their graphs.
func TestPinyinTransientDoesNotPopulateNameCache(t *testing.T) {
	ReleaseIdleCaches()
	defer ReleaseIdleCaches()
	pattern := PreparePattern("qqyy")
	for i := range 100 {
		FuzzyMatchPattern(fmt.Sprintf("QQ音乐%d", i), pattern, false, true)
	}
	if size := pinyinCacheSize.Load(); size != 0 {
		t.Fatalf("transient searches retained %d name graphs", size)
	}
}

// TestPinyinTransientConcurrentRelease keeps borrowed readings valid while idle
// cleanup drops the published dictionary and other matchers rebuild it.
func TestPinyinTransientConcurrentRelease(t *testing.T) {
	pattern := PreparePattern("cqyh")
	want := FuzzyMatchPattern("重庆银行", pattern, false, true)
	var workers sync.WaitGroup
	for range 4 {
		workers.Go(func() {
			for range 100 {
				if got := FuzzyMatchPattern("重庆银行", pattern, false, true); got != want {
					t.Errorf("score changed during release: %+v != %+v", got, want)
				}
			}
		})
	}
	for range 10 {
		ReleasePinyinDictionary()
	}
	workers.Wait()
}

// BenchmarkPinyinTransientMatch measures reusable scoring without a name cache.
func BenchmarkPinyinTransientMatch(b *testing.B) {
	pattern := PreparePattern("qqyy")
	FuzzyMatchPattern("QQ音乐播放器", pattern, false, true)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		FuzzyMatchPattern("QQ音乐播放器", pattern, false, true)
	}
}
