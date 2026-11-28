//go:build windows

package filesearch

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"wox/plugin/system/file_search/indexpolicy"
	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

func TestNativeIgnoreVolumeLifecycle(t *testing.T) {
	ctx := context.Background()
	p := indexpolicy.New()
	p.SetIgnorePatterns([]string{"e:/**", "F:\\"})
	stream := func(ctx context.Context, kind protocol.Kind, b protocol.Boundary, emit filesearchservice.FactSink) error {
		if b.Volume != "C:\\" {
			t.Errorf("contacted excluded volume %q", b.Volume)
			return ctx.Err()
		}
		if kind == protocol.Enumerate {
			return emitNativeIndexFixture(emit, b.Volume, "visible.txt")
		}
		<-ctx.Done()
		return ctx.Err()
	}
	roots := []string{"C:\\", "E:\\", "F:\\"}
	dir := t.TempDir()
	index, err := newNTFSNameIndex(ctx, dir, roots, stream, nativeIndexCallbacks{ignore: p.FilenameMatcher()})
	if err != nil {
		t.Fatal(err)
	}
	defer index.Close()
	awaitNativeMatches(t, index, "visible", 1)
	if len(index.directory.shards) != 1 {
		t.Fatalf("attached roots: %v", nativeRootPaths(index))
	}
	p.SetIgnorePatterns([]string{"E:\\", "F:/**", "visible.txt"})
	if !index.updateIgnoreRules(p.FilenameMatcher()) {
		t.Fatal("folder rule restarted volume workers")
	}
	awaitNativeMatches(t, index, "visible", 0)
	p.SetIgnorePatterns([]string{"C:\\", "E:\\", "F:\\"})
	if index.updateIgnoreRules(p.FilenameMatcher()) {
		t.Fatal("volume membership change missed")
	}
	empty, err := newNTFSNameIndex(ctx, t.TempDir(), roots, stream, nativeIndexCallbacks{ignore: p.FilenameMatcher()})
	if err != nil {
		t.Fatal(err)
	}
	defer empty.Close()
	if len(empty.directory.shards) != 0 {
		t.Fatal("all-excluded configuration fell back to indexing")
	}
}

