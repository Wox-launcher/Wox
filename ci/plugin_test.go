package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestUpdatePluginStore covers upgrades and compatibility repairs without altering other fields.
func TestUpdatePluginStore(t *testing.T) {
	const original = `[
  {"Id":"test-plugin","Name":"Test","Version":"1.0.0","MinWoxVersion":"2.0.0","DateUpdated":"old","Custom":{"keep":true}}
]`
	for _, tc := range []struct {
		name       string
		metadata   storePluginManifest
		loadErr    error
		wantChange bool
		wantMin    string
	}{
		{name: "upgrade", metadata: storePluginManifest{Id: "test-plugin", Version: "1.1.0", MinWoxVersion: "2.4.5"}, wantChange: true, wantMin: "2.4.5"},
		{name: "repair same version", metadata: storePluginManifest{Id: "test-plugin", Version: "1.0.0", MinWoxVersion: "2.4.5"}, wantChange: true, wantMin: "2.4.5"},
		{name: "legacy default", metadata: storePluginManifest{Id: "test-plugin", Version: "1.1.0"}, wantChange: true, wantMin: "2.0.0"},
		{name: "unchanged", metadata: storePluginManifest{Id: "test-plugin", Version: "v1.0.0", MinWoxVersion: "2.0.0"}},
		{name: "no downgrade", metadata: storePluginManifest{Id: "test-plugin", Version: "0.9.0", MinWoxVersion: "2.4.5"}},
		{name: "invalid minimum", metadata: storePluginManifest{Id: "test-plugin", Version: "1.1.0", MinWoxVersion: "invalid"}},
		{name: "invalid version", metadata: storePluginManifest{Id: "test-plugin", Version: "invalid", MinWoxVersion: "2.4.5"}},
		{name: "wrong plugin", metadata: storePluginManifest{Id: "different", Version: "1.1.0", MinWoxVersion: "2.4.5"}},
		{name: "download failure", loadErr: fmt.Errorf("download failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := updatePluginStore([]byte(original), func(string) (storePluginManifest, error) {
				return tc.metadata, tc.loadErr
			}, func(string) (*pluginRelease, error) { return nil, nil })
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantChange {
				if string(updated) != original {
					t.Fatalf("unexpected catalog change: %s", updated)
				}
				return
			}
			var plugins []struct {
				storePluginManifest
				Custom map[string]bool
			}
			if err := json.Unmarshal(updated, &plugins); err != nil {
				t.Fatal(err)
			}
			if plugins[0].Version != tc.metadata.Version || plugins[0].MinWoxVersion != tc.wantMin || (plugins[0].DateUpdated == "old") != (tc.name == "repair same version") {
				t.Fatalf("unexpected updated manifest: %+v", plugins[0])
			}
			if !plugins[0].Custom["keep"] || !strings.Contains(string(updated), `"Custom":{"keep":true}`) {
				t.Fatalf("unrelated field was changed: %s", updated)
			}
		})
	}
}

func TestUpdatePluginStoreReleaseDate(t *testing.T) {
	const original = `[{"Id":"test","Version":"1.0.0","MinWoxVersion":"2.0.0","DateUpdated":"2020-01-01 00:00:00","DownloadUrl":"latest"}]`
	metadata := storePluginManifest{Id: "test", Version: "1.1.0", MinWoxVersion: "2.0.0"}
	published := time.Date(2026, 11, 4, 1, 22, 32, 0, time.UTC)
	for _, tc := range []struct {
		name       string
		releaseErr error
		pinned     storePluginManifest
		pinnedErr  error
		wantUpdate bool
	}{
		{name: "release date", pinned: metadata, wantUpdate: true},
		{name: "release lookup failed", releaseErr: fmt.Errorf("offline"), pinned: metadata},
		{name: "release changed", pinned: storePluginManifest{Id: "test", Version: "1.2.0", MinWoxVersion: "2.0.0"}},
		{name: "pinned download failed", pinnedErr: fmt.Errorf("offline")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			updated, err := updatePluginStore([]byte(original), func(url string) (storePluginManifest, error) {
				if url == "latest" {
					return metadata, nil
				}
				if url != "pinned" {
					t.Fatalf("unexpected URL %q", url)
				}
				return tc.pinned, tc.pinnedErr
			}, func(string) (*pluginRelease, error) {
				return &pluginRelease{DownloadURL: "pinned", PublishedAt: published}, tc.releaseErr
			})
			if err != nil {
				t.Fatal(err)
			}
			if !tc.wantUpdate && string(updated) != original {
				t.Fatalf("unexpected update: %s", updated)
			}
			if tc.wantUpdate && (!strings.Contains(string(updated), `"Version":"1.1.0"`) || !strings.Contains(string(updated), `"DateUpdated":"2026-11-04 01:22:32"`)) {
				t.Fatalf("wrong release update: %s", updated)
			}
		})
	}
}

