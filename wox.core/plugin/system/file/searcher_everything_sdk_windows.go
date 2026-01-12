// Search Everything : http://voidtools.com/
// https://www.voidtools.com/forum/viewtopic.php?t=15853
// https://github.com/voidtools/everything_sdk3
package file

import (
	"fmt"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

const (
	EVERYTHING3_OK                        = 0
	EVERYTHING3_ERROR_IPC_PIPE_NOT_FOUND  = 0xE0000002
	EVERYTHING3_PROPERTY_ID_SIZE          = 2
	EVERYTHING3_PROPERTY_ID_DATE_MODIFIED = 5
	EVERYTHING3_PROPERTY_ID_PATH_AND_NAME = 240
)

// general
var Everything3_GetLastError *syscall.LazyProc
var Everything3_ConnectW *syscall.LazyProc
var Everything3_DestroyClient *syscall.LazyProc
var Everything3_GetMajorVersion *syscall.LazyProc
var Everything3_GetMinorVersion *syscall.LazyProc
var Everything3_GetRevision *syscall.LazyProc

// search state
var Everything3_CreateSearchState *syscall.LazyProc
var Everything3_DestroySearchState *syscall.LazyProc
var Everything3_SetSearchMatchCase *syscall.LazyProc
var Everything3_SetSearchMatchWholeWords *syscall.LazyProc
var Everything3_SetSearchMatchPath *syscall.LazyProc
var Everything3_SetSearchRegex *syscall.LazyProc
var Everything3_SetSearchTextW *syscall.LazyProc
var Everything3_SetSearchViewportCount *syscall.LazyProc
var Everything3_AddSearchPropertyRequest *syscall.LazyProc

// execute search
var Everything3_Search *syscall.LazyProc

// result list
var Everything3_DestroyResultList *syscall.LazyProc
var Everything3_GetResultListViewportCount *syscall.LazyProc
var Everything3_IsFolderResult *syscall.LazyProc
var Everything3_GetResultFullPathNameW *syscall.LazyProc
var Everything3_GetResultSize *syscall.LazyProc
var Everything3_GetResultDateModified *syscall.LazyProc

var everything3DLL *syscall.LazyDLL
var clientMu sync.Mutex
var cachedClient uintptr

func initEverythingDLL(dllPath string) {
	everything3DLL = syscall.NewLazyDLL(dllPath)
	if everything3DLL != nil {
		Everything3_GetLastError = everything3DLL.NewProc("Everything3_GetLastError")
		Everything3_ConnectW = everything3DLL.NewProc("Everything3_ConnectW")
		Everything3_DestroyClient = everything3DLL.NewProc("Everything3_DestroyClient")
		Everything3_GetMajorVersion = everything3DLL.NewProc("Everything3_GetMajorVersion")
		Everything3_GetMinorVersion = everything3DLL.NewProc("Everything3_GetMinorVersion")
		Everything3_GetRevision = everything3DLL.NewProc("Everything3_GetRevision")

		Everything3_CreateSearchState = everything3DLL.NewProc("Everything3_CreateSearchState")
		Everything3_DestroySearchState = everything3DLL.NewProc("Everything3_DestroySearchState")
		Everything3_SetSearchMatchCase = everything3DLL.NewProc("Everything3_SetSearchMatchCase")
		Everything3_SetSearchMatchWholeWords = everything3DLL.NewProc("Everything3_SetSearchMatchWholeWords")
		Everything3_SetSearchMatchPath = everything3DLL.NewProc("Everything3_SetSearchMatchPath")
		Everything3_SetSearchRegex = everything3DLL.NewProc("Everything3_SetSearchRegex")
		Everything3_SetSearchTextW = everything3DLL.NewProc("Everything3_SetSearchTextW")
		Everything3_SetSearchViewportCount = everything3DLL.NewProc("Everything3_SetSearchViewportCount")
		Everything3_AddSearchPropertyRequest = everything3DLL.NewProc("Everything3_AddSearchPropertyRequest")

		Everything3_Search = everything3DLL.NewProc("Everything3_Search")

		Everything3_DestroyResultList = everything3DLL.NewProc("Everything3_DestroyResultList")
		Everything3_GetResultListViewportCount = everything3DLL.NewProc("Everything3_GetResultListViewportCount")
		Everything3_IsFolderResult = everything3DLL.NewProc("Everything3_IsFolderResult")
		Everything3_GetResultFullPathNameW = everything3DLL.NewProc("Everything3_GetResultFullPathNameW")
		Everything3_GetResultSize = everything3DLL.NewProc("Everything3_GetResultSize")
		Everything3_GetResultDateModified = everything3DLL.NewProc("Everything3_GetResultDateModified")
	}
}

func connectEverythingClient() uintptr {
	if Everything3_ConnectW == nil {
		return 0
	}
	client, _, _ := Everything3_ConnectW.Call(0)
	if client != 0 {
		return client
	}
	instance, _ := syscall.UTF16PtrFromString("1.5a")
	client, _, _ = Everything3_ConnectW.Call(uintptr(unsafe.Pointer(instance)))
	return client
}

func destroyEverythingClient(client uintptr) {
	if Everything3_DestroyClient != nil && client != 0 {
		Everything3_DestroyClient.Call(client)
	}
}

func getCachedClient() uintptr {
	clientMu.Lock()
	defer clientMu.Unlock()

	if cachedClient != 0 {
		return cachedClient
	}
	cachedClient = connectEverythingClient()
	return cachedClient
}

func resetCachedClient() {
	clientMu.Lock()
	defer clientMu.Unlock()

	if cachedClient != 0 {
		destroyEverythingClient(cachedClient)
		cachedClient = 0
	}
}

// FileInfo resemble os.FileInfo
type FileInfo struct {
	name    string
	size    int64
	modTime time.Time
	isDir   bool
}

func (fi *FileInfo) Name() string       { return fi.name }
func (fi *FileInfo) Size() int64        { return fi.size }
func (fi *FileInfo) ModTime() time.Time { return fi.modTime }
func (fi *FileInfo) IsDir() bool        { return fi.isDir }

// WalkFunc is the type of the function called for each file or directory visited by Walk.
type WalkFunc func(path string, info FileInfo, err error) error

// Walk calling walkFn for each file or directory in queried result
func Walk(root string, maxCount int, walkFn WalkFunc) error {
	client := getCachedClient()
	if client == 0 {
		return EverythingNotRunningError
	}

	searchState := createSearchState()
	if searchState == 0 {
		return EverythingNotRunningError
	}
	defer destroySearchState(searchState)

	setSearchMatchDefaults(searchState)
	setSearchText(searchState, root)
	if maxCount > 0 {
		setSearchViewportCount(searchState, maxCount)
	}
	addSearchPropertyRequest(searchState, EVERYTHING3_PROPERTY_ID_PATH_AND_NAME)
	addSearchPropertyRequest(searchState, EVERYTHING3_PROPERTY_ID_SIZE)
	addSearchPropertyRequest(searchState, EVERYTHING3_PROPERTY_ID_DATE_MODIFIED)

	resultList := search(client, searchState)
	if resultList == 0 {
		resetCachedClient()
		client = getCachedClient()
		if client == 0 {
			return EverythingNotRunningError
		}
		resultList = search(client, searchState)
	}
	if resultList == 0 {
		return EverythingNotRunningError
	}
	defer destroyResultList(resultList)

	num := getResultListViewportCount(resultList)
	if maxCount > 0 && num > maxCount {
		num = maxCount
	}

	for i := 0; i < num; i++ {
		var fi FileInfo
		fi.name = getResultFullPathName(resultList, i)
		fi.size = getResultSize(resultList, i)
		fi.modTime = getResultDateModified(resultList, i)
		fi.isDir = isFolderResult(resultList, i)
		if err := walkFn(fi.name, fi, nil); err != nil {
			return err
		}
	}
	return nil
}

// GetVersionString print ver
func GetVersionString() (ver string) {
	client := connectEverythingClient()
	if client == 0 {
		return ""
	}
	defer destroyEverythingClient(client)

	if Everything3_GetMajorVersion == nil || Everything3_GetMinorVersion == nil || Everything3_GetRevision == nil {
		return ""
	}

	major, _, _ := Everything3_GetMajorVersion.Call(client)
	minor, _, _ := Everything3_GetMinorVersion.Call(client)
	revision, _, _ := Everything3_GetRevision.Call(client)
	ver = fmt.Sprintf("%d.%d.%d", int(major), int(minor), int(revision))
	return
}

func createSearchState() uintptr {
	if Everything3_CreateSearchState == nil {
		return 0
	}
	state, _, _ := Everything3_CreateSearchState.Call()
	return state
}

func destroySearchState(searchState uintptr) {
	if Everything3_DestroySearchState != nil && searchState != 0 {
		Everything3_DestroySearchState.Call(searchState)
	}
}

func setSearchMatchDefaults(searchState uintptr) {
	if searchState == 0 {
		return
	}
	setSearchBool(Everything3_SetSearchMatchCase, searchState, false)
	setSearchBool(Everything3_SetSearchMatchWholeWords, searchState, false)
	setSearchBool(Everything3_SetSearchMatchPath, searchState, false)
	setSearchBool(Everything3_SetSearchRegex, searchState, false)
}

func setSearchBool(proc *syscall.LazyProc, searchState uintptr, enabled bool) {
	if proc == nil || searchState == 0 {
		return
	}
	var value uintptr
	if enabled {
		value = 1
	}
	proc.Call(searchState, value)
}

func setSearchText(searchState uintptr, text string) {
	if Everything3_SetSearchTextW != nil && searchState != 0 {
		Everything3_SetSearchTextW.Call(searchState, uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(text))))
	}
}

