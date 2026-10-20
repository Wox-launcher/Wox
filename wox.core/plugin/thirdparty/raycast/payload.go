package raycast

import (
	"encoding/json"
)

// Payload is the decrypted Raycast 2.x document. Unknown fields are ignored.
type Payload struct {
	NodeExtensions struct {
		Extensions []ExportExtension `json:"extensions"`
	} `json:"nodeExtensions"`
	Settings struct {
		General            map[string]json.RawMessage `json:"general"`
		Commands           []Command                  `json:"commands"`
		InternalExtensions []InternalExtension        `json:"internalExtensions"`
		NodeExtensions     []ExtensionSettings        `json:"nodeExtensions"`
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
	Emoji struct {
		Emojis []EmojiEntry `json:"emojis"`
	} `json:"emoji"`
	MCPServers struct {
		Servers []MCPServer `json:"servers"`
	} `json:"mcpServers"`
}

// ExportExtension identifies code installed locally; the export itself contains no executable files.
type ExportExtension struct {
	UUID        string                     `json:"uuid"`
	Name        string                     `json:"name"`
	Author      string                     `json:"author"`
	Owner       string                     `json:"owner"`
	Version     string                     `json:"version"`
	Preferences map[string]json.RawMessage `json:"-"`
}

// ExtensionSettings holds explicit preference values, separate from type descriptions and runtime storage.
type ExtensionSettings struct {
	ID   string                     `json:"id"`
	Meta map[string]json.RawMessage `json:"meta"`
}

// EmojiEntry is one symbol the user has used. CustomKeywords are extra search words.
type EmojiEntry struct {
	Symbol         string   `json:"symbol"`
	CustomKeywords []string `json:"customKeywords"`
}

// MCPServer is one exported stdio or HTTP MCP server.
type MCPServer struct {
	Title     string   `json:"title"`
	Type      string   `json:"type"`
	Command   string   `json:"command"`
	Arguments []string `json:"arguments"`
	URL       string   `json:"url"`
}

// InternalExtension is one built-in Raycast extension and the preferences stored in its meta object.
type InternalExtension struct {
	ID   string          `json:"id"`
	Meta json.RawMessage `json:"meta"`
}

// Command is one Raycast command binding. Hotkeys may use more than one field name.
type Command struct {
	Meta          map[string]json.RawMessage `json:"meta"`
	ExtensionID   string                     `json:"extensionId"`
	ID            string                     `json:"id"`
	Name          string                     `json:"name"`
	Title         string                     `json:"title"`
	Alias         string                     `json:"alias"`
	Link          string                     `json:"link"`
	Path          string                     `json:"path"`
	Deeplink      string                     `json:"deeplink"`
	MacOSHotkey   json.RawMessage            `json:"macosHotkey"`
	Hotkey        json.RawMessage            `json:"hotkey"`
	WindowsHotkey json.RawMessage            `json:"windowsHotkey"`
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
	entries := len(payload.NodeExtensions.Extensions) + len(payload.Settings.NodeExtensions) + len(payload.Settings.Commands) + len(payload.Snippets.Snippets) + len(payload.Quicklinks.Quicklinks) + len(payload.ClipboardHistory.Entries) + len(payload.Notes.Notes)
	if limits.MaxEntries > 0 && entries > limits.MaxEntries {
		return Payload{}, exportErr(codeLimit)
	}
	return payload, nil
}
