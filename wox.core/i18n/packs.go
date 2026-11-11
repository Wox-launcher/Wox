package i18n

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"
	"wox/util"
)

const languagePackURL = "https://raw.githubusercontent.com/Wox-launcher/Wox/master/lang/"
const languagePackLimit = 2 * 1024 * 1024

// IsBundled reports which languages are always available, including on first launch offline.
func IsBundled(code LangCode) bool {
	return code == LangCodeEnUs || code == LangCodeZhCn
}

// Pack is a validated language table ready to publish without further disk I/O.
type Pack struct {
	code         LangCode
	translations map[string]string
}

type cachedPack struct {
	Data json.RawMessage `json:"translations"`
}

// packStore owns the persistent cache and shares in-flight transfers across callers.
type packStore struct {
	directory      string
	refreshStarted atomic.Bool
	download       func(context.Context, string, int64, func(int64, int64)) ([]byte, error)
	timeout        time.Duration
	jobs           *util.HashMap[LangCode, *packDownload]
	checked        *util.HashMap[LangCode, packFile]
}

type packFile struct {
	info  os.FileInfo
	valid bool
}

type packDownload struct {
	done     chan struct{}
	progress atomic.Pointer[PackProgress]
	pack     Pack
	err      error
}

// PackProgress reports a transfer or the subsequent application. Total is negative until a transfer's size is known.
type PackProgress struct {
	Received int64
	Total    int64
	Applying bool
}

// ConfigurePacks sets the cache location before startup loads settings.
func (m *Manager) ConfigurePacks(directory string) {
	m.packs = &packStore{
		directory: directory, download: util.HttpDownloadBytes, timeout: 30 * time.Second,
		jobs: util.NewHashMap[LangCode, *packDownload](), checked: util.NewHashMap[LangCode, packFile](),
	}
}

// PackInstalled checks the cache only when the file identity or contents have changed.
func (m *Manager) PackInstalled(code LangCode) bool {
	if IsBundled(code) {
		return true
	}
	if m.packs == nil || !IsSupportedLangCode(string(code)) {
		return false
	}
	info, err := os.Stat(m.packs.path(code))
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	if checked, known := m.packs.checked.Load(code); known && os.SameFile(checked.info, info) &&
		checked.info.ModTime() == info.ModTime() && checked.info.Size() == info.Size() {
		return checked.valid
	}
	_, err = m.packs.read(code)
	return err == nil
}

// RefreshCachedPacks updates installed downloadable packs once per process startup.
func (m *Manager) RefreshCachedPacks(ctx context.Context, onRefreshed func(LangCode)) {
	if m.packs == nil || !m.packs.refreshStarted.CompareAndSwap(false, true) {
		return
	}
	for _, language := range GetSupportedLanguages() {
		if ctx.Err() != nil {
			return
		}
		code := language.Code
		if IsBundled(code) || !m.PackInstalled(code) {
			continue
		}
		if _, err := m.DownloadPack(ctx, code, nil); err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("refresh cached language %s: %v", code, err))
			continue
		}
		if onRefreshed != nil {
			onRefreshed(code)
		}
	}
}

func (s *packStore) path(code LangCode) string {
	return filepath.Join(s.directory, string(code)+".json")
}

