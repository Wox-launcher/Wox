package file

import (
	"fmt"
	"wox/util"
)

var searcher Searcher = &MacSearcher{}

type MacSearcher struct {
}

func (m *MacSearcher) Search(pattern SearchPattern) []SearchResult {
	// use mdfind to search files
	// mdfind -onlyin /path/to/search 'kMDItemDisplayName==pattern'

	var arguments = []string{fmt.Sprintf("kMDItemDisplayName=='%s'", pattern.Name)}
	if len(pattern.Paths) > 0 {
		arguments = append(arguments, "-onlyin", pattern.Paths[0])
	}

	output, err := util.ShellRunOutput("mdfind", arguments...)
	if err != nil {
		return nil
	}

	var results []SearchResult
	for _, line := range output {
		results = append(results, SearchResult{Name: string(line), Path: string(line)})
	}
	return []SearchResult{}
}
