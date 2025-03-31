package file

import (
	"context"
	"path"
	"syscall"
	"unsafe"
	"wox/util"
)

var (
	user32          = syscall.NewLazyDLL("user32.dll")
	procFindWindowW = user32.NewProc("FindWindowW")
)

func FindWindow(className, windowName string) uintptr {
	cn, _ := syscall.UTF16PtrFromString(className)
	wn, _ := syscall.UTF16PtrFromString(windowName)
	ret, _, _ := procFindWindowW.Call(
		uintptr(unsafe.Pointer(cn)),
		uintptr(unsafe.Pointer(wn)),
	)
	return ret
}

var searcher Searcher = &WindowsSearcher{}

type WindowsSearcher struct {
}

func (m *WindowsSearcher) Init(ctx context.Context) error {
	dllPath := path.Join(util.GetLocation().GetOthersDirectory(), "Everything64.dll")
	initEverythingDLL(dllPath)
	return nil
}

func (m *WindowsSearcher) Search(pattern SearchPattern) ([]SearchResult, error) {
	// if everything is not running, return error
	hWnd := FindWindow("EVERYTHING_TASKBAR_NOTIFICATION", "")
	if hWnd == 0 {
		return nil, EverythingNotRunningError
	}

	var results []SearchResult
	Walk(pattern.Name, 100, func(path string, info FileInfo, err error) error {
		if err != nil {
			return err
		}

		results = append(results, SearchResult{
			Name: info.Name(),
			Path: path,
		})

		return nil
	})
	return results, nil
}