// read checks the cached envelope and remembers the validation result for the chooser.
func (s *packStore) read(code LangCode) (pack Pack, resultErr error) {
	file, err := os.Open(s.path(code))
	if err != nil {
		return Pack{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return Pack{}, err
	}
	if !info.Mode().IsRegular() {
		return Pack{}, errors.New("language pack cache is not a regular file")
	}
	defer func() { s.checked.Store(code, packFile{info: info, valid: resultErr == nil}) }()
	var cached cachedPack
	const cacheLimit = languagePackLimit + 1024
	data, err := io.ReadAll(io.LimitReader(file, cacheLimit+1))
	if err != nil {
		return Pack{}, err
	}
	if len(data) > cacheLimit {
		return Pack{}, errors.New("language pack cache exceeds size limit")
	}
	if err := json.Unmarshal(data, &cached); err != nil {
		return Pack{}, err
	}
	translations, err := parsePack(cached.Data)
	return Pack{code: code, translations: translations}, err
}

// DownloadPack joins an existing transfer or fetches a fresh copy from the official source.
func (m *Manager) DownloadPack(ctx context.Context, code LangCode, progress func(PackProgress)) (Pack, error) {
	if IsBundled(code) || !IsSupportedLangCode(string(code)) {
		return Pack{}, fmt.Errorf("no downloadable language pack for %s", code)
	}
	if m.packs == nil {
		return Pack{}, errors.New("language pack cache is unavailable")
	}
	if err := ctx.Err(); err != nil {
		return Pack{}, err
	}
	s := m.packs
	pending := &packDownload{done: make(chan struct{})}
	pending.progress.Store(&PackProgress{Total: -1})
	job, exists := s.jobs.LoadOrStore(code, pending)
	if !exists {
		// Canceling a picker cancels its switch, while other waiters can still use the transfer.
		util.Go(context.WithoutCancel(ctx), "download language pack", func() {
			defer close(job.done)
			defer s.jobs.Delete(code)
			ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), s.timeout)
			defer cancel()
			job.pack, job.err = s.fetch(ctx, code, func(received, total int64) {
				job.progress.Store(&PackProgress{Received: received, Total: total})
			})
		})
	}
	var updates <-chan time.Time
	if progress != nil {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()
		updates = ticker.C
	}
	var last *PackProgress
	report := func() {
		if latest := job.progress.Load(); progress != nil && latest != nil && latest != last {
			last = latest
			progress(*latest)
		}
	}
	report()
	for {
		select {
		case <-ctx.Done():
			return Pack{}, ctx.Err()
		case <-job.done:
			report()
			return job.pack, job.err
		case <-updates:
			report()
		}
	}
}

func (s *packStore) fetch(ctx context.Context, code LangCode, progress func(int64, int64)) (Pack, error) {
	data, err := s.download(ctx, languagePackURL+string(code)+".json", languagePackLimit, progress)
	if err != nil {
		return Pack{}, fmt.Errorf("download language pack: %w", err)
	}
	translations, err := parsePack(data)
	if err != nil {
		return Pack{}, fmt.Errorf("validate language pack: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return Pack{}, err
	}
	if err := s.write(code, data); err != nil {
		return Pack{}, fmt.Errorf("save language pack: %w", err)
	}
	return Pack{code: code, translations: translations}, nil
}

// parsePack accepts partial catalogs so releases can share the master translations.
func parsePack(data []byte) (map[string]string, error) {
	if len(data) > languagePackLimit || !utf8.Valid(data) {
		return nil, errors.New("invalid language pack size or encoding")
	}
	var translations map[string]string
	if err := json.Unmarshal(data, &translations); err != nil {
		return nil, err
	}
	if strings.TrimSpace(translations["ui_lang"]) == "" {
		return nil, errors.New("language pack is missing the language label")
	}
	for key, value := range translations {
		if strings.TrimSpace(value) == "" {
			delete(translations, key)
		}
	}
	return translations, nil
}

// write replaces a cache entry only after its new contents have been validated and written.
func (s *packStore) write(code LangCode, data []byte) error {
	if err := os.MkdirAll(s.directory, 0700); err != nil {
		return err
	}
	file, err := os.CreateTemp(s.directory, ".language-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	encoder := json.NewEncoder(file)
	encoder.SetEscapeHTML(false)
	err = encoder.Encode(cachedPack{Data: data})
	closeErr := file.Close()
	if err != nil || closeErr != nil {
		return errors.Join(err, closeErr)
	}
	return os.Rename(file.Name(), s.path(code))
}
