package smokefixture

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
)

// NewStoreServer serves real and invalid plugin packages for local store smoke tests.
func NewStoreServer() (*httptest.Server, error) {
	var packageBytes bytes.Buffer
	archive := zip.NewWriter(&packageBytes)
	manifest, err := archive.Create("plugin.json")
	if err != nil {
		return nil, err
	}
	metadata := `{"Id":"com.wox.smoke.storetoolbar","Name":"Store Toolbar Smoke","Version":"1.0.1","MinWoxVersion":"2.0.0","Runtime":"NODEJS","Entry":"index.js","TriggerKeywords":["storetoolbarsmoke"],"SupportedOS":["Windows","Linux","Darwin"]}`
	if _, err := io.WriteString(manifest, metadata); err != nil {
		return nil, err
	}
	entry, err := archive.Create("index.js")
	if err != nil {
		return nil, err
	}
	if _, err := io.WriteString(entry, `module.exports.plugin = { async init(ctx, params) {}, async query(ctx, query) { return { Results: [] } } }`); err != nil {
		return nil, err
	}
	if err := archive.Close(); err != nil {
		return nil, err
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.URL.Path == "/store.json" {
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{
					"Id": "com.wox.smoke.storetoolbar", "Name": "Store Toolbar Smoke", "Author": "Wox Smoke",
					"Version": "1.0.1", "MinWoxVersion": "2.0.0", "Runtime": "NODEJS", "Description": "Store toolbar fixture",
					"DownloadCount": 100,
					"DownloadUrl":   server.URL + "/fixture.wox", "SupportedOS": []string{"Windows", "Linux", "Darwin"},
				},
				{
					"Id": "com.wox.smoke.storefailure", "Name": "Store Failure Smoke", "Author": "Wox Smoke",
					"Version": "1.0.0", "MinWoxVersion": "2.0.0", "Runtime": "NODEJS", "Description": "Invalid package fixture",
					"DownloadCount": 10,
					"DownloadUrl":   server.URL + "/broken.wox", "SupportedOS": []string{"Windows", "Linux", "Darwin"},
				},
			})
			return
		}
		if request.URL.Path == "/fixture.wox" {
			_, _ = w.Write(packageBytes.Bytes())
			return
		}
		if request.URL.Path == "/broken.wox" {
			_, _ = io.WriteString(w, "invalid store plugin archive")
			return
		}
		http.NotFound(w, request)
	}))
	return server, nil
}
