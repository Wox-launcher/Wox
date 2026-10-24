package shell

import (
	"bytes"
	"debug/pe"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/go-ole/go-ole"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// DesktopLaunchKey identifies equivalent default launches without executing them. Unsupported links
// have no key: indexers must retain their original entries rather than guess from GetPath alone.
func DesktopLaunchKey(path string) string {
	req := shellExecuteRequest{File: path, Verb: "open", Show: shellExecuteShowNormal}
	if strings.EqualFold(filepath.Ext(path), ".lnk") {
		var ok bool
		req, ok = shortcutLaunchRequest(path, "open")
		if !ok {
			return ""
		}
	} else if !strings.EqualFold(filepath.Ext(path), ".exe") {
		return ""
	}
	absPath, err := filepath.Abs(req.File)
	if err != nil {
		return ""
	}
	req.File = strings.ToLower(filepath.Clean(absPath))
	if req.Directory != "" {
		req.Directory = strings.ToLower(filepath.Clean(req.Directory))
	}
	// Structured encoding avoids delimiter collisions; arguments and empty directories stay distinct.
	key, _ := json.Marshal(req)
	return string(key)
}

// shortcutLaunchRequest bypasses Shell link activation only when all launch-affecting data is understood.
// Anything unsupported stays with Windows, including console apps whose title/icon can depend on the .lnk itself.
//
// A local QQMusic comparison on 2026-09-28 measured median time from launch request to the existing
// app window becoming visible and foreground, not merely ShellExecuteEx returning (milliseconds):
//
//	Launch method                 Same harness process (n=6)  Fresh harness process (n=3)
//	.lnk + ASYNCOK                427                         807
//	STA + .lnk + NOASYNC          441                         760
//	Read .lnk, then exe + ASYNCOK  286                         358
//	Direct exe + ASYNCOK          276                         352
//
// Shortcut reading is included in the timing. QQMusic stayed running throughout; "fresh" refers
// only to the independent Go test harness, not an app cold start. All 48 measured launches succeeded.
// Reading the link reduced foreground latency by 33-56%, while STA + NOASYNC did not remove the gap.
// This supports bypassing link activation for supported links, but does not establish a COM timeout cause.
func shortcutLaunchRequest(path, verb string) (shellExecuteRequest, bool) {
	req, err := readShortcutLaunchRequest(path, verb, shortcutDefaultActivation)
	return req, err == nil
}

// readShortcutLaunchRequest permits presentation-only app identity for explicit executable launches.
// Normal Open and deduplication remain strict because they must preserve Shell activation identity.
func readShortcutLaunchRequest(path, verb string, purpose shortcutLaunchPurpose) (shellExecuteRequest, error) {
	fallback := shellExecuteRequest{File: path, Verb: verb, Show: shellExecuteShowNormal}
	if !strings.EqualFold(filepath.Ext(path), ".lnk") || (verb != "open" && verb != "runas") {
		return fallback, fmt.Errorf("unsupported shortcut extension or launch verb")
	}
	absPath, err := filepath.Abs(path)
	if err != nil || !shortcutLocalPath(absPath) {
		return fallback, fmt.Errorf("shortcut path is not an absolute local path")
	}
	pathPtr, err := windows.UTF16PtrFromString(absPath)
	if err != nil {
		return fallback, fmt.Errorf("shortcut path cannot be encoded as UTF-16")
	}
	// Keep the inspected file stable until IPersistFile.Load has read it too.
	handle, err := windows.CreateFile(pathPtr, windows.GENERIC_READ, windows.FILE_SHARE_READ, nil, windows.OPEN_EXISTING, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return fallback, fmt.Errorf("cannot open shortcut: %w", err)
	}
	file := os.NewFile(uintptr(handle), absPath)
	defer file.Close()
	// Oversized or unrecognized links are handled by Shell, not partially decoded here.
	data, err := io.ReadAll(io.LimitReader(file, shortcutMaxFileSize+1))
	if err != nil {
		return fallback, fmt.Errorf("read shortcut: %w", err)
	}
	if len(data) > shortcutMaxFileSize {
		return fallback, fmt.Errorf("shortcut exceeds %d bytes", shortcutMaxFileSize)
	}
	if err := validateShortcutData(data, purpose); err != nil {
		return fallback, err
	}

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	cleanupCOM, err := initializeCOMForShell()
	if err != nil {
		return fallback, fmt.Errorf("cannot initialize Shell COM: %w", err)
	}
	defer cleanupCOM()
	link, err := ole.CreateInstance(ole.NewGUID("{00021401-0000-0000-C000-000000000046}"), ole.NewGUID("{000214F9-0000-0000-C000-000000000046}"))
	if err != nil {
		return fallback, fmt.Errorf("cannot create IShellLink: %w", err)
	}
	defer link.Release()
	var persist *ole.IUnknown
	if link.PutQueryInterface(ole.NewGUID("{0000010b-0000-0000-C000-000000000046}"), &persist) != nil {
		return fallback, fmt.Errorf("cannot query IPersistFile")
	}
	defer persist.Release()
	if !shortcutCOMCall(persist, 5, uintptr(unsafe.Pointer(pathPtr)), 0) { // IPersistFile.Load(STGM_READ)
		return fallback, fmt.Errorf("IPersistFile.Load failed")
	}

	// Do not Resolve: missing/moved targets must retain Shell's tracking and repair behavior.
	targetBuffer := make([]uint16, windows.MAX_PATH)
	if !shortcutCOMCall(link, 3, uintptr(unsafe.Pointer(&targetBuffer[0])), uintptr(len(targetBuffer)), 0, 4) { // GetPath(SLGP_RAWPATH)
		return fallback, fmt.Errorf("IShellLink.GetPath failed")
	}
	target, ok := shortcutString(targetBuffer)
	if !ok {
		return fallback, fmt.Errorf("target path is truncated or invalid UTF-16")
	}
	// StringData has a 16-bit character count; include room for its terminating NUL.
	argsBuffer, directoryBuffer := make([]uint16, 1<<16), make([]uint16, 1<<16)
	if !shortcutCOMCall(link, 10, uintptr(unsafe.Pointer(&argsBuffer[0])), uintptr(len(argsBuffer))) ||
		!shortcutCOMCall(link, 8, uintptr(unsafe.Pointer(&directoryBuffer[0])), uintptr(len(directoryBuffer))) {
		return fallback, fmt.Errorf("IShellLink.GetArguments or GetWorkingDirectory failed")
	}
	args, argsOK := shortcutString(argsBuffer)
	directory, directoryOK := shortcutString(directoryBuffer)
	if !argsOK || !directoryOK {
		return fallback, fmt.Errorf("arguments or working directory are truncated or invalid UTF-16")
	}
	target, err = registry.ExpandString(target)
	if err != nil || !shortcutLocalPath(target) || !strings.EqualFold(filepath.Ext(target), ".exe") {
		return fallback, fmt.Errorf("target is not a local executable or contains invalid environment variables")
	}
	if directory != "" {
		directory, err = registry.ExpandString(directory)
		if err != nil || !shortcutLocalPath(directory) {
			return fallback, fmt.Errorf("working directory is not local or contains invalid environment variables")
		}
		info, err := os.Stat(directory)
		if err != nil || !info.IsDir() {
			return fallback, fmt.Errorf("working directory does not exist or is not a directory")
		}
	}
	image, err := pe.Open(target)
	if err != nil {
		return fallback, fmt.Errorf("cannot read target PE image: %w", err)
	}
	defer image.Close()
	var subsystem uint16
	switch header := image.OptionalHeader.(type) {
	case *pe.OptionalHeader32:
		subsystem = header.Subsystem
	case *pe.OptionalHeader64:
		subsystem = header.Subsystem
	}
	if subsystem != pe.IMAGE_SUBSYSTEM_WINDOWS_GUI || image.Characteristics&pe.IMAGE_FILE_DLL != 0 {
		return fallback, fmt.Errorf("target is not a GUI executable or is a DLL")
	}
	if binary.LittleEndian.Uint32(data[shortcutFlagsOffset:])&shortcutRunAsUser != 0 {
		verb = "runas"
	}
	return shellExecuteRequest{File: target, Verb: verb, Parameters: args, Directory: directory, Show: int32(binary.LittleEndian.Uint32(data[shortcutShowOffset:]))}, nil
}

// shortcutLocalPath excludes namespaces, relative paths, UNC paths, and mapped network drives before filesystem access.
func shortcutLocalPath(path string) bool {
	if len(path) < 3 || path[1] != ':' || (path[2] != '\\' && path[2] != '/') || !filepath.IsAbs(path) || strings.ContainsRune(path, 0) {
		return false
	}
	return windows.GetDriveType(windows.StringToUTF16Ptr(path[:3])) == windows.DRIVE_FIXED
}

// shortcutCOMCall requires S_OK; even a partial result must take the original .lnk path.
//
//go:uintptrescapes
func shortcutCOMCall(object *ole.IUnknown, slot int, args ...uintptr) bool {
	vtable := (*[32]uintptr)(unsafe.Pointer(object.RawVTable))
	hr, _, _ := syscall.SyscallN(vtable[slot], append([]uintptr{uintptr(unsafe.Pointer(object))}, args...)...)
	return hr == 0
}

// shortcutString rejects possible truncation and UTF-16 that Go cannot round-trip unchanged.
func shortcutString(buffer []uint16) (string, bool) {
	end := slices.Index(buffer, uint16(0))
	if end < 0 || end >= len(buffer)-1 {
		return "", false
	}
	value := string(utf16.Decode(buffer[:end]))
	return value, slices.Equal(utf16.Encode([]rune(value)), buffer[:end])
}

// shortcutDataSupported scans MS-SHLLINK framing, not target resolution. A block allowlist is necessary:
// checking LinkFlags alone misses console settings, app identity, and unknown future extensions.
func shortcutDataSupported(data []byte) bool {
	return validateShortcutData(data, shortcutDefaultActivation) == nil
}

// validateShortcutData validates the complete link before any native metadata is used.
func validateShortcutData(data []byte, purpose shortcutLaunchPurpose) error {
	if len(data) < shortcutHeaderSize+shortcutDWORDSize || binary.LittleEndian.Uint32(data) != shortcutHeaderSize ||
		!bytes.Equal(data[shortcutCLSIDOffset:shortcutFlagsOffset], shortcutShellLinkCLSID) {
		return fmt.Errorf("invalid or truncated Shell Link header")
	}
	flags := binary.LittleEndian.Uint32(data[shortcutFlagsOffset:])
	// Basic fields, environment paths/icons, runas, and tracking/cache flags only.
	if unsupported := flags & ^uint32(shortcutAllowedFlags); unsupported != 0 {
		return fmt.Errorf("unsupported LinkFlags: %#x", unsupported)
	}
	if !bytes.Equal(data[shortcutReservedOffset:shortcutHeaderSize], make([]byte, shortcutReservedSize)) {
		return fmt.Errorf("nonzero reserved header bytes")
	}
	show := binary.LittleEndian.Uint32(data[shortcutShowOffset:])
	if show != shortcutShowNormal && show != shortcutShowMaximized && show != shortcutShowMinNoActive {
		return fmt.Errorf("unsupported ShowCommand: %d", show)
	}
	rest := data[shortcutHeaderSize:]
	if flags&shortcutHasTargetIDList != 0 {
		if len(rest) < shortcutWordSize {
			return fmt.Errorf("truncated target ID list size")
		}
		size := int(binary.LittleEndian.Uint16(rest))
		if size < shortcutWordSize || size > len(rest)-shortcutWordSize {
			return fmt.Errorf("invalid target ID list size")
		}
		rest = rest[shortcutWordSize+size:]
	}
	if flags&shortcutHasLinkInfo != 0 {
		if len(rest) < shortcutDWORDSize {
			return fmt.Errorf("truncated LinkInfo size")
		}
		size := uint64(binary.LittleEndian.Uint32(rest))
		if size < shortcutLinkInfoMinSize || size > uint64(len(rest)) {
			return fmt.Errorf("invalid LinkInfo size")
		}
		rest = rest[size:]
	}
	for bit := uint32(shortcutHasName); bit <= shortcutHasIconLocation; bit <<= 1 {
		if flags&bit == 0 {
			continue
		}
		if len(rest) < shortcutWordSize {
			return fmt.Errorf("truncated StringData length")
		}
		size := int(binary.LittleEndian.Uint16(rest))
		if flags&shortcutIsUnicode != 0 {
			size *= shortcutWordSize
		}
		if size > len(rest)-shortcutWordSize {
			return fmt.Errorf("truncated StringData payload")
		}
		value := rest[shortcutWordSize : shortcutWordSize+size]
		// Embedded NULs would silently discard a suffix when read through IShellLink.
		for i := 0; i < len(value); i++ {
			if flags&shortcutIsUnicode != 0 {
				if binary.LittleEndian.Uint16(value[i:]) == 0 {
					return fmt.Errorf("embedded NUL in Unicode StringData")
				}
				i++
			} else if value[i] == 0 {
				return fmt.Errorf("embedded NUL in ANSI StringData")
			}
		}
		rest = rest[shortcutWordSize+size:]
	}
	seenBlocks := make(map[uint32]bool)
	for len(rest) >= shortcutDWORDSize {
		size := uint64(binary.LittleEndian.Uint32(rest))
		if size == 0 {
			if len(rest) != shortcutDWORDSize {
				return fmt.Errorf("trailing data after ExtraData terminator")
			}
			if (flags&shortcutHasEnvironment != 0) != seenBlocks[shortcutEnvironmentBlock] ||
				(flags&shortcutHasEnvironmentIcon != 0) != seenBlocks[shortcutEnvironmentIconBlock] ||
				(flags&shortcutPreferEnvironmentPath != 0 && flags&shortcutHasEnvironment == 0) {
				return fmt.Errorf("environment flags do not match ExtraData blocks")
			}
			return nil
		}
		if size < shortcutExtraHeaderSize || size > uint64(len(rest)) {
			return fmt.Errorf("invalid ExtraData block size")
		}
		block := rest[:size]
		signature := binary.LittleEndian.Uint32(block[shortcutExtraSignatureOffset:])
		switch signature {
		case shortcutEnvironmentBlock, shortcutEnvironmentIconBlock: // Environment path/icon, interpreted by IShellLink.Load/GetPath.
			if size != shortcutEnvironmentBlockSize {
				return fmt.Errorf("invalid environment block size")
			}
		case shortcutTrackerBlock: // Distributed link tracking; only an existing target may use the fast path.
			if size != shortcutTrackerBlockSize {
				return fmt.Errorf("invalid tracker block size")
			}
		case shortcutSpecialFolderBlock: // Special-folder translation is performed when Shell loads the link.
			if size != shortcutSpecialFolderBlockSize {
				return fmt.Errorf("invalid special-folder block size")
			}
		case shortcutKnownFolderBlock: // Known-folder translation.
			if size != shortcutKnownFolderBlockSize {
				return fmt.Errorf("invalid known-folder block size")
			}
		case shortcutPropertyStoreBlock:
			if err := validateShortcutMetadata(block[shortcutExtraHeaderSize:], purpose); err != nil {
				return fmt.Errorf("property store: %w", err)
			}
		default:
			return fmt.Errorf("unsupported ExtraData signature: %#x", signature)
		}
		if seenBlocks[signature] {
			return fmt.Errorf("duplicate ExtraData signature: %#x", signature)
		}
		seenBlocks[signature] = true
		rest = rest[size:]
	}
	return fmt.Errorf("missing ExtraData terminator")
}

// shortcutMetadataSupported permits only the creator SID and volume ID found in ordinary saved links.
// AppUserModel properties and every other property stay with Shell, even if GetPath returns an exe.
func shortcutMetadataSupported(data []byte) bool {
	return validateShortcutMetadata(data, shortcutDefaultActivation) == nil
}

// validateShortcutMetadata also accepts app identity and toast registration for explicit launches.
// Relaunch commands and unknown properties still require the original Shell path.
func validateShortcutMetadata(data []byte, purpose shortcutLaunchPurpose) error {
	for len(data) >= shortcutDWORDSize {
		size := uint64(binary.LittleEndian.Uint32(data))
		if size == 0 {
			if len(data) != shortcutDWORDSize {
				return fmt.Errorf("trailing data after property-store terminator")
			}
			return nil
		}
		if size < shortcutStoreHeaderSize+shortcutDWORDSize || size > uint64(len(data)) || binary.LittleEndian.Uint32(data[shortcutStoreVersionOffset:]) != shortcutStoreVersion {
			return fmt.Errorf("invalid serialized property-store size or version")
		}
		format := data[shortcutStoreFormatOffset:shortcutStoreHeaderSize]
		values := data[shortcutStoreHeaderSize:size]
		for {
			if len(values) < shortcutDWORDSize {
				return fmt.Errorf("truncated property value size")
			}
			valueSize := uint64(binary.LittleEndian.Uint32(values))
			if valueSize == 0 {
				if len(values) != shortcutDWORDSize {
					return fmt.Errorf("trailing data after property-value terminator")
				}
				break
			}
			if valueSize < shortcutPropertyHeaderSize || valueSize > uint64(len(values)) || values[shortcutPropertyReservedOffset] != 0 || values[shortcutPropertyPaddingOffset] != 0 || values[shortcutPropertyPaddingOffset+1] != 0 {
				return fmt.Errorf("invalid property value size or reserved bytes")
			}
			id, kind := binary.LittleEndian.Uint32(values[shortcutPropertyIDOffset:]), binary.LittleEndian.Uint16(values[shortcutPropertyTypeOffset:])
			creatorSID := bytes.Equal(format, shortcutCreatorSIDFormat) && id == shortcutCreatorSIDProperty && kind == shortcutVariantString
			volumeID := bytes.Equal(format, shortcutVolumeIDFormat) && id == shortcutVolumeIDProperty && kind == shortcutVariantGUID
			appIdentity := purpose == shortcutExplicitExecutable && bytes.Equal(format, shortcutAppUserModelFormat)
			appID := appIdentity && id == shortcutAppIDProperty && kind == shortcutVariantString
			dualMode := appIdentity && id == shortcutDualModeProperty && kind == shortcutVariantBool
			// Chrome registers a toast callback on its link; it does not affect direct executable launches.
			toastActivator := appIdentity && id == shortcutToastActivatorProperty && kind == shortcutVariantGUID
			if !creatorSID && !volumeID && !appID && !dualMode && !toastActivator {
				return fmt.Errorf("unsupported property format=%x id=%d type=%d purpose=%s", format, id, kind, purpose)
			}
			if dualMode {
				if valueSize < shortcutPropertyBoolSize || valueSize > shortcutPropertyPaddedBoolSize || (binary.LittleEndian.Uint16(values[shortcutPropertyHeaderSize:]) != 0 && binary.LittleEndian.Uint16(values[shortcutPropertyHeaderSize:]) != shortcutVariantTrue) {
					return fmt.Errorf("invalid IsDualMode boolean size or value")
				}
				for _, padding := range values[shortcutPropertyBoolSize:valueSize] {
					if padding != 0 {
						return fmt.Errorf("nonzero IsDualMode padding")
					}
				}
			}
			if (volumeID || toastActivator) && valueSize != shortcutPropertyGUIDSize {
				return fmt.Errorf("invalid GUID property size: %d", valueSize)
			}
			if creatorSID || appID {
				if valueSize < shortcutPropertyStringOffset+shortcutWordSize {
					return fmt.Errorf("truncated string property")
				}
				characters := uint64(binary.LittleEndian.Uint32(values[shortcutPropertyHeaderSize:]))
				end := shortcutPropertyStringOffset + characters*shortcutWordSize
				if characters == 0 || end > valueSize || binary.LittleEndian.Uint16(values[end-shortcutWordSize:]) != 0 {
					return fmt.Errorf("invalid string property length or terminator")
				}
			}
			values = values[valueSize:]
		}
		data = data[size:]
	}
	return fmt.Errorf("missing property-store terminator")
}
