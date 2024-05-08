package file

type SearchPattern struct {
	Name  string   // The name of the file or directory.
	Paths []string // Search path if specified.
}

type SearchResult struct {
	Name string
	Path string
}

type Searcher interface {
	Search(pattern SearchPattern) []SearchResult
}
