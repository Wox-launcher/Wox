package file

type SearchPattern struct {
	Name string // The name of the file or directory.
}

type SearchResult struct {
	Name string
	Path string
}

type Searcher interface {
	Search(pattern SearchPattern) []SearchResult
}
