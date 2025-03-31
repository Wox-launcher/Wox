package file

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"wox/util/shell"
)

var searcher Searcher = &MacSearcher{}

type MacSearcher struct {
}

func (m *MacSearcher) Init(ctx context.Context) error {
	return nil
}

func (m *MacSearcher) Search(pattern SearchPattern) ([]SearchResult, error) {
	// if the search pattern is too short, return empty result
	if len(pattern.Name) <= 3 {
		return []SearchResult{}, nil
	}

	// use mdfind to search files
	cmd := fmt.Sprintf("mdfind \"kMDItemDisplayName=='%s'\" | head -n 20", pattern.Name)
	output, err := shell.RunOutput("bash", "-c", cmd)
	if err != nil {
		return nil, err
	}

	//read output line by line
	var results []SearchResult
	for _, line := range bytes.Split(output, []byte("\n")) {
		if len(line) > 0 {
			path := string(line)
			fileName := filepath.Base(path)
			results = append(results, SearchResult{Name: fileName, Path: path})
		}
	}
	return results, nil
}
