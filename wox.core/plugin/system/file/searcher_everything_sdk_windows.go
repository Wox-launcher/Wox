// Search Everything : http://voidtools.com/
// https://www.voidtools.com/support/everything/sdk/
// https://github.com/jof4002/Everything
package file

import (
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

const (
	EVERYTHING_OK                     = 0 // no error detected
	EVERYTHING_ERROR_MEMORY           = 1 // out of memory.
	EVERYTHING_ERROR_IPC              = 2 // Everything search client is not running
	EVERYTHING_ERROR_REGISTERCLASSEX  = 3 // unable to register window class.
	EVERYTHING_ERROR_CREATEWINDOW     = 4 // unable to create listening window
	EVERYTHING_ERROR_CREATETHREAD     = 5 // unable to create listening thread
	EVERYTHING_ERROR_INVALIDINDEX     = 6 // invalid index
	EVERYTHING_ERROR_INVALIDCALL      = 7 // invalid call
	EVERYTHING_ERROR_INVALIDREQUEST   = 8 // invalid request data, request data first.
	EVERYTHING_ERROR_INVALIDPARAMETER = 9 // bad parameter.
)

const (
	EVERYTHING_SORT_NAME_ASCENDING                   = 1
	EVERYTHING_SORT_NAME_DESCENDING                  = 2
	EVERYTHING_SORT_PATH_ASCENDING                   = 3
	EVERYTHING_SORT_PATH_DESCENDING                  = 4
	EVERYTHING_SORT_SIZE_ASCENDING                   = 5
	EVERYTHING_SORT_SIZE_DESCENDING                  = 6
	EVERYTHING_SORT_EXTENSION_ASCENDING              = 7
	EVERYTHING_SORT_EXTENSION_DESCENDING             = 8
	EVERYTHING_SORT_TYPE_NAME_ASCENDING              = 9
	EVERYTHING_SORT_TYPE_NAME_DESCENDING             = 10
	EVERYTHING_SORT_DATE_CREATED_ASCENDING           = 11
	EVERYTHING_SORT_DATE_CREATED_DESCENDING          = 12
	EVERYTHING_SORT_DATE_MODIFIED_ASCENDING          = 13
	EVERYTHING_SORT_DATE_MODIFIED_DESCENDING         = 14
	EVERYTHING_SORT_ATTRIBUTES_ASCENDING             = 15
	EVERYTHING_SORT_ATTRIBUTES_DESCENDING            = 16
	EVERYTHING_SORT_FILE_LIST_FILENAME_ASCENDING     = 17
	EVERYTHING_SORT_FILE_LIST_FILENAME_DESCENDING    = 18
	EVERYTHING_SORT_RUN_COUNT_ASCENDING              = 19
	EVERYTHING_SORT_RUN_COUNT_DESCENDING             = 20
	EVERYTHING_SORT_DATE_RECENTLY_CHANGED_ASCENDING  = 21
	EVERYTHING_SORT_DATE_RECENTLY_CHANGED_DESCENDING = 22
	EVERYTHING_SORT_DATE_ACCESSED_ASCENDING          = 23
	EVERYTHING_SORT_DATE_ACCESSED_DESCENDING         = 24
	EVERYTHING_SORT_DATE_RUN_ASCENDING               = 25
	EVERYTHING_SORT_DATE_RUN_DESCENDING              = 26
)

const (
	EVERYTHING_REQUEST_FILE_NAME                           = 0x00000001
	EVERYTHING_REQUEST_PATH                                = 0x00000002
	EVERYTHING_REQUEST_FULL_PATH_AND_FILE_NAME             = 0x00000004
	EVERYTHING_REQUEST_EXTENSION                           = 0x00000008
	EVERYTHING_REQUEST_SIZE                                = 0x00000010
	EVERYTHING_REQUEST_DATE_CREATED                        = 0x00000020
	EVERYTHING_REQUEST_DATE_MODIFIED                       = 0x00000040
	EVERYTHING_REQUEST_DATE_ACCESSED                       = 0x00000080
	EVERYTHING_REQUEST_ATTRIBUTES                          = 0x00000100
	EVERYTHING_REQUEST_FILE_LIST_FILE_NAME                 = 0x00000200
	EVERYTHING_REQUEST_RUN_COUNT                           = 0x00000400
	EVERYTHING_REQUEST_DATE_RUN                            = 0x00000800
	EVERYTHING_REQUEST_DATE_RECENTLY_CHANGED               = 0x00001000
	EVERYTHING_REQUEST_HIGHLIGHTED_FILE_NAME               = 0x00002000
	EVERYTHING_REQUEST_HIGHLIGHTED_PATH                    = 0x00004000
	EVERYTHING_REQUEST_HIGHLIGHTED_FULL_PATH_AND_FILE_NAME = 0x00008000
)

// manipulate search state
var Everything_SetSearch *syscall.LazyProc
var Everything_SetMatchPath *syscall.LazyProc
var Everything_SetMatchCase *syscall.LazyProc
var Everything_SetMatchWholeWord *syscall.LazyProc
var Everything_SetRegex *syscall.LazyProc
var Everything_SetMax *syscall.LazyProc
var Everything_SetOffset *syscall.LazyProc
var Everything_SetReplyWindow *syscall.LazyProc
var Everything_SetReplyID *syscall.LazyProc
var Everything_SetSort *syscall.LazyProc
var Everything_SetRequestFlags *syscall.LazyProc

// read search state
var Everything_GetSearch *syscall.LazyProc
var Everything_GetMatchPath *syscall.LazyProc
var Everything_GetMatchCase *syscall.LazyProc
var Everything_GetMatchWholeWord *syscall.LazyProc
var Everything_GetRegex *syscall.LazyProc
var Everything_GetMax *syscall.LazyProc
var Everything_GetOffset *syscall.LazyProc
var Everything_GetReplyWindow *syscall.LazyProc
var Everything_GetReplyID *syscall.LazyProc
var Everything_GetLastError *syscall.LazyProc
var Everything_GetSort *syscall.LazyProc
var Everything_GetRequestFlags *syscall.LazyProc

// execute query
var Everything_Query *syscall.LazyProc

// query reply
var Everything_IsQueryReply *syscall.LazyProc

// write result state

// read result state
var Everything_GetNumFileResults *syscall.LazyProc
var Everything_GetNumFolderResults *syscall.LazyProc
var Everything_GetNumResults *syscall.LazyProc
var Everything_GetTotFileResults *syscall.LazyProc
var Everything_GetTotFolderResults *syscall.LazyProc
var Everything_GetTotResults *syscall.LazyProc

var Everything_IsFolderResult *syscall.LazyProc
var Everything_IsFileResult *syscall.LazyProc
var Everything_GetResultFileName *syscall.LazyProc
var Everything_GetResultPath *syscall.LazyProc
var Everything_GetResultFullPathName *syscall.LazyProc
var Everything_GetResultListSort *syscall.LazyProc
var Everything_GetResultListRequestedFlags *syscall.LazyProc
var Everything_GetResultExstension *syscall.LazyProc
var Everything_GetResultSize *syscall.LazyProc
var Everything_GetResultDateModified *syscall.LazyProc
var Everything_GetResultDateAccessed *syscall.LazyProc
var Everything_GetResultAttributes *syscall.LazyProc

// reset state and free any allocated memory
var Everything_Reset *syscall.LazyProc
var Everything_CleanUp *syscall.LazyProc
var Everything_IsDBLoaded *syscall.LazyProc

func initEverythingDLL(dllPath string) {
	mod := syscall.NewLazyDLL(dllPath)
	if mod != nil {
		// Search State
		Everything_SetSearch = mod.NewProc("Everything_SetSearchW")
		Everything_SetMatchPath = mod.NewProc("Everything_SetMatchPath")
		Everything_SetMatchCase = mod.NewProc("Everything_SetMatchCase")
		Everything_SetMatchWholeWord = mod.NewProc("Everything_SetMatchWholeWord")
		Everything_SetRegex = mod.NewProc("Everything_SetRegex")
		Everything_SetMax = mod.NewProc("Everything_SetMax")
		Everything_SetOffset = mod.NewProc("Everything_SetOffset")
		Everything_SetReplyWindow = mod.NewProc("Everything_SetReplyWindow")
		Everything_SetReplyID = mod.NewProc("Everything_SetReplyID")
		Everything_SetSort = mod.NewProc("Everything_SetSort")
		Everything_SetRequestFlags = mod.NewProc("Everything_SetRequestFlags")
		// Read Search State
		Everything_GetSearch = mod.NewProc("Everything_GetSearchW")
		Everything_GetMatchPath = mod.NewProc("Everything_GetMatchPath")
		Everything_GetMatchCase = mod.NewProc("Everything_GetMatchCase")
		Everything_GetMatchWholeWord = mod.NewProc("Everything_GetMatchWholeWord")
		Everything_GetRegex = mod.NewProc("Everything_GetRegex")
		Everything_GetMax = mod.NewProc("Everything_GetMax")
		Everything_GetOffset = mod.NewProc("Everything_GetOffset")
		Everything_GetReplyWindow = mod.NewProc("Everything_GetReplyWindow")
		Everything_GetReplyID = mod.NewProc("Everything_GetReplyID")
		Everything_GetLastError = mod.NewProc("Everything_GetLastError")
		Everything_GetRequestFlags = mod.NewProc("Everything_GetRequestFlags")
		// Query
		Everything_Query = mod.NewProc("Everything_QueryW")
		// Query Reply
		Everything_IsQueryReply = mod.NewProc("Everything_QueryW")
		// Reading results
		Everything_GetNumFileResults = mod.NewProc("Everything_GetNumFileResults")
		Everything_GetNumFolderResults = mod.NewProc("Everything_GetNumFolderResults")
		Everything_GetNumResults = mod.NewProc("Everything_GetNumResults")
		Everything_GetTotFileResults = mod.NewProc("Everything_GetTotFileResults")
		Everything_GetTotFolderResults = mod.NewProc("Everything_GetTotFolderResults")
		Everything_GetTotResults = mod.NewProc("Everything_GetTotResults")

		Everything_IsFolderResult = mod.NewProc("Everything_IsFolderResult")
		Everything_IsFileResult = mod.NewProc("Everything_IsFileResult")
		Everything_GetResultFileName = mod.NewProc("Everything_GetResultFileName")
		Everything_GetResultPath = mod.NewProc("Everything_GetResultPath")
		Everything_GetResultFullPathName = mod.NewProc("Everything_GetResultFullPathNameW")
		Everything_GetResultListSort = mod.NewProc("Everything_GetResultListSort")
		Everything_GetResultListRequestedFlags = mod.NewProc("Everything_GetResultListRequestedFlags")
		Everything_GetResultExstension = mod.NewProc("Everything_GetResultExstension")
		Everything_GetResultSize = mod.NewProc("Everything_GetResultSize")
		Everything_GetResultDateModified = mod.NewProc("Everything_GetResultDateModified")
		Everything_GetResultDateAccessed = mod.NewProc("Everything_GetResultDateAccessed")
		Everything_GetResultAttributes = mod.NewProc("Everything_GetResultDateAttributes")

		// Reset
		Everything_Reset = mod.NewProc("Everything_Reset")
		Everything_CleanUp = mod.NewProc("Everything_CleanUP")
		Everything_IsDBLoaded = mod.NewProc("Everything_IsDBLoaded")
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

// Walk calling walkFn for each file or directory in queried resulr
func Walk(root string, maxCount int, walkFn WalkFunc) error {
	SetSearch(root)
	SetRequestFlags(EVERYTHING_REQUEST_FILE_NAME | EVERYTHING_REQUEST_PATH | EVERYTHING_REQUEST_SIZE | EVERYTHING_REQUEST_DATE_MODIFIED)
	Query(true)
	num := GetNumResults()
	if maxCount > 0 && num > maxCount {
		num = maxCount
	}
	for i := 0; i < num; i++ {
		var fi FileInfo
		fi.name = GetResultFullPathName(i)
		fi.size = GetResultSize(i)
		fi.modTime = GetResultDateModified(i)
		fi.isDir = IsFolderResult(i)
		err := walkFn(fi.name, fi, nil)
		if err != nil {
			return err
		}
	}
	return nil

}

// GetVersionString print ver
func GetVersionString() (ver string) {
	mod := syscall.NewLazyDLL("Everything64.dll")
	if mod != nil {
		fmajor := mod.NewProc("Everything_GetMajorVersion")
		fminor := mod.NewProc("Everything_GetMinorVersion")
		frevision := mod.NewProc("Everything_GetRevision")
		p1, _, _ := fmajor.Call()
		p2, _, _ := fminor.Call()
		p3, _, _ := frevision.Call()
		ver = fmt.Sprintf("%d.%d.%d", int(p1), int(p2), int(p3))
	}
	return
}

// SetSearch void Everything_SetSearchW(LPCWSTR lpString);
func SetSearch(str string) {
	if Everything_SetSearch != nil {
		Everything_SetSearch.Call(uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(str))))
	}
}

// SetMatchPath void Everything_SetMatchPath(BOOL bEnable);
func SetMatchPath(bEnable bool) {
	if Everything_SetMatchPath != nil {
		var param int
		if bEnable {
			param = 1
		}
		Everything_SetMatchPath.Call(uintptr(param))
	}
}

// SetMatchCase void Everything_SetMatchCase(BOOL bEnable);
func SetMatchCase(bEnable bool) {
	if Everything_SetMatchCase != nil {
		var param int
		if bEnable {
			param = 1
		}
		Everything_SetMatchCase.Call(uintptr(param))
	}
}

// SetRegex void Everything_SetRegex(BOOL bEnable);
func SetRegex(bEnable bool) {
	if Everything_SetRegex != nil {
		var param int
		if bEnable {
			param = 1
		}
		Everything_SetRegex.Call(uintptr(param))
	}
}

// SetSort void Everything_SetSort(DWORD dwSort); // Everything 1.4.1
func SetSort(sortMode int) {
	if Everything_SetSort != nil {
		Everything_SetSort.Call(uintptr(sortMode))
	}
}

// SetRequestFlags void Everything_SetRequestFlags(DWORD dwRequestFlags); // Everything 1.4.1
func SetRequestFlags(flags int) {
	if Everything_SetRequestFlags != nil {
		Everything_SetRequestFlags.Call(uintptr(flags))
	}
}

// GetSort DWORD Everything_GetSort(void); // Everything 1.4.1
func GetSort() (ret int) {
	if Everything_GetSort != nil {
		r, _, _ := Everything_GetSort.Call()
		ret = int(r)
	}
	return
}

// Query BOOL Everything_QueryW(BOOL bWait);
func Query(bWait bool) (ret bool) {
	if Everything_Query != nil {
		var param int
		if bWait {
			param = 1
		}
		r, _, _ := Everything_Query.Call(uintptr(param))
		ret = r != 0
	}
	return
}

// GetNumResults DWORD Everything_GetNumResults(void);
func GetNumResults() (ret int) {
	if Everything_GetNumResults != nil {
		r, _, _ := Everything_GetNumResults.Call()
		ret = int(r)
	}
	return
}

// GetResultFullPathName DWORD Everything_GetResultFullPathNameW(DWORD dwIndex,LPWSTR wbuf,DWORD wbuf_size_in_wchars);
func GetResultFullPathName(index int) (path string) {
	if Everything_GetResultFullPathName != nil {
		var pathbuf = make([]uint16, 1024)
		Everything_GetResultFullPathName.Call(uintptr(index), uintptr(unsafe.Pointer(&pathbuf[0])), 1023) // bufsize-1
		path = syscall.UTF16ToString(pathbuf)
	}
	return
}

// IsFolderResult BOOL Everything_IsFolderResult(DWORD dwIndex);
func IsFolderResult(index int) (ret bool) {
	if Everything_IsFolderResult != nil {
		r, _, _ := Everything_IsFolderResult.Call(uintptr(index))
		ret = r != 0
	}
	return
}

// IsFileResult BOOL Everything_IsFileResult(DWORD dwIndex);
func IsFileResult(index int) (ret bool) {
	if Everything_IsFileResult != nil {
		r, _, _ := Everything_IsFileResult.Call(uintptr(index))
		ret = r != 0
	}
	return
}

func GetSearch() (query string) {
	if Everything_GetSearch != nil {
		r, _, _ := Everything_GetSearch.Call()
		query = *(*string)(unsafe.Pointer(&r))
	}
	return
}

func GetMatchPath() (enabled int32) {
	if Everything_GetMatchPath != nil {
		r, _, _ := Everything_GetMatchPath.Call()
		enabled = int32(r)
	}
	return
}

func GetMatchCase() (enabled int32) {
	if Everything_GetMatchCase != nil {
		r, _, _ := Everything_GetMatchCase.Call()
		enabled = int32(r)
	}
	return
}

func GetMatchWholeWord() (enabled bool) {
	if Everything_GetMatchWholeWord != nil {
		r, _, _ := Everything_GetMatchWholeWord.Call()
		enabled = r != 0
	}
	return
}

func GetRegex() (enabled bool) {
	if Everything_GetRegex != nil {
		r, _, _ := Everything_GetRegex.Call()
		enabled = r != 0
	}
	return
}

// GetResultListSort DWORD Everything_GetResultListSort(void); // Everything 1.4.1
func GetResultListSort() (mode int) {
	if Everything_GetResultListSort != nil {
		r, _, _ := Everything_GetResultListSort.Call()
		mode = int(r)
	}
	return
}

// GetResultSize BOOL Everything_GetResultSize(DWORD dwIndex,LARGE_INTEGER *lpSize); // Everything 1.4.1
func GetResultSize(index int) (size int64) {
	if Everything_GetResultSize != nil {
		Everything_GetResultSize.Call(uintptr(index), uintptr(unsafe.Pointer(&size)))
	}
	return
}

// GetResultDateModified BOOL Everything_GetResultDateModified(DWORD dwIndex,FILETIME *lpDateModified); // Everything 1.4.1
func GetResultDateModified(index int) (t time.Time) {
	if Everything_GetResultDateModified != nil {
		var ft syscall.Filetime
		Everything_GetResultDateModified.Call(uintptr(index), uintptr(unsafe.Pointer(&ft)))
		t = time.Unix(0, ft.Nanoseconds())
	}
	return
}

// Reset void Everything_Reset(void);
func Reset() {
	if Everything_Reset != nil {
		Everything_Reset.Call()
	}
}
