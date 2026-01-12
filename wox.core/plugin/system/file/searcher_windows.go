package file

import (
	"context"
	"path"
	"path/filepath"
	"wox/util"
)

var searcher Searcher = &WindowsSearcher{}

type WindowsSearcher struct {
}

func (m *WindowsSearcher) Init(ctx context.Context) error {
	dllPath := path.Join(util.GetLocation().GetOthersDirectory(), "Everything3_x64.dll")
	initEverythingDLL(dllPath)

	legacyDLLPath := path.Join(util.GetLocation().GetOthersDirectory(), "Everything64.dll")
	initEverything2DLL(legacyDLLPath)
	return nil
}

func (m *WindowsSearcher) Search(pattern SearchPattern) ([]SearchResult, error) {
	var results []SearchResult

	// Everything 1.15 SDK
	err := Walk(pattern.Name, 100, func(path string, info FileInfo, err error) error {
		if err != nil {
			return err
		}

		results = append(results, SearchResult{
			Name: filepath.Base(path),
			Path: path,
		})

		return nil
	})
	// Everything 1.4 SDK
	if err == EverythingNotRunningError {
		err = walkEverything2(pattern.Name, 100, func(path string, info FileInfo, err error) error {
			if err != nil {
				return err
			}

			results = append(results, SearchResult{
				Name: filepath.Base(path),
				Path: path,
			})

			return nil
		})
	}
	if err != nil {
		return nil, err
	}
	return results, nil
}