func setSearchViewportCount(searchState uintptr, count int) {
	if Everything3_SetSearchViewportCount != nil && searchState != 0 {
		Everything3_SetSearchViewportCount.Call(searchState, uintptr(count))
	}
}

func addSearchPropertyRequest(searchState uintptr, propertyID int) {
	if Everything3_AddSearchPropertyRequest != nil && searchState != 0 {
		Everything3_AddSearchPropertyRequest.Call(searchState, uintptr(propertyID))
	}
}

func search(client uintptr, searchState uintptr) uintptr {
	if Everything3_Search == nil {
		return 0
	}
	resultList, _, _ := Everything3_Search.Call(client, searchState)
	return resultList
}

func destroyResultList(resultList uintptr) {
	if Everything3_DestroyResultList != nil && resultList != 0 {
		Everything3_DestroyResultList.Call(resultList)
	}
}

func getResultListViewportCount(resultList uintptr) int {
	if Everything3_GetResultListViewportCount == nil {
		return 0
	}
	count, _, _ := Everything3_GetResultListViewportCount.Call(resultList)
	return int(count)
}

func getResultFullPathName(resultList uintptr, index int) string {
	if Everything3_GetResultFullPathNameW == nil {
		return ""
	}
	pathbuf := make([]uint16, 32768)
	Everything3_GetResultFullPathNameW.Call(
		resultList,
		uintptr(index),
		uintptr(unsafe.Pointer(&pathbuf[0])),
		uintptr(len(pathbuf)),
	)
	return syscall.UTF16ToString(pathbuf)
}

func getResultSize(resultList uintptr, index int) int64 {
	if Everything3_GetResultSize == nil {
		return 0
	}
	size, _, _ := Everything3_GetResultSize.Call(resultList, uintptr(index))
	return int64(size)
}

func getResultDateModified(resultList uintptr, index int) time.Time {
	if Everything3_GetResultDateModified == nil {
		return time.Time{}
	}
	value, _, _ := Everything3_GetResultDateModified.Call(resultList, uintptr(index))
	ft := syscall.Filetime{
		LowDateTime:  uint32(value),
		HighDateTime: uint32(value >> 32),
	}
	return time.Unix(0, ft.Nanoseconds())
}

func isFolderResult(resultList uintptr, index int) bool {
	if Everything3_IsFolderResult == nil {
		return false
	}
	r, _, _ := Everything3_IsFolderResult.Call(resultList, uintptr(index))
	return r != 0
}