func TestFetchPluginRelease(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/owner/repo/releases/latest" && r.URL.Path != "/repos/owner/repo/releases/tags/v1.0" {
			t.Errorf("unexpected API path %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("GitHub token was not sent")
		}
		fmt.Fprint(w, `{"tag_name":"v1.0","published_at":"2026-11-04T09:22:32+08:00","assets":[{"name":"plugin.wox"}]}`)
	}))
	defer server.Close()
	for _, downloadURL := range []string{
		"https://github.com/owner/repo/releases/latest/download/plugin.wox",
		"https://github.com/owner/repo/releases/download/v1.0/plugin.wox",
	} {
		release, err := fetchPluginRelease(downloadURL, server.URL, server.Client())
		if err != nil || release == nil || release.DownloadURL != "https://github.com/owner/repo/releases/download/v1.0/plugin.wox" || release.PublishedAt.UTC().Format(time.RFC3339) != "2026-11-04T01:22:32Z" {
			t.Fatalf("release for %q = %+v, %v", downloadURL, release, err)
		}
	}
	if release, err := fetchPluginRelease("https://gist.github.com/owner/123", server.URL, server.Client()); err != nil || release != nil {
		t.Fatalf("non-release URL = %+v, %v", release, err)
	}
	if _, err := fetchPluginRelease("https://github.com/owner/repo/releases/latest/download/missing.wox", server.URL, server.Client()); err == nil {
		t.Fatal("missing asset must fail")
	}
}

func TestFetchPluginReleaseRejectsUnpublishedRelease(t *testing.T) {
	for _, body := range []string{
		`{"tag_name":"v1.0","published_at":null,"assets":[{"name":"plugin.wox"}]}`,
		`{"tag_name":"v1.0","published_at":"2026-11-04T01:22:32Z","draft":true,"assets":[{"name":"plugin.wox"}]}`,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, body)
		}))
		_, err := fetchPluginRelease("https://github.com/owner/repo/releases/latest/download/plugin.wox", server.URL, server.Client())
		server.Close()
		if err == nil {
			t.Fatalf("expected an error for %s", body)
		}
	}
}

// TestGetLatestPluginMetadata exercises real downloads of packages and both inline comment formats.
func TestGetLatestPluginMetadata(t *testing.T) {
	const manifest = `{"Id":"test-plugin","Version":"1.2.3-beta.1","MinWoxVersion":"2.4.5"}`
	for _, tc := range []struct {
		name    string
		path    string
		body    []byte
		status  int
		wantErr bool
	}{
		{name: "package", path: "/plugin.wox?download=1", body: pluginTestPackage(t, "plugin.json", manifest)},
		{name: "python", path: "/plugin.py", body: []byte("#!/usr/bin/env python3\r\n# Plugin\r\n# " + manifest + "\r\nprint('hello')")},
		{name: "nodejs long header", path: "/plugin.js", body: []byte(strings.Repeat("// Header\n", 25) + "// " + manifest + "\nconsole.log('hello')")},
		{name: "extensionless gist", path: "/raw/plugin", body: []byte("# " + manifest)},
		{name: "multiline metadata", path: "/plugin.py", body: []byte("# {\n# \"Id\":\"test-plugin\",\n# \"Version\":\"1.2.3-beta.1\",\n# \"MinWoxVersion\":\"2.4.5\",\n# \"Description\":\"contains } and \\\"quotes\\\"\"\n# }\n# trailing notes\nprint('hello')")},
		{name: "missing manifest", path: "/plugin.wox", body: pluginTestPackage(t, "nested/plugin.json", manifest), wantErr: true},
		{name: "invalid zip", path: "/plugin.wox", body: []byte("not a zip"), wantErr: true},
		{name: "invalid manifest", path: "/plugin.wox", body: pluginTestPackage(t, "plugin.json", "{"), wantErr: true},
		{name: "no header", path: "/plugin.py", body: []byte("print('hello')\n# " + manifest), wantErr: true},
		{name: "invalid inline JSON", path: "/plugin.js", body: []byte("// {broken}"), wantErr: true},
		{name: "http failure", path: "/plugin.py", body: []byte("# " + manifest), status: http.StatusNotFound, wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.status != 0 {
					w.WriteHeader(tc.status)
				}
				_, _ = w.Write(tc.body)
			}))
			defer server.Close()
			metadata, err := getLatestPluginMetadata(server.URL + tc.path)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %+v", metadata)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if metadata.Id != "test-plugin" || metadata.Version != "1.2.3-beta.1" || metadata.MinWoxVersion != "2.4.5" {
				t.Fatalf("unexpected metadata: %+v", metadata)
			}
		})
	}
}

// pluginTestPackage builds an in-memory plugin archive for download tests.
func pluginTestPackage(t *testing.T, name, content string) []byte {
	t.Helper()
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	file, err := writer.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := file.Write([]byte(content)); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}
