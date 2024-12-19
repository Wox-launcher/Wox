package file

var searcher Searcher = &LinuxSearcher{}

type LinuxSearcher struct {
}

func (m *LinuxSearcher) Search(pattern SearchPattern) []SearchResult {
	return []SearchResult{}
}
