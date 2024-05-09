package file

var searcher Searcher = &WindowsSearcher{}

type WindowsSearcher struct {
}

func (m *WindowsSearcher) Search(pattern SearchPattern) []SearchResult {
	return []SearchResult{}
}
