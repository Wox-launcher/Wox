package file

import "context"

var searcher Searcher = &LinuxSearcher{}

type LinuxSearcher struct {
}

func (m *LinuxSearcher) Init(ctx context.Context) error {
	return nil
}

func (m *LinuxSearcher) Search(pattern SearchPattern) ([]SearchResult, error) {
	return []SearchResult{}, nil
}
