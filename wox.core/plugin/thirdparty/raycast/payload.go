package raycast

import (
	"encoding/json"
)

// Payload is the decrypted Raycast 2.x document. Unknown fields are ignored.
type Payload struct {
	Settings struct {
		General  map[string]json.RawMessage `json:"general"`
		Commands []Command                  `json:"commands"`
	} `json:"settings"`
	Snippets struct {
		Snippets []Snippet `json:"snippets"`
	} `json:"snippets"`
	Quicklinks struct {
		Quicklinks        []Quicklink        `json:"quicklinks"`
		OpenWithPlatforms []OpenWithPlatform `json:"openWithPlatforms"`
	} `json:"quicklinks"`
	ClipboardHistory struct {
		Entries []ClipboardEntry `json:"clipboardEntries"`
	} `json:"clipboardHistory"`
	Notes struct {
		Notes []Note `json:"notes"`
	} `json:"notes"`
}

// Command is one Raycast command binding. Hotkeys may use more than one field name.
type Command struct {
	ExtensionID   string          `json:"extensionId"`
	ID            string          `json:"id"`
	Name          string          `json:"name"`
	Title         string          `json:"title"`
	Alias         string          `json:"alias"`
	Link          string          `json:"link"`
	Path          string          `json:"path"`
	Deeplink      string          `json:"deeplink"`
	MacOSHotkey   json.RawMessage `json:"macosHotkey"`
	Hotkey        json.RawMessage `json:"hotkey"`
	WindowsHotkey json.RawMessage `json:"windowsHotkey"`
}

// Snippet is one exported text snippet.
type Snippet struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Text    string `json:"text"`
	Keyword string `json:"keyword"`
}

// Quicklink is one exported link, file, or folder. OpenWith may be a path or a platform id.
// Path is used when an export keeps the filesystem target separate from Link.
type Quicklink struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Link     string `json:"link"`
	Path     string `json:"path"`
	Type     string `json:"type"`
	OpenWith string `json:"openWith"`
}

// OpenWithPlatform maps a platform id to per-OS application paths.
type OpenWithPlatform struct {
	ID      string `json:"id"`
	MacOS   string `json:"macos"`
	Windows string `json:"windows"`
	Linux   string `json:"linux"`
}

// Note is one exported Raycast note. Text is a short preview; RawContent is the document.
type Note struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Text       string          `json:"text"`
	RawContent json.RawMessage `json:"rawContent"`
}

// ClipboardEntry is one history row. Representations are nested under items.
type ClipboardEntry struct {
	CreatedAt string          `json:"createdAt"`
	Pinned    json.RawMessage `json:"pinned"`
	Items     []struct {
		Representations []ClipboardRepresentation `json:"representations"`
	} `json:"items"`
}

// ClipboardRepresentation is one mime payload inside a clipboard entry.
type ClipboardRepresentation struct {
	MimeType    string `json:"mimeType"`
	ContentType string `json:"contentType"`
	Content     string `json:"content"`
}

// ParsePayload decodes the export and rejects documents with too many rows.
func ParsePayload(raw []byte, limits Limits) (Payload, error) {
	var payload Payload
	if err := json.Unmarshal(raw, &payload); err != nil {
		return Payload{}, exportErr(codeCorrupt)
	}
	entries := len(payload.Settings.Commands) + len(payload.Snippets.Snippets) + len(payload.Quicklinks.Quicklinks) + len(payload.ClipboardHistory.Entries) + len(payload.Notes.Notes)
	if limits.MaxEntries > 0 && entries > limits.MaxEntries {
		return Payload{}, exportErr(codeLimit)
	}
	return payload, nil
}
