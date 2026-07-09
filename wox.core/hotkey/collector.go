package hotkey

// EntrySource identifies which Wox subsystem owns a hotkey.
type EntrySource string

const (
	SourceMain      EntrySource = "main"
	SourceSelection EntrySource = "selection"
	SourceQuery     EntrySource = "query"
	SourceDictation EntrySource = "dictation"
)

// Entry describes one Wox-owned hotkey before it is bound to the platform.
type Entry struct {
	Source     EntrySource
	ID         string
	CombineKey string
	OnPress    func()
	OnRelease  func() // nil = press mode; non-nil = hold mode
}

type collector struct {
	entries []Entry
}

func (c *collector) set(source EntrySource, id string, entry Entry) {
	entry.Source = source
	entry.ID = id
	for i, e := range c.entries {
		if e.Source == source && e.ID == id {
			c.entries[i] = entry
			return
		}
	}
	c.entries = append(c.entries, entry)
}

func (c *collector) remove(source EntrySource, id string) {
	for i, e := range c.entries {
		if e.Source == source && e.ID == id {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			return
		}
	}
}

func (c *collector) replaceSource(source EntrySource, entries []Entry) {
	filtered := c.entries[:0]
	for _, e := range c.entries {
		if e.Source != source {
			filtered = append(filtered, e)
		}
	}
	for _, entry := range entries {
		entry.Source = source
		filtered = append(filtered, entry)
	}
	c.entries = filtered
}

func (c *collector) snapshot() []Entry {
	result := make([]Entry, len(c.entries))
	copy(result, c.entries)
	return result
}

func (c *collector) restore(entries []Entry) {
	c.entries = make([]Entry, len(entries))
	copy(c.entries, entries)
}

func newCollector() *collector {
	return &collector{}
}
