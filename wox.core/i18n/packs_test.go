package i18n

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"
	"wox/network"
	"wox/util"
)

func TestPackDownloadSharesProgressAndRefreshesCache(t *testing.T) {
	manager := &Manager{enUsLang: map[string]string{"ui_lang": "Language", "fallback": "English"}}
	manager.ConfigurePacks(t.TempDir())
	if err := manager.UpdateLang(context.Background(), LangCodeEnUs); err != nil {
		t.Fatal(err)
	}
	var requests atomic.Int32
	release := make(chan struct{})
	released := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Cache-Control") != "no-cache" {
			t.Error("request must revalidate cached responses")
		}
		n := requests.Add(1)
		_, _ = fmt.Fprint(w, `{"ui_lang":"Idioma","fallback":"  ",`)
		w.(http.Flusher).Flush()
		if n == 1 {
			<-release
		}
		_, _ = fmt.Fprintf(w, `"revision":"%d"}`, n)
	}))
	defer server.Close()
	defer func() {
		if !released {
			close(release)
		}
	}()
	manager.packs.download = func(ctx context.Context, url string, limit int64, progress func(int64, int64)) ([]byte, error) {
		if url != languagePackURL+"pt_BR.json" {
			return nil, fmt.Errorf("unexpected official URL %q", url)
		}
		return util.HttpDownloadBytes(ctx, server.URL, limit, progress)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := func() (<-chan PackProgress, <-chan error) {
		updates := make(chan PackProgress, 8)
		result := make(chan error, 1)
		go func() {
			_, err := manager.DownloadPack(ctx, LangCodePtBr, func(progress PackProgress) { updates <- progress })
			result <- err
		}()
		return updates, result
	}
	awaitBytes := func(updates <-chan PackProgress) {
		t.Helper()
		for {
			select {
			case progress := <-updates:
				if progress.Total != -1 {
					t.Fatalf("chunked transfer progress = %+v", progress)
				}
				if progress.Received > 0 {
					return
				}
			case <-ctx.Done():
				t.Fatal("no streaming progress before the response completed")
			}
		}
	}
	first, firstResult := start()
	awaitBytes(first)
	second, secondResult := start()
	awaitBytes(second)
	close(release)
	released = true
	for _, result := range []<-chan error{firstResult, secondResult} {
		if err := <-result; err != nil {
			t.Fatal(err)
		}
	}
	if requests.Load() != 1 {
		t.Fatalf("got %d requests for one pack", requests.Load())
	}
	if !manager.PackInstalled(LangCodePtBr) {
		t.Fatal("successful transfer did not install the pack")
	}
	if err := manager.RefreshActivePack(ctx, LangCodePtBr); err != nil || manager.GetCurrentLangCode() != LangCodeEnUs {
		t.Fatalf("refresh changed the active language: %v", err)
	}
	if err := manager.UpdateLang(ctx, LangCodePtBr); err != nil {
		t.Fatal(err)
	}
	pack, err := manager.DownloadPack(ctx, LangCodePtBr, nil)
	if err != nil || pack.translations["revision"] != "2" {
		t.Fatalf("manual refresh = %+v, %v", pack, err)
	}
	if err := manager.RefreshActivePack(ctx, LangCodePtBr); err != nil || manager.TranslateWox(ctx, "revision") != "2" {
		t.Fatalf("active language did not receive fresh translations: %v", err)
	}
	// A fresh process can load the pack without contacting the server.
	restarted := &Manager{enUsLang: manager.enUsLang}
	restarted.ConfigurePacks(manager.packs.directory)
	pack, err = restarted.ReadPack(ctx, LangCodePtBr)
	if err != nil || pack.translations["revision"] != "2" {
		t.Fatalf("cached pack after restart = %+v, %v", pack, err)
	}
	restarted.ActivatePack(pack)
	if got := restarted.TranslateWox(ctx, "fallback"); got != "English" {
		t.Fatalf("blank translation did not fall back to English: %q", got)
	}
}

func TestSpanishLanguagePackLoadsFromDownload(t *testing.T) {
	ctx := context.Background()
	manager := &Manager{}
	manager.ConfigurePacks(t.TempDir())
	manager.packs.download = func(_ context.Context, url string, _ int64, _ func(int64, int64)) ([]byte, error) {
		if url != languagePackURL+"es_ES.json" {
			return nil, fmt.Errorf("unexpected Spanish language pack URL %q", url)
		}
		return os.ReadFile("../../lang/es_ES.json")
	}
	pack, err := manager.DownloadPack(ctx, LangCodeEsEs, nil)
	if err != nil {
		t.Fatal(err)
	}
	manager.ActivatePack(pack)
	if got := manager.TranslateWox(ctx, "i18n:ui_lang"); got != "Idioma" {
		t.Fatalf("Spanish language label = %q", got)
	}
}

