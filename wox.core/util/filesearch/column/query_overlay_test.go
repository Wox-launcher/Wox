package column

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
)

// TestQueryOverlayLazyPreparation keeps background replay, cursor durability and
// query invalidation independent, including ignored and repeated observations.
func TestQueryOverlayLazyPreparation(t *testing.T) {
	s, dir := fixtureStore(t)
	if s.query != nil {
		t.Fatal("publication prepared unused query data")
	}
	change := Change{Record: Record{ID: 10, ParentID: 5, Name: "moved", Flags: Directory, Version: 10}}
	if err := s.ApplyUnsynced([]Change{change}, "change"); err != nil {
		t.Fatal(err)
	}
	if err := s.Read(func(v View) error {
		if s.query != nil || v.Query != nil {
			t.Fatal("ordinary read prepared query data")
		}
		if path, ok := v.Path(12); !ok || filepath.Base(filepath.Dir(path)) != "moved" {
			t.Fatalf("background view lost rename: %q, %v", path, ok)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var prepared *QueryOverlay
	if err := s.ReadQuery(func(v View) error { prepared = v.Query; return nil }); err != nil {
		t.Fatal(err)
	}
	before := s.Revision()
	if err := s.Apply(nil, "checkpoint"); err != nil {
		t.Fatal(err)
	}
	if s.unsynced || s.Revision() <= before {
		t.Fatal("cursor commit did not sync and advance revision")
	}
	for _, observation := range []Change{change, {Record: Record{ID: 10, Version: 9}, Deleted: true}} {
		if err := s.Apply([]Change{observation}, "checkpoint"); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.ReadQuery(func(v View) error {
		if v.Query != prepared || v.Cursor != "checkpoint" {
			t.Fatal("cursor-only or ignored changes rebuilt query data")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	change.Name, change.Version = "again", 11
	if err := s.Apply([]Change{change}, "rename"); err != nil {
		t.Fatal(err)
	}
	change.Name, change.Version = "final", 12
	if err := s.Apply([]Change{change}, "final"); err != nil {
		t.Fatal(err)
	}
	if s.query != nil {
		t.Fatal("background changes rebuilt query data")
	}
	if err := s.ReadQuery(func(v View) error {
		if v.Query == prepared || len(v.Query.LiveDirectories) != 1 || filepath.Base(v.Query.LiveDirectories[0].Path) != "final" {
			t.Fatal("search did not prepare the final coalesced state")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	restored, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer restored.Close()
	if restored.query != nil {
		t.Fatal("journal recovery prepared unused query data")
	}
	if err := restored.ReadQuery(func(v View) error {
		if v.Cursor != "final" || filepath.Base(v.Query.LiveDirectories[0].Path) != "final" {
			t.Fatal("lazy preparation changed journal recovery")
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestQueryOverlayConcurrentLeases checks one preparation per state and that
// readers never combine a query overlay with a different mutation's mapping.
func TestQueryOverlayConcurrentLeases(t *testing.T) {
	s, _ := fixtureStore(t)
	const readers = 8
	prepared := make(chan *QueryOverlay, readers)
	var workers sync.WaitGroup
	for range readers {
		workers.Go(func() {
			if err := s.ReadQuery(func(v View) error { prepared <- v.Query; return nil }); err != nil {
				t.Error(err)
			}
		})
	}
	workers.Wait()
	close(prepared)
	var first *QueryOverlay
	for query := range prepared {
		if first == nil {
			first = query
		} else if query != first {
			t.Fatal("concurrent searches prepared duplicate overlays")
		}
	}
	for range readers {
		workers.Go(func() {
			for range 30 {
				if err := s.ReadQuery(func(v View) error {
					record, _ := v.Lookup(10)
					if len(v.Query.LiveDirectories) > 0 && filepath.Base(v.Query.LiveDirectories[0].Path) != record.Name {
						t.Error("overlay and record came from different states")
					}
					return nil
				}); err != nil {
					t.Error(err)
				}
			}
		})
	}
	for version := uint64(1); version <= 30; version++ {
		change := Change{Record: Record{ID: 10, ParentID: 5, Name: fmt.Sprintf("moved-%d", version), Flags: Directory, Version: version}}
		if err := s.ApplyUnsynced([]Change{change}, change.Name); err != nil {
			t.Error(err)
			break
		}
	}
	workers.Wait()
}
