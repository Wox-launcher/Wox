package filesearchservice

import "testing"

func TestUpdateAvailableOnlyForNewerEmbeddedService(t *testing.T) {
	if !UpdateAvailable("1.0.0", "1.1.0") {
		t.Fatal("newer embedded service should be offered")
	}
	if UpdateAvailable("1.1.0", "1.0.0") || UpdateAvailable("broken", "1.0.0") {
		t.Fatal("older or invalid embedded service must not replace the installed service")
	}
}

func TestRememberIndexedVolumeRootsKeepsLastNonEmptyList(t *testing.T) {
	previous := indexedVolumeRootsCache.Load()
	t.Cleanup(func() {
		if previous != nil {
			indexedVolumeRootsCache.Store(previous)
		}
	})

	rememberIndexedVolumeRoots([]string{`C:\`})
	rememberIndexedVolumeRoots(nil)
	got := cachedIndexedVolumeRoots()
	if len(got) != 1 || got[0] != `C:\` {
		t.Fatalf("cached volumes = %v, want the last published list", got)
	}
}