func TestRefreshCachedPacksUpdatesInstalledLanguagesOnEveryStartup(t *testing.T) {
	ctx := context.Background()
	cache := t.TempDir()
	var requests []string
	russianURL := languagePackURL + "ru_RU.json"
	failRussian := true
	label := "First refresh"
	start := func() *Manager {
		manager := &Manager{}
		manager.ConfigurePacks(cache)
		manager.packs.download = func(_ context.Context, url string, _ int64, _ func(int64, int64)) ([]byte, error) {
			requests = append(requests, url)
			if url == russianURL && failRussian {
				return nil, errors.New("temporary download failure")
			}
			return fmt.Appendf(nil, `{"ui_lang":%q}`, label), nil
		}
		return manager
	}
	checkPack := func(manager *Manager, code LangCode, label string) {
		t.Helper()
		pack, err := manager.ReadPack(ctx, code)
		if err != nil || pack.translations["ui_lang"] != label {
			t.Fatalf("pack %s: label=%q err=%v", code, pack.translations["ui_lang"], err)
		}
	}
	start().RefreshCachedPacks(ctx, nil)
	if len(requests) != 0 {
		t.Fatalf("fresh startup requested %v", requests)
	}
	manager := start()
	for _, code := range []LangCode{LangCodeRuRu, LangCodePtBr} {
		if err := manager.packs.write(code, []byte(`{"ui_lang":"Old"}`)); err != nil {
			t.Fatal(err)
		}
	}
	// Existing caches may include the Wox version from the previous refresh policy.
	if err := os.WriteFile(manager.packs.path(LangCodeJaJp), []byte(`{"version":"2.5.0","translations":{"ui_lang":"Old"}}`), 0600); err != nil {
		t.Fatal(err)
	}

	var refreshed []LangCode
	wantRequests := []string{russianURL, languagePackURL + "pt_BR.json", languagePackURL + "ja_JP.json"}
	manager.RefreshCachedPacks(ctx, func(code LangCode) { refreshed = append(refreshed, code) })
	manager.RefreshCachedPacks(ctx, nil)
	if !slices.Equal(requests, wantRequests) ||
		!slices.Equal(refreshed, []LangCode{LangCodePtBr, LangCodeJaJp}) {
		t.Fatalf("first startup requests=%v refreshed=%v", requests, refreshed)
	}
	checkPack(manager, LangCodeRuRu, "Old")
	checkPack(manager, LangCodePtBr, label)
	checkPack(manager, LangCodeJaJp, label)

	failRussian = false
	requests = nil
	label = "Second refresh"
	manager = start()
	manager.RefreshCachedPacks(ctx, nil)
	if !slices.Equal(requests, wantRequests) {
		t.Fatalf("next startup requested %v, want %v", requests, wantRequests)
	}
	checkPack(manager, LangCodeRuRu, label)
	checkPack(manager, LangCodePtBr, label)
	checkPack(manager, LangCodeJaJp, label)
}

func TestFailedPackUpdatesKeepExistingCache(t *testing.T) {
	const valid = `{"ui_lang":"日本語"}`
	manager := &Manager{}
	manager.ConfigurePacks(t.TempDir())
	if err := manager.packs.write(LangCodeJaJp, []byte(valid)); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, scenario := range []string{"offline", "timeout", "404", "truncated", "invalid", "too large"} {
		t.Run(scenario, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch scenario {
				case "timeout":
					<-r.Context().Done()
				case "404":
					http.NotFound(w, r)
				case "truncated":
					w.Header().Set("Content-Length", "500")
					_, _ = fmt.Fprint(w, valid)
				case "invalid":
					_, _ = fmt.Fprint(w, `{"ui_lang":15}`)
				case "too large":
					w.Header().Set("Content-Length", fmt.Sprint(languagePackLimit+1))
				}
			}))
			defer server.Close()
			manager.packs.timeout = time.Second
			if scenario == "timeout" {
				manager.packs.timeout = 100 * time.Millisecond
			}
			manager.packs.download = func(ctx context.Context, url string, limit int64, progress func(int64, int64)) ([]byte, error) {
				if scenario != "offline" {
					url = server.URL
				}
				return util.HttpDownloadBytes(ctx, url, limit, progress)
			}
			if scenario == "offline" {
				offline := network.IsOffline()
				network.Default.SetOffline(true)
				defer network.Default.SetOffline(offline)
			}
			_, err := manager.DownloadPack(ctx, LangCodeJaJp, nil)
			if err == nil || (scenario == "offline" && !errors.Is(err, network.ErrOffline)) {
				t.Fatalf("unexpected download error: %v", err)
			}
			pack, err := manager.ReadPack(ctx, LangCodeJaJp)
			if err != nil || pack.translations["ui_lang"] != "日本語" {
				t.Fatalf("failed refresh damaged the cache: %+v, %v", pack, err)
			}
		})
	}
}

func TestPackWriteFailureCanBeRetried(t *testing.T) {
	manager := &Manager{}
	manager.ConfigurePacks(t.TempDir())
	manager.packs.download = func(context.Context, string, int64, func(int64, int64)) ([]byte, error) {
		return []byte(`{"ui_lang":"한국어"}`), nil
	}
	path := manager.packs.path(LangCodeKoKr)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := manager.DownloadPack(context.Background(), LangCodeKoKr, nil); err == nil {
		t.Fatal("cache write succeeded over a directory")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Clean(path), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	if manager.PackInstalled(LangCodeKoKr) {
		t.Fatal("damaged pack shown as installed")
	}
	if _, err := manager.DownloadPack(context.Background(), LangCodeKoKr, nil); err != nil || !manager.PackInstalled(LangCodeKoKr) {
		t.Fatalf("retry did not replace the damaged cache: %v", err)
	}
}
