package file

import "context"

type SearchPattern struct {
	Name string // The name of the file or directory.
}

type SearchResult struct {
	Name string
	Path string
}

type Searcher interface {
	Init(ctx context.Context) error
	Search(pattern SearchPattern) ([]SearchResult, error)
}
