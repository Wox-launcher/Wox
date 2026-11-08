package licenses

import (
	"embed"
	"encoding/json"
	"runtime"
	"sync"

	"wox/internal/assetfs"
)

// Entry contains the attribution and original license text for a bundled dependency.
type Entry struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
	Text    string `json:"text"`
}

//go:embed catalog
var raw embed.FS

var assets = assetfs.New(raw)

// Entries loads the license catalog lazily. Callers must treat the entries as read-only.
var Entries = sync.OnceValue(func() []Entry {
	data, err := assets.ReadFile("catalog/" + runtime.GOOS + "-" + runtime.GOARCH + ".json")
	if err != nil {
		return nil
	}
	var entries []Entry
	if json.Unmarshal(data, &entries) != nil {
		return nil
	}
	return entries
})
