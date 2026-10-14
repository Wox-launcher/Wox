package filesearch

import "testing"

// TestNativeIndexDurationResetPreservesUnrelatedMetadata keeps manual reset scoped.
func TestNativeIndexDurationResetPreservesUnrelatedMetadata(t *testing.T) {
	db, ctx := openTestFileSearchDB(t)
	if err := db.SetLastFullIndexElapsedMs(ctx, 1000); err != nil {
		t.Fatal(err)
	}
	for _, root := range []string{"volume-a", "volume-b"} {
		if err := db.setNativeFullIndexElapsedMs(ctx, root, 2000); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.db.ExecContext(ctx, `INSERT INTO meta(key, value) VALUES('unrelated', 'keep')`); err != nil {
		t.Fatal(err)
	}
	if err := db.ResetIndex(ctx); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{lastFullIndexElapsedMsKey, nativeFullIndexElapsedMsPrefix + normalizePath("volume-a"), nativeFullIndexElapsedMsPrefix + normalizePath("volume-b")} {
		value, err := db.indexElapsedMs(ctx, key)
		if err != nil || value != 0 {
			t.Fatalf("reset duration %q: %d, %v", key, value, err)
		}
	}
	var value string
	if err := db.db.QueryRowContext(ctx, `SELECT value FROM meta WHERE key = 'unrelated'`).Scan(&value); err != nil || value != "keep" {
		t.Fatalf("unrelated metadata: %q, %v", value, err)
	}
}

func TestResolveLastIndexElapsedMsPrefersLiveRunThenPersisted(t *testing.T) {
	if got := resolveLastIndexElapsedMs(1200, 8000); got != 1200 {
		t.Fatalf("live elapsed = %d, want the in-flight run", got)
	}
	if got := resolveLastIndexElapsedMs(0, 8000); got != 8000 {
		t.Fatalf("idle elapsed = %d, want the persisted full-index duration", got)
	}
	if got := resolveLastIndexElapsedMs(0, 0); got != 0 {
		t.Fatalf("empty elapsed = %d, want 0", got)
	}
}
