//go:build windows

package filesearchservice

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf16"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	fsctlEnumUSNData  = 0x000900b3
	mftReadBufferSize = 1 << 20
	ntfsRootRecord    = 5
)

type mftEnumData struct {
	StartFileReferenceNumber uint64
	LowUSN                   int64
	HighUSN                  int64
}

type mftNode struct {
	Reference       uint64
	ParentReference uint64
	Name            string
	IsDir           bool
	USN             int64
	Reason          uint32
}

// EnumerateSnapshot streams paths from the NTFS MFT through Windows' native USN control code.
func EnumerateSnapshot(ctx context.Context, rootPath string, emit func(SnapshotEntry) error) error {
	return EnumerateSnapshots(ctx, []string{rootPath}, emit)
}

// EnumerateSnapshots reads one NTFS volume and returns the union of its requested roots.
func EnumerateSnapshots(ctx context.Context, rootPaths []string, emit func(SnapshotEntry) error) error {
	cleanRoots, volume, err := validateSnapshotRoots(rootPaths)
	if err != nil {
		return err
	}

	volumeName, err := windows.UTF16PtrFromString(`\\.\` + volume)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(
		volumeName,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return fmt.Errorf("open NTFS volume %s: %w", volume, err)
	}
	defer windows.CloseHandle(handle)

	nodes, err := enumerateMFTNodes(ctx, handle)
	if err != nil {
		return err
	}
	entries, err := resolveMFTEntriesForRoots(cleanRoots, volume+`\`, nodes)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if emit != nil {
			if err := emit(entry); err != nil {
				return err
			}
		}
	}
	return nil
}

// validateSnapshotRoots normalizes one same-volume request before opening the raw volume.
func validateSnapshotRoots(rootPaths []string) ([]string, string, error) {
	cleanRoots := make([]string, 0, len(rootPaths))
	seen := make(map[string]struct{}, len(rootPaths))
	volume := ""
	for _, candidate := range rootPaths {
		rootPath := filepath.Clean(strings.TrimSpace(candidate))
		rootVolume := filepath.VolumeName(rootPath)
		if len(rootVolume) != 2 || rootVolume[1] != ':' || !filepath.IsAbs(rootPath) {
			return nil, "", fmt.Errorf("MFT snapshots require an absolute drive path, got %q", candidate)
		}
		if volume != "" && !strings.EqualFold(rootVolume, volume) {
			return nil, "", errors.New("MFT snapshot roots must be on the same volume")
		}
		info, err := os.Stat(rootPath)
		if err != nil {
			return nil, "", err
		}
		if !info.IsDir() {
			return nil, "", fmt.Errorf("MFT snapshot root is not a directory: %s", rootPath)
		}
		key := strings.ToLower(rootPath)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		cleanRoots = append(cleanRoots, rootPath)
		volume = rootVolume
	}
	if len(cleanRoots) == 0 {
		return nil, "", errors.New("MFT snapshot roots are empty")
	}
	return cleanRoots, volume, nil
}

func enumerateMFTNodes(ctx context.Context, handle windows.Handle) (map[uint64]mftNode, error) {
	query := mftEnumData{HighUSN: math.MaxInt64}
	buffer := make([]byte, mftReadBufferSize)
	// ponytail: the first implementation keeps one compact parent/name map in
	// memory; use a disk-backed map only if multi-million-record volumes prove it necessary.
	nodes := make(map[uint64]mftNode, 64*1024)
	for {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		var returned uint32
		err := windows.DeviceIoControl(
			handle,
			fsctlEnumUSNData,
			(*byte)(unsafe.Pointer(&query)),
			uint32(unsafe.Sizeof(query)),
			&buffer[0],
			uint32(len(buffer)),
			&returned,
			nil,
		)
		if errors.Is(err, windows.ERROR_HANDLE_EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("enumerate NTFS MFT: %w", err)
		}
		next, batch, err := parseUSNRecords(buffer[:returned])
		if err != nil {
			return nil, err
		}
		for _, node := range batch {
			nodes[node.Reference] = node
		}
		if next <= query.StartFileReferenceNumber {
			break
		}
		query.StartFileReferenceNumber = next
	}
	return nodes, nil
}

func parseUSNRecords(buffer []byte) (uint64, []mftNode, error) {
	if len(buffer) < 8 {
		return 0, nil, fmt.Errorf("invalid MFT response length %d", len(buffer))
	}
	next := binary.LittleEndian.Uint64(buffer[:8])
	var nodes []mftNode
	if len(buffer) > 8 {
		nodes = make([]mftNode, 0, 1024)
	}
	for offset := 8; offset < len(buffer); {
		if len(buffer)-offset < 60 {
			return 0, nil, fmt.Errorf("truncated USN record at offset %d", offset)
		}
		record := buffer[offset:]
		recordLength := int(binary.LittleEndian.Uint32(record[:4]))
		if recordLength < 60 || recordLength > len(record) {
			return 0, nil, fmt.Errorf("invalid USN record length %d", recordLength)
		}
		if binary.LittleEndian.Uint16(record[4:6]) != 2 {
			return 0, nil, fmt.Errorf("unsupported USN record version %d", binary.LittleEndian.Uint16(record[4:6]))
		}
		nameLength := int(binary.LittleEndian.Uint16(record[56:58]))
		nameOffset := int(binary.LittleEndian.Uint16(record[58:60]))
		if nameLength%2 != 0 || nameOffset < 60 || nameOffset+nameLength > recordLength {
			return 0, nil, fmt.Errorf("invalid USN file name range %d:%d", nameOffset, nameLength)
		}
		nameWords := make([]uint16, nameLength/2)
		for index := range nameWords {
			nameWords[index] = binary.LittleEndian.Uint16(record[nameOffset+index*2:])
		}
		nodes = append(nodes, mftNode{
			Reference:       binary.LittleEndian.Uint64(record[8:16]),
			ParentReference: binary.LittleEndian.Uint64(record[16:24]),
			Name:            string(utf16.Decode(nameWords)),
			IsDir:           binary.LittleEndian.Uint32(record[52:56])&windows.FILE_ATTRIBUTE_DIRECTORY != 0,
			USN:             int64(binary.LittleEndian.Uint64(record[24:32])),
			Reason:          binary.LittleEndian.Uint32(record[40:44]),
		})
		offset += recordLength
	}
	return next, nodes, nil
}

func resolveMFTEntries(rootPath string, volumeRoot string, nodes map[uint64]mftNode) ([]SnapshotEntry, error) {
	return resolveMFTEntriesForRoots([]string{rootPath}, volumeRoot, nodes)
}

// resolveMFTEntriesForRoots resolves the MFT once and keeps only the requested root union.
func resolveMFTEntriesForRoots(rootPaths []string, volumeRoot string, nodes map[uint64]mftNode) ([]SnapshotEntry, error) {
	rootReference := uint64(0)
	for reference := range nodes {
		if reference&0x0000FFFFFFFFFFFF == ntfsRootRecord {
			rootReference = reference
			break
		}
	}
	if rootReference == 0 {
		// FSCTL_ENUM_USN_DATA may omit record #5 itself while top-level records
		// still carry its full reference, including the current sequence number.
		for _, node := range nodes {
			if node.ParentReference&0x0000FFFFFFFFFFFF == ntfsRootRecord {
				rootReference = node.ParentReference
				break
			}
		}
	}
	if rootReference == 0 {
		return nil, errors.New("NTFS root record is missing")
	}

	resolved := map[uint64]string{rootReference: filepath.Clean(volumeRoot)}
	resolving := make(map[uint64]bool)
	var resolve func(uint64) (string, bool)
	resolve = func(reference uint64) (string, bool) {
		if path, ok := resolved[reference]; ok {
			return path, true
		}
		if resolving[reference] {
			return "", false
		}
		node, ok := nodes[reference]
		if !ok || node.Name == "" {
			return "", false
		}
		resolving[reference] = true
		parent, ok := resolve(node.ParentReference)
		delete(resolving, reference)
		if !ok {
			return "", false
		}
		path := filepath.Join(parent, node.Name)
		resolved[reference] = path
		return path, true
	}

	entries := make([]SnapshotEntry, 0, 64*1024)
	for _, rootPath := range rootPaths {
		entries = append(entries, SnapshotEntry{Path: rootPath, IsDir: true})
	}
	for reference, node := range nodes {
		path, ok := resolve(reference)
		if !ok || matchesSnapshotRoot(path, rootPaths) {
			continue
		}
		for _, rootPath := range rootPaths {
			if pathWithinRoot(rootPath, path) {
				entries = append(entries, SnapshotEntry{Path: path, IsDir: node.IsDir})
				break
			}
		}
	}
	// Parent-before-child ordering lets Wox reuse its existing traversal-policy contexts.
	sort.Slice(entries, func(left, right int) bool {
		if len(entries[left].Path) != len(entries[right].Path) {
			return len(entries[left].Path) < len(entries[right].Path)
		}
		return entries[left].Path < entries[right].Path
	})
	return entries, nil
}

func matchesSnapshotRoot(path string, rootPaths []string) bool {
	for _, rootPath := range rootPaths {
		if strings.EqualFold(path, rootPath) {
			return true
		}
	}
	return false
}

func pathWithinRoot(rootPath string, path string) bool {
	relative, err := filepath.Rel(rootPath, path)
	if err != nil || filepath.IsAbs(relative) {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, `..\`))
}
