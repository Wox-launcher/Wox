package launcher

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"wox/common"
	woxui "wox/ui/runtime"
)

type retryImageResolver struct {
	started chan string
	release chan struct{}
}

func (r *retryImageResolver) LoadLazyResultImage(_ context.Context, _ string, token string) (common.WoxImage, error) {
	r.started <- token
	if token == "old" {
		<-r.release
		return common.WoxImage{}, errors.New("lazy image token not found")
	}
	return common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#ff0000"/></svg>`), nil
}

func (*retryImageResolver) ResolveImage(context.Context, string, common.WoxImage, int) (common.WoxImage, error) {
	return common.WoxImage{}, errors.New("unexpected image resolution")
}

// TestLazyImageRetriesAfterQueryChanges covers stale tokens and shared row icons.
func TestLazyImageRetriesAfterQueryChanges(t *testing.T) {
	resolver := &retryImageResolver{started: make(chan string, 4), release: make(chan struct{})}
	changed := make(chan struct{}, 10)
	store := imageStore{
		isDestroyed:  func() bool { return false },
		services:     resolver,
		lifecycleCtx: context.Background(),
		runOnUI:      func(_ string, fn func()) error { fn(); return nil },
		invalidateAllWindows: func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		},
		images:         map[string]*woxui.Image{},
		imageRequested: map[string]string{},
		imageLastUsed:  map[string]uint64{},
		imageErrors:    map[string]string{},
	}
	source := func(token, query string) woxImage {
		data, err := json.Marshal(lazyImagePayload{
			Token: token, CacheKey: "shared", QueryID: query,
			Placeholder: woxImage{ImageType: "svg", ImageData: `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="10" height="10" fill="#000000"/></svg>`},
		})
		if err != nil {
			t.Fatal(err)
		}
		return woxImage{ImageType: "lazyloadimage", ImageData: string(data)}
	}
	old, sibling, next := source("old", "first"), source("sibling", "first"), source("new", "second")
	key, _, _ := imageAppearanceCacheKey(old, nil, 20, 20, false, nil)
	waitForToken := func(want string) {
		t.Helper()
		select {
		case token := <-resolver.started:
			if token != want {
				t.Fatalf("token = %s, want %s", token, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for token %s", want)
		}
	}
	store.imageForTintAppearance(old, nil, 20, 20, false, nil, false)
	waitForToken("old")
	// A new query arrives while the old token is still resolving.
	store.imageForTintAppearance(next, nil, 20, 20, false, nil, false)
	close(resolver.release)
	waitFor := func(condition func() bool) {
		t.Helper()
		deadline := time.After(5 * time.Second)
		for !condition() {
			select {
			case <-changed:
			case <-deadline:
				t.Fatal("image update timed out")
			}
		}
	}
	waitFor(func() bool { store.imageMu.RLock(); defer store.imageMu.RUnlock(); return store.imageErrors[key] != "" })
	store.imageForTintAppearance(sibling, nil, 20, 20, false, nil, false)
	store.imageMu.RLock()
	requested := store.imageRequested[key]
	store.imageMu.RUnlock()
	if requested != old.ImageData {
		t.Fatal("another row in the same query retried a failed image")
	}
	if store.imageForTintAppearance(next, nil, 20, 20, false, nil, false) == nil {
		t.Fatal("retry did not retain the placeholder")
	}
	waitForToken("new")
	waitFor(func() bool {
		store.imageMu.RLock()
		defer store.imageMu.RUnlock()
		image := store.images[key]
		return image != nil && image.RGBAAt(5, 5).R == 255 && store.imageErrors[key] == ""
	})
	store.imageForTintAppearance(source("third", "third"), nil, 20, 20, false, nil, false)
	store.imageMu.RLock()
	requested = store.imageRequested[key]
	store.imageMu.RUnlock()
	if requested != next.ImageData {
		t.Fatal("successful image was reloaded for a new query")
	}
}

func TestRetryErrorsClearsCachedPlaceholder(t *testing.T) {
	store := imageStore{
		images:         map[string]*woxui.Image{"icon": {Width: 1, Height: 1}},
		imageRequested: map[string]string{"icon": "source"},
		imageErrors:    map[string]string{"icon": "offline"},
	}
	store.retryErrors()
	if store.images["icon"] != nil || store.imageRequested["icon"] != "" || store.imageErrors["icon"] != "" {
		t.Fatal("failed placeholder remained cached after connectivity recovery")
	}
}
