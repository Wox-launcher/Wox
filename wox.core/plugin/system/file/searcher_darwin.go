package file

/*
#cgo LDFLAGS: -framework CoreServices -framework CoreFoundation
#include <stdbool.h>
#include <stdlib.h>

bool wox_mdquery_search_paths(const char *query, int maxResults, char **outPaths, char **outError);
void wox_mdquery_free(char *ptr);
*/
import "C"

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"unsafe"
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

	paths, err := searchByMDQueryPaths(pattern.Name, 20)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(paths))
	for _, path := range paths {
		fileName := filepath.Base(path)
		results = append(results, SearchResult{Name: fileName, Path: path})
	}

	return results, nil
}

func searchByMDQueryPaths(name string, maxResults int) ([]string, error) {
	if maxResults <= 0 {
		maxResults = 20
	}

	query := fmt.Sprintf("kMDItemDisplayName=='%s'", escapeMDQueryLiteral(name))
	cQuery := C.CString(query)
	defer C.free(unsafe.Pointer(cQuery))

	var cPaths *C.char
	var cErr *C.char
	ok := C.wox_mdquery_search_paths(cQuery, C.int(maxResults), &cPaths, &cErr)

	if cErr != nil {
		defer C.wox_mdquery_free(cErr)
	}
	if cPaths != nil {
		defer C.wox_mdquery_free(cPaths)
	}

	if !ok {
		errMsg := "mdquery search failed"
		if cErr != nil {
			errMsg = C.GoString(cErr)
		}
		return nil, errors.New(errMsg)
	}

	if cPaths == nil {
		return []string{}, nil
	}

	lines := strings.Split(C.GoString(cPaths), "\n")
	paths := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}

	return paths, nil
}

func escapeMDQueryLiteral(value string) string {
	replacer := strings.NewReplacer("\\", "\\\\", "'", "\\'")
	return replacer.Replace(value)
}