func TestNativeIgnoreSearchAndReplay(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := column.OpenStore(dir, prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	builder := column.NewBuilder("C:\\", 1)
	for _, r := range []column.Record{
		{ID: 20, ParentID: 3, Name: "needle.txt"}, // Child facts may precede their parents.
		{ID: 2, ParentID: 1, Name: "NODE_MODULES", Flags: column.Directory},
		{ID: 3, ParentID: 2, Name: "sub", Flags: column.Directory},
		{ID: 4, ParentID: 1, Name: "src", Flags: column.Directory},
		{ID: 21, ParentID: 4, Name: "needle.txt"}, {ID: 22, ParentID: 4, Name: "file.tmp"},
		{ID: 23, ParentID: 4, Name: "artifact-123", Flags: column.Directory},
		{ID: 24, ParentID: 23, Name: "nested", Flags: column.Directory},
		{ID: 25, ParentID: 24, Name: "needle.txt"},
	} {
		if err := builder.Add(r); err != nil {
			t.Fatal(err)
		}
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Publish(base, "", "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	p := indexpolicy.New()
	p.SetIgnorePatterns([]string{"node_modules", "*.tmp", "src/artifact-*"})
	setRules := func() {
		if rules := p.FilenameMatcher(); rules != nil {
			store.SetQueryProjection(&nativeQueryVisibility{rules: rules, cacheDir: dir})
		} else {
			store.SetQueryProjection(nil)
		}
	}
	setRules()
	search := func(query string, limit int, want ...string) {
		t.Helper()
		err := store.ReadQuery(func(v column.View) error {
			results, err := searchColumnView(ctx, v, SearchQuery{Raw: query}, limit)
			var paths []string
			for _, r := range results {
				paths = append(paths, r.Path)
			}
			slices.Sort(paths)
			slices.Sort(want)
			if !slices.Equal(paths, want) {
				t.Errorf("%q: got %q, want %q", query, paths, want)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	const visible = "C:\\src\\needle.txt"
	search("needle", 1, visible)
	search("*.tmp", 10)
	search("src/artifact", 10)
	search("node_modules", 10)
	apply := func(r column.Record) {
		if err := store.Apply([]column.Change{{Record: r}}, ""); err != nil {
			t.Fatal(err)
		}
	}
	apply(column.Record{ID: 30, ParentID: 3, Name: "needle2.txt"})
	search("needle", 10, visible)
	apply(column.Record{ID: 2, ParentID: 1, Name: "normal", Flags: column.Directory})
	search("needle", 10, visible, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt")
	apply(column.Record{ID: 4, ParentID: 2, Name: "node_modules", Flags: column.Directory})
	search("needle", 10, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt")
	apply(column.Record{ID: 31, ParentID: 4, Name: "fresh", Flags: column.Directory})
	search("node_modules/fresh", 10)
	if err := store.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	search("needle", 10, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = column.OpenStore(dir, prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	setRules()
	search("needle", 10, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt")
	data, err := os.ReadFile(filepath.Join(dir, "visibility.cache"))
	if err != nil {
		t.Fatal(err)
	}
	data[len(data)-1] ^= 255
	if err := os.WriteFile(filepath.Join(dir, "visibility.cache"), data, 0600); err != nil {
		t.Fatal(err)
	}
	setRules()
	search("needle", 10, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt")
	p.SetIgnorePatterns(nil)
	setRules()
	search("needle", 10, "C:\\normal\\sub\\needle.txt", "C:\\normal\\sub\\needle2.txt", "C:\\normal\\node_modules\\needle.txt", "C:\\normal\\node_modules\\artifact-123\\nested\\needle.txt")
}

// A restored generation and ordinary keystrokes must never re-evaluate its rules.
type cachedVisibilityRules struct{ FilenameIgnoreRules }

func (cachedVisibilityRules) ClassifyName(string) (bool, bool) {
	panic("reclassified a cached NTFS basename")
}

func (cachedVisibilityRules) IgnorePath(string, string) bool {
	panic("reclassified a cached NTFS path")
}

// Compare cached scores to ordinary scoring across rule edits and live USN facts.
func TestNativeIgnoreFrequentNameRanking(t *testing.T) {
	ctx := context.Background()
	store, err := column.OpenStore(t.TempDir(), prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	builder := column.NewBuilder("C:\\", 1)
	add := func(id, parent uint64, name string, flags byte) {
		if err := builder.Add(column.Record{ID: id, ParentID: parent, Name: name, Flags: flags}); err != nil {
			t.Fatal(err)
		}
	}
	add(2, 1, "node_modules", column.Directory)
	for n := uint64(0); n < 40; n++ {
		id := 20 + n*4
		add(id, 2, fmt.Sprint(n), column.Directory)
		add(id+1, id, "test", 0)
		add(id+2, 1, fmt.Sprintf("src%d", n), column.Directory)
		add(id+3, id+2, "test.go", 0)
	}
	for n := uint64(0); n < 16; n++ {
		add(300+n, 1, fmt.Sprintf("test-sample-%02d", n), 0)
	}
	for n, name := range []string{"test", "TEST.md", "t-e-s-t", " Test ", "TestFolder", "test\u6587\u4ef6.txt"} {
		add(uint64(500+n), 1, name, 0)
	}
	base, err := builder.Finish()
	if err != nil {
		t.Fatal(err)
	}
	p := indexpolicy.New()
	p.SetIgnorePatterns([]string{"node_modules"})
	filter := &nativeQueryVisibility{rules: p.FilenameMatcher(), cacheDir: t.TempDir()}
	store.SetQueryProjection(filter)
	if err := store.Publish(base, "", "ntfs-v3", 0); err != nil {
		t.Fatal(err)
	}
	compare := func() {
		t.Helper()
		err := store.ReadQuery(func(v column.View) error {
			if len(v.FrequentMatches["test"]) == 0 {
				t.Fatal("missing cached scores")
			}
			if len(v.Delta) == 0 && v.PrefixCounts["test"].WithoutPinyin < 10 {
				t.Fatal("expected at least ten distinct visible prefix-quality matches")
			}
			ordinary := v
			ordinary.FrequentMatches, ordinary.FrequentByInitial, ordinary.PrefixCounts = nil, nil, nil
			for _, raw := range []string{"test", "TEST", "TeSt"} {
				for _, disabled := range []bool{false, true} {
					q := SearchQuery{Raw: raw, DisablePinyin: disabled}
					for _, limit := range []int{1, 10, 100, 0} {
						want, err := searchColumnView(ctx, ordinary, q, limit)
						if err != nil {
							return err
						}
						got, err := searchColumnView(ctx, v, q, limit)
						if err != nil {
							return err
						}
						if !slices.Equal(got, want) {
							t.Fatalf("%q pinyin disabled=%t limit=%d: got %v, want %v", raw, disabled, limit, got, want)
						}
					}
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	compare()
	store.SetQueryProjection(&nativeQueryVisibility{rules: cachedVisibilityRules{p.FilenameMatcher()}, cacheDir: filter.cacheDir})
	compare()
	store.SetQueryProjection(&nativeQueryVisibility{rules: p.FilenameMatcher(), cacheDir: filter.cacheDir})
	if err := store.Apply([]column.Change{{Record: column.Record{ID: 500, ParentID: 2, Name: "test"}}, {Record: column.Record{ID: 1001, ParentID: 1, Name: "test_new"}}}, ""); err != nil {
		t.Fatal(err)
	}
	compare()
}

// Path-specific rules must not prune same-named directories on another drive
// or at a different depth when path checks are gated by the basename.
func TestNativeIgnorePathScopes(t *testing.T) {
	p := indexpolicy.New()
	p.SetIgnorePatterns([]string{"D:\\Projects\\cache", "**/build/**/vendor/**", "src/generated-*/out", "*.tmp"})
	rules := p.FilenameMatcher()
	filter := nativeQueryVisibility{rules: rules}
	for _, tc := range []struct {
		path    string
		ignored bool
	}{
		{"D:\\Projects\\CACHE\\child.go", true}, {"C:\\Projects\\cache\\child.go", false},
		{"D:\\Other\\Projects\\cache\\child.go", false}, {"D:\\Projects\\cache2", false},
		{"C:\\src\\build\\amd64\\vendor\\x.go", true}, {"C:\\src\\vendor\\x.go", false},
		{"C:\\src\\generated-x\\out\\nested", true}, {"C:\\src\\gen\\out\\nested", false},
		{"C:\\projects\\File.TMP", true}, {"C:\\projects\\File.TMPx", false},
	} {
		root := filepath.VolumeName(tc.path) + "\\"
		if got := filter.ignoredDirectory(root, tc.path); got != tc.ignored {
			t.Errorf("%q: ignored = %t, want %t", tc.path, got, tc.ignored)
		}
	}
}
