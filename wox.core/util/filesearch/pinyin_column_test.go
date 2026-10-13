package filesearch

import (
	"context"
	"reflect"
	"testing"

	"wox/util/filesearch/column"
)

// TestColumnPinyinMaskSurvivesCompaction compares persisted acceleration with
// exhaustive scoring after an overlay adds another polyphonic filename.
func TestColumnPinyinMaskSurvivesCompaction(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := column.OpenStore(dir, prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	base := columnQueryFixture(t).Base
	if err := store.Publish(base, "initial", "test", 0); err != nil {
		t.Fatal(err)
	}
	if err := store.Apply([]column.Change{{Record: column.Record{ID: 30, ParentID: 3, Name: "银行测试2026.txt"}}}, "next"); err != nil {
		t.Fatal(err)
	}
	if err := store.Compact(ctx); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = column.OpenStore(dir, prepareColumnNames)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Read(func(view column.View) error {
		// Capture baseline masks so each query exercises both paths against
		// identical persisted rows without modifying the read-only mapping.
		unfiltered := *view.Base
		unfiltered.NameFlags = append([]byte(nil), view.Base.NameFlags...)
		for id := range unfiltered.NameFlags {
			if unfiltered.NameFlags[id]&column.NameNonASCII != 0 && unfiltered.NameFlags[id]&column.NameMaskComplete == 0 {
				t.Fatalf("name %q lost its prepared mask", view.Base.Name(uint32(id)))
			}
			unfiltered.NameFlags[id] &^= column.NameMaskComplete
		}
		for _, raw := range []string{"chongqing", "zhongqing", "cq", "zq", "yinhang", "yinxing", "yhcs2026", "woxcs2026", "cafe", "uber", "absent"} {
			for _, disabled := range []bool{true, false} {
				query := SearchQuery{Raw: raw, DisablePinyin: disabled}
				want, err := searchColumnView(ctx, column.View{Base: &unfiltered}, query, 50)
				if err != nil {
					return err
				}
				got, err := searchColumnView(ctx, view, query, 50)
				if err != nil {
					return err
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("%q pinyin=%v: got %+v, want %+v", raw, !disabled, got, want)
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
