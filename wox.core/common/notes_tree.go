package common

// NoteGroup is independently synced; deleted groups retain their parent for late child updates.
type NoteGroup struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ParentID string `json:"parentId"`
	Order    int    `json:"order"`
	Deleted  bool   `json:"deleted,omitempty"`
}

// NotePlacement keeps organization changes separate from document revisions.
type NotePlacement struct {
	NoteID   string `json:"noteId"`
	ParentID string `json:"parentId"`
	Order    int    `json:"order"`
}

// NotesTree contains raw organization records; consumers resolve missing or deleted parents.
type NotesTree struct {
	Groups     []NoteGroup
	Placements []NotePlacement
}

// NotesTreeChange describes one atomic organization command. BeforeID is a same-kind sibling.
type NotesTreeChange struct {
	Action   string
	ID       string
	Group    bool
	Name     string
	ParentID string
	BeforeID string
	First    bool
}

// ResolveNoteParent follows deleted ancestors and exposes malformed remote trees at the root.
func ResolveNoteParent(parent string, groups map[string]NoteGroup) string {
	seen := map[string]bool{}
	resolved := ""
	for parent != "" {
		if seen[parent] {
			return ""
		}
		seen[parent] = true
		group, ok := groups[parent]
		if !ok {
			return ""
		}
		if resolved == "" && !group.Deleted {
			resolved = parent
		}
		parent = group.ParentID
	}
	return resolved
}
