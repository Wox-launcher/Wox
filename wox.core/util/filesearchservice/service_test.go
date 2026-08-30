package filesearchservice

import "testing"

func TestUpdateAvailableOnlyForNewerEmbeddedService(t *testing.T) {
	if !updateAvailable("1.0.0", "1.1.0") {
		t.Fatal("newer embedded service should be offered")
	}
	if updateAvailable("1.1.0", "1.0.0") || updateAvailable("broken", "1.0.0") {
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

func TestNotInstalledStatusNeverHidesAQueryFailure(t *testing.T) {
	status := GetStatus()
	t.Logf("file index service status: state=%s detail=%q", status.State, status.Detail)
	if status.State == StateNotInstalled && status.Detail != "" {
		t.Fatalf("SCM query failure was reported as not installed: %s", status.Detail)
	}
}
