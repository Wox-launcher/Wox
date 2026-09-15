package common

import (
	"fmt"
	"strings"
)

const (
	QueryElementText     = "text"
	QueryElementArgument = "argument"
	QueryElementBlock    = "block"
)

// QueryHint carries semantic values and background guidance for continuous input.
// It describes supplied query text; it never supplies or overrides that text.
// Command templates contain only suffix elements; instances include the command text.
type QueryHint struct {
	Elements []QueryElement
	// CommandSuggestions identifies core-owned command guidance; it is not part of plugin JSON.
	CommandSuggestions bool `json:"-"`
}

// QueryElement separates editable values from placeholders and atomic content.
type QueryElement struct {
	Id          string
	Kind        string
	Text        string     `json:",omitempty"`
	Value       string     `json:",omitempty"`
	Placeholder I18nString `json:",omitempty"`
	Required    bool       `json:",omitempty"`
	// Suggestions are literal, ordered input candidates, not constraints or translated labels.
	Suggestions []string `json:",omitempty"`
}

func (e QueryElement) Content() string {
	if e.Kind == QueryElementText {
		return e.Text
	}
	return e.Value
}

// Clone isolates edits and history snapshots from shared command templates.
func (s *QueryHint) Clone() *QueryHint {
	if s == nil {
		return nil
	}
	clone := &QueryHint{Elements: append([]QueryElement(nil), s.Elements...), CommandSuggestions: s.CommandSuggestions}
	for i := range clone.Elements {
		clone.Elements[i].Suggestions = append([]string(nil), s.Elements[i].Suggestions...)
	}
	return clone
}

// PlainText provides a lossy compatibility projection, never a serialization format.
func (s *QueryHint) PlainText() string {
	if s == nil {
		return ""
	}
	var result strings.Builder
	for _, e := range s.Elements {
		result.WriteString(e.Content())
	}
	return result.String()
}

// Argument looks up a named value without attempting to parse its text projection.
func (s *QueryHint) Argument(id string) string {
	if s != nil {
		for _, e := range s.Elements {
			if e.Id == id && e.Kind == QueryElementArgument {
				return e.Value
			}
		}
	}
	return ""
}

// Validate rejects malformed input before it can replace the user's current query.
func (s *QueryHint) Validate() error {
	if s == nil {
		return nil
	}
	if len(s.Elements) == 0 {
		return fmt.Errorf("query hint must contain elements")
	}
	seen := make(map[string]bool, len(s.Elements))
	for _, e := range s.Elements {
		if e.Id == "" || seen[e.Id] {
			return fmt.Errorf("query element id must be nonempty and unique: %q", e.Id)
		}
		seen[e.Id] = true
		if e.Kind != QueryElementArgument && len(e.Suggestions) > 0 {
			return fmt.Errorf("non-argument element %q has suggestions", e.Id)
		}
		switch e.Kind {
		case QueryElementText:
			if e.Value != "" || e.Placeholder != "" || e.Required {
				return fmt.Errorf("text element %q has argument fields", e.Id)
			}
		case QueryElementArgument, QueryElementBlock:
			if e.Text != "" {
				return fmt.Errorf("value element %q has text", e.Id)
			}
			if e.Kind == QueryElementBlock && (e.Placeholder != "" || e.Required) {
				return fmt.Errorf("block %q has argument fields", e.Id)
			}
		default:
			return fmt.Errorf("unknown query element kind %q", e.Kind)
		}
	}
	return nil
}

// NormalizeForQuery discards invalid or stale decoration without blocking the query.
func (s *QueryHint) NormalizeForQuery(queryType, text string) *QueryHint {
	if s == nil || queryType != "input" || s.Validate() != nil || s.PlainText() != text {
		return nil
	}
	return s
}
