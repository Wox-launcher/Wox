// Package woxview compiles a versioned, declarative JSON view tree into native Wox widgets.
// It is feature independent: plugin windows, settings pages, and previews adapt their own
// data and events to it, and it never imports those features.
package woxview

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// SchemaVersion is the only tree version this build accepts.
const SchemaVersion = 1

// Limits bound untrusted trees before any widget work happens.
const (
	MaxBytes = 4 * 1024 * 1024
	MaxDepth = 64
	MaxNodes = 5000
)

// NodeType names one entry in the component catalog. Values are part of the public wire format.
type NodeType string

const (
	NodeView        NodeType = "view"
	NodeRow         NodeType = "row"
	NodeGrid        NodeType = "grid"
	NodeColumn      NodeType = "column"
	NodeStack       NodeType = "stack"
	NodeScroll      NodeType = "scroll"
	NodeLazyList    NodeType = "lazy-list"
	NodeText        NodeType = "text"
	NodeSpan        NodeType = "span"
	NodeImage       NodeType = "image"
	NodeMorphPath   NodeType = "morph-path"
	NodeSpacer      NodeType = "spacer"
	NodeMarkdown    NodeType = "markdown"
	NodeButton      NodeType = "button"
	NodeIconButton  NodeType = "icon-button"
	NodeTextField   NodeType = "text-field"
	NodeSwitch      NodeType = "switch"
	NodeCheckbox    NodeType = "checkbox"
	NodeSlider      NodeType = "slider"
	NodeProgress    NodeType = "progress"
	NodeLoading     NodeType = "loading"
	NodeDivider     NodeType = "divider"
	NodeForm        NodeType = "form"
	NodeSubmit      NodeType = "submit"
	NodeSelect      NodeType = "select"
	NodeList        NodeType = "list"
	NodeRadioGroup  NodeType = "radio-group"
	NodeTabs        NodeType = "tabs"
	NodeMultiSelect NodeType = "multi-select"
	NodeTags        NodeType = "tags"
	NodeTag         NodeType = "tag"
	NodeBadge       NodeType = "badge"
	NodeNumberField NodeType = "number-field"
	NodeHotkey      NodeType = "hotkey"
	NodeTable       NodeType = "table"
	NodeCode        NodeType = "code"
	NodeLink        NodeType = "link"
)

// HandlerID is an opaque callback identity owned by whoever produced the tree.
type HandlerID string

// Handler references a producer-side callback; the wire form is {"$handler":"h1"}.
type Handler struct {
	ID HandlerID `json:"$handler"`
}

// Tree is an immutable, validated view tree.
type Tree struct {
	handlerSet map[HandlerID]struct{}
	Version    int   `json:"version"`
	Root       *Node `json:"root"`
}

// Node is one validated element. Props holds the catalog-specific *XxxProps value for Type.
type Node struct {
	Type     NodeType
	Key      string
	Props    any
	Children []*Node
}

type rawNode struct {
	Type     NodeType          `json:"type"`
	Key      string            `json:"key,omitempty"`
	Props    json.RawMessage   `json:"props,omitempty"`
	Children []json.RawMessage `json:"children,omitempty"`
}

// Parse decodes and validates a tree. It fails closed: unknown node types, unknown props,
// invalid enum values, and oversized trees are rejected so callers can keep their previous tree.
func Parse(data []byte) (*Tree, error) {
	if len(data) > MaxBytes {
		return nil, invalid("$", "woxview tree exceeds 4 MiB")
	}
	var raw struct {
		Version int             `json:"version"`
		Root    json.RawMessage `json:"root"`
	}
	if err := decodeStrict(data, &raw); err != nil {
		return nil, invalid("$", "invalid woxview tree: %w", err)
	}
	if raw.Version != SchemaVersion {
		return nil, invalid("version", "unsupported woxview version %d", raw.Version)
	}
	if len(raw.Root) == 0 {
		return nil, invalid("root", "woxview tree has no root")
	}
	count := 0
	root, err := decodeNode(raw.Root, 1, &count, "root")
	if err != nil {
		return nil, err
	}
	return NewTree(root)
}

// decodeNode only decodes shape; NewTree applies the shared validation. Depth and count are
// still checked here so a hostile document cannot force deep recursion before validation.
func decodeNode(data json.RawMessage, depth int, count *int, path string) (*Node, error) {
	if depth > MaxDepth {
		return nil, invalid(path, "%s: woxview tree deeper than %d", path, MaxDepth)
	}
	*count++
	if *count > MaxNodes {
		return nil, invalid(path, "woxview tree has more than %d nodes", MaxNodes)
	}
	var raw rawNode
	if err := decodeStrict(data, &raw); err != nil {
		return nil, invalid(path, "%s: %w", path, err)
	}
	spec, ok := catalog[raw.Type]
	if !ok {
		return nil, invalid(path+".type", "%s: unknown node type %q", path, raw.Type)
	}
	path = path + "/" + string(raw.Type)
	props := spec.newProps()
	if len(raw.Props) > 0 && !bytes.Equal(bytes.TrimSpace(raw.Props), []byte("null")) {
		if err := decodeStrict(raw.Props, props); err != nil {
			return nil, invalid(path+".props", "%s props: %w", path, err)
		}
	}
	node := &Node{Type: raw.Type, Key: raw.Key, Props: props}
	for index, child := range raw.Children {
		parsed, err := decodeNode(child, depth+1, count, fmt.Sprintf("%s[%d]", path, index))
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, parsed)
	}
	return node, nil
}

// decodeStrict rejects unknown fields and trailing data so typos surface as errors instead of being ignored.
func decodeStrict(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return errors.New("unexpected trailing data")
	}
	return nil
}

func validateHandler(name string, handler *Handler) error {
	if handler == nil {
		return nil
	}
	if handler.ID == "" || len(handler.ID) > 128 {
		return fmt.Errorf("%s: handler id must be 1-128 bytes", name)
	}
	return nil
}
