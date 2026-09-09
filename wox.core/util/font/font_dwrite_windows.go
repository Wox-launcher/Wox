package font

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	ole32DLL                = windows.NewLazySystemDLL("ole32.dll")
	dwriteDLL               = windows.NewLazySystemDLL("dwrite.dll")
	procCoInitializeEx      = ole32DLL.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32DLL.NewProc("CoUninitialize")
	procDWriteCreateFactory = dwriteDLL.NewProc("DWriteCreateFactory")
	iidIDWriteFactory       = windows.GUID{Data1: 0xb859ee5a, Data2: 0xd838, Data3: 0x4b5b, Data4: [8]byte{0xa2, 0xe8, 0x1a, 0xdc, 0x7d, 0x93, 0xdb, 0x48}}
)

const (
	coinitMultithreaded     = 0
	dwriteFactoryTypeShared = 0
	sFalse                  = 1
	rpcEChangedMode         = 0x80010106
)

// enumerateDirectWriteFontFamilies returns every localized family name from the system font collection.
func enumerateDirectWriteFontFamilies() ([]string, error) {
	initialized, err := initializeCOM()
	if err != nil {
		return nil, err
	}
	if initialized {
		defer procCoUninitialize.Call()
	}

	var factory uintptr
	hr, _, _ := procDWriteCreateFactory.Call(
		dwriteFactoryTypeShared,
		uintptr(unsafe.Pointer(&iidIDWriteFactory)),
		uintptr(unsafe.Pointer(&factory)),
	)
	if hresultFailed(hr) || factory == 0 {
		return nil, fmt.Errorf("DWriteCreateFactory failed: 0x%08X", uint32(hr))
	}
	defer comRelease(factory)

	var collection uintptr
	// IDWriteFactory.GetSystemFontCollection is vtable index 3.
	if hr = comCall(factory, 3, uintptr(unsafe.Pointer(&collection)), 0); hresultFailed(hr) || collection == 0 {
		return nil, fmt.Errorf("GetSystemFontCollection failed: 0x%08X", uint32(hr))
	}
	defer comRelease(collection)

	count := uint32(comCall(collection, 3)) // IDWriteFontCollection.GetFontFamilyCount
	families := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		var family uintptr
		if hr = comCall(collection, 4, uintptr(index), uintptr(unsafe.Pointer(&family))); hresultFailed(hr) || family == 0 {
			continue
		}
		families = append(families, readDirectWriteFamilyNames(family)...)
		comRelease(family)
	}
	if len(families) == 0 {
		return nil, fmt.Errorf("DirectWrite returned no font families")
	}
	return families, nil
}

// initializeCOM prepares the calling thread for DirectWrite. The caller must CoUninitialize only when this returns true.
func initializeCOM() (bool, error) {
	hr, _, _ := procCoInitializeEx.Call(0, coinitMultithreaded)
	if hr == 0 {
		return true, nil
	}
	if hr == sFalse || hr == rpcEChangedMode {
		return false, nil
	}
	if hresultFailed(hr) {
		return false, fmt.Errorf("CoInitializeEx failed: 0x%08X", uint32(hr))
	}
	return false, nil
}

// readDirectWriteFamilyNames collects localized names so both "PingFang SC" and "苹方-简" are searchable.
func readDirectWriteFamilyNames(family uintptr) []string {
	var names uintptr
	if hr := comCall(family, 6, uintptr(unsafe.Pointer(&names))); hresultFailed(hr) || names == 0 { // IDWriteFontFamily.GetFamilyNames
		return nil
	}
	defer comRelease(names)

	count := uint32(comCall(names, 3))
	families := make([]string, 0, count)
	for index := uint32(0); index < count; index++ {
		name := strings.TrimSpace(localizedStringAt(names, index))
		if name == "" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "@") {
			continue
		}
		families = append(families, name)
	}
	return families
}

// localizedStringAt reads one IDWriteLocalizedStrings entry as a Go string.
func localizedStringAt(names uintptr, index uint32) string {
	var length uint32
	if hr := comCall(names, 7, uintptr(index), uintptr(unsafe.Pointer(&length))); hresultFailed(hr) {
		return ""
	}
	buffer := make([]uint16, length+1)
	if hr := comCall(names, 8, uintptr(index), uintptr(unsafe.Pointer(&buffer[0])), uintptr(length+1)); hresultFailed(hr) {
		return ""
	}
	return windows.UTF16ToString(buffer)
}

func comCall(object uintptr, index int, args ...uintptr) uintptr {
	vtable := *(*uintptr)(unsafe.Pointer(object))
	method := *(*uintptr)(unsafe.Pointer(vtable + uintptr(index)*unsafe.Sizeof(uintptr(0))))
	arguments := make([]uintptr, 0, 1+len(args))
	arguments = append(arguments, object)
	arguments = append(arguments, args...)
	result, _, _ := syscall.SyscallN(method, arguments...)
	return result
}

func comRelease(object uintptr) {
	if object != 0 {
		comCall(object, 2)
	}
}

func hresultFailed(hr uintptr) bool {
	return int32(hr) < 0
}
