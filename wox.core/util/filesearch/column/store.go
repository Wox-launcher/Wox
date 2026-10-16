package column

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var ErrStaleBuild = errors.New("column state changed during build")
var ErrNotReady = errors.New("column snapshot is not ready")

type manifest struct {
	Version    int    `json:"version"`
	Generation string `json:"generation"`
	Cursor     string `json:"cursor"`
	Policy     string `json:"policy"`
}

// Store serializes durable mutations and lends read leases across base/delta traversal.
// Its directory belongs to exactly one live engine instance.
type Store struct {
	mu                       sync.RWMutex
	dir                      string
	manifest                 manifest
	base                     *Snapshot
	delta                    map[uint64]Change
	cursor                   string
	log                      *os.File
	revision                 uint64
	closed                   bool
	writeErr                 error
	prepareSnapshot          func(*Snapshot) error
	countsMu                 sync.Mutex
	countsValid              bool
	countsRevision           uint64
	countFiles, countEntries int64
}

// Counts caches a coherent revision's visible counts without constructing paths.
// Repeated Settings/status reads are constant time until the source changes.
func (s *Store) Counts(ctx context.Context) (files, entries int64, err error) {
	s.countsMu.Lock()
	defer s.countsMu.Unlock()
	err = s.Read(func(view View) error {
		if s.countsValid && s.countsRevision == view.Revision {
			files, entries = s.countFiles, s.countEntries
			return nil
		}
		var err error
		files, entries, err = countVisibleEntries(ctx, view)
		if err != nil {
			return err
		}
		s.countsRevision, s.countFiles, s.countEntries = view.Revision, files, entries
		s.countsValid = true
		return nil
	})
	return
}

// countVisibleEntries resolves each parent directory once per revision. Looking
// up every file and all its ancestors through the sorted ID column made whole-
// volume status requests exceed their deadline before the cache could populate.
func countVisibleEntries(ctx context.Context, view View) (files, entries int64, err error) {
	const visiting, reachable, unreachable = byte(1), byte(2), byte(3)
	parents := map[uint64]byte{view.Base.IDs[0]: reachable}
	var local [32]uint64
	directoryReachable := func(id uint64) (bool, error) {
		chain := local[:0]
		valid := false
		for {
			if state := parents[id]; state != 0 {
				valid = state == reachable
				break
			}
			if err := ctx.Err(); err != nil {
				return false, err
			}
			parents[id] = visiting
			chain = append(chain, id)
			record, exists := view.Lookup(id)
			if !exists || record.Flags&Directory == 0 {
				break
			}
			id = record.ParentID
		}
		state := unreachable
		if valid {
			state = reachable
		}
		for _, id := range chain {
			parents[id] = state
		}
		return valid, nil
	}
	count := func(flags byte, parentID uint64, root bool) error {
		if flags&Structural != 0 {
			return nil
		}
		valid := root
		if !root {
			var err error
			valid, err = directoryReachable(parentID)
			if err != nil {
				return err
			}
		}
		if valid {
			entries++
			if flags&Directory == 0 {
				files++
			}
		}
		return nil
	}
	for row, id := range view.Base.IDs {
		if row%4096 == 0 && ctx.Err() != nil {
			return 0, 0, ctx.Err()
		}
		if _, changed := view.Delta[id]; changed {
			continue
		}
		parent := view.Base.Parents[row]
		if row != 0 && parent == NoRow {
			continue
		}
		parentID := id
		if row != 0 {
			parentID = view.Base.IDs[parent]
		}
		if err := count(view.Base.Flags[row], parentID, row == 0); err != nil {
			return 0, 0, err
		}
	}
	for _, change := range view.Delta {
		if err := ctx.Err(); err != nil {
			return 0, 0, err
		}
		if !change.Deleted {
			if err := count(change.Flags, change.ParentID, change.ID == view.Base.IDs[0]); err != nil {
				return 0, 0, err
			}
		}
	}
	return files, entries, nil
}

// NeedsCompaction bounds both live overlay growth and repeated writes to the
// same entries. The latter grow the journal even when the overlay remains small.
func (s *Store) NeedsCompaction() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.base == nil || s.closed {
		return false
	}
	threshold := min(100000, max(10000, s.base.Len()/10))
	if len(s.delta) >= threshold {
		return true
	}
	info, err := s.log.Stat()
	return err == nil && info.Size() >= 64<<20
}

// View is valid only for the duration of Store.Read. Borrowed names must not escape it.
type View struct {
	Base     *Snapshot
	Delta    map[uint64]Change
	Cursor   string
	Revision uint64
	Policy   string
}

// OpenStore restores the last published generation and only complete journal batches.
func OpenStore(dir string, preparers ...func(*Snapshot) error) (*Store, error) {
	if len(preparers) > 1 {
		return nil, errors.New("only one snapshot preparer is allowed")
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, delta: make(map[uint64]Change)}
	if len(preparers) != 0 {
		s.prepareSnapshot = preparers[0]
	}
	data, err := os.ReadFile(filepath.Join(dir, "current.json"))
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(data, &s.manifest); err != nil {
		return nil, err
	}
	decoded, err := hex.DecodeString(s.manifest.Generation)
	if err != nil || len(decoded) != 16 || s.manifest.Version != 1 {
		return nil, fmt.Errorf("invalid column generation")
	}
	s.base, err = Open(filepath.Join(dir, s.manifest.Generation+".columns"))
	if err != nil {
		return nil, err
	}
	s.log, err = os.OpenFile(filepath.Join(dir, s.manifest.Generation+".log"), os.O_RDWR, 0600)
	if err != nil {
		s.base.Close()
		return nil, err
	}
	s.cursor = s.manifest.Cursor
	var validBytes int64
	for {
		changes, cursor, size, readErr := readBatch(s.log)
		if errors.Is(readErr, io.EOF) || errors.Is(readErr, io.ErrUnexpectedEOF) {
			break
		}
		if readErr != nil {
			s.Close()
			return nil, readErr
		}
		if err := s.validateChanges(changes); err != nil {
			s.Close()
			return nil, err
		}
		s.applyMemory(changes, cursor)
		validBytes += size
	}
	if err := s.log.Truncate(validBytes); err != nil {
		s.Close()
		return nil, err
	}
	if _, err := s.log.Seek(validBytes, io.SeekStart); err != nil {
		s.Close()
		return nil, err
	}
	return s, nil
}

// Read pins the current mapping and delta until the callback returns.
func (s *Store) Read(callback func(View) error) error {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.closed {
		return os.ErrClosed
	}
	if s.base == nil {
		return ErrNotReady
	}
	return callback(View{Base: s.base, Delta: s.delta, Cursor: s.cursor, Revision: s.revision, Policy: s.manifest.Policy})
}

func (s *Store) Revision() uint64 {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revision
}

// ReleaseIdleMemory waits for borrowed views before evicting clean mapped pages.
// Names and deltas remain valid; the OS faults file pages back on the next read.
func (s *Store) ReleaseIdleMemory() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.base == nil || s.base.mapping == nil {
		return nil
	}
	return s.base.mapping.releasePages()
}

// Publish writes and validates off the query lock, then rejects a stale build before switching.
// Source coordinators replay events through their captured boundary before calling it.
func (s *Store) Publish(base *Snapshot, cursor, policy string, expectedRevision uint64) (err error) {
	if err := base.Validate(); err != nil {
		return err
	}
	// The caller owns this unpublished snapshot; preparation never mutates a
	// live mapping or extends the query lock's critical section.
	if s.prepareSnapshot != nil {
		if err := s.prepareSnapshot(base); err != nil {
			return err
		}
	}
	var token [16]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	generation := hex.EncodeToString(token[:])
	columnPath := filepath.Join(s.dir, generation+".columns")
	logPath := filepath.Join(s.dir, generation+".log")
	manifestPath := filepath.Join(s.dir, generation+".json")
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(columnPath)
			_ = os.Remove(logPath)
			_ = os.Remove(manifestPath)
		}
	}()
	mapped, err := writeAndOpen(columnPath, base)
	if err != nil {
		return err
	}
	defer func() {
		if !committed {
			mapped.Close()
		}
	}()
	logFile, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer func() {
		if !committed {
			logFile.Close()
		}
	}()
	if err := logFile.Sync(); err != nil {
		return err
	}
	next := manifest{Version: 1, Generation: generation, Cursor: cursor, Policy: policy}
	payload, err := json.Marshal(next)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(manifestPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	_, writeErr := file.Write(payload)
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		return writeErr
	}
	if closeErr != nil {
		return closeErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if s.revision != expectedRevision {
		return ErrStaleBuild
	}
	replaced, publishErr := replaceManifest(manifestPath, filepath.Join(s.dir, "current.json"))
	if !replaced {
		return publishErr
	}
	old := s.manifest.Generation
	if s.base != nil {
		_ = s.base.Close()
	}
	if s.log != nil {
		_ = s.log.Close()
	}
	s.base, s.log, s.manifest, s.cursor = mapped, logFile, next, cursor
	s.delta = make(map[uint64]Change)
	s.revision++
	// A directory fsync can fail after rename succeeded. Keep the published files
	// and in-memory generation aligned instead of deleting the now-referenced data.
	s.writeErr = publishErr
	committed = true
	if old != "" && publishErr == nil {
		_ = os.Remove(filepath.Join(s.dir, old+".columns"))
		_ = os.Remove(filepath.Join(s.dir, old+".log"))
	}
	return publishErr
}

// Apply makes mutations durable before exposing them or advancing the recoverable cursor.
func (s *Store) Apply(changes []Change, cursor string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return os.ErrClosed
	}
	if s.base == nil {
		return ErrNotReady
	}
	if s.writeErr != nil {
		return s.writeErr
	}
	if err := s.validateChanges(changes); err != nil {
		return err
	}
	data, err := encodeBatch(changes, cursor)
	if err != nil {
		return err
	}
	if _, err := s.log.Write(data); err != nil {
		s.writeErr = err
		return err
	}
	if err := s.log.Sync(); err != nil {
		s.writeErr = err
		return err
	}
	s.applyMemory(changes, cursor)
	return nil
}

// validateChanges keeps malformed identities and names out of the durable overlay.
func (s *Store) validateChanges(changes []Change) error {
	proposed := make(map[uint64]Change, len(changes))
	for _, change := range changes {
		if change.ID == s.base.IDs[0] || change.ID == change.ParentID {
			return fmt.Errorf("invalid delta identity")
		}
		name := change.Name
		if !change.Deleted && (name == "" || name == "." || name == ".." || strings.ContainsRune(name, 0) || strings.ContainsRune(name, '/') || strings.ContainsRune(name, filepath.Separator) || filepath.VolumeName(name) != "") {
			return fmt.Errorf("invalid delta name")
		}
		if current, ok := proposed[change.ID]; ok && change.Version != 0 && current.Version > change.Version {
			continue
		}
		if current, ok := s.delta[change.ID]; ok && change.Version != 0 && current.Version > change.Version {
			continue
		}
		if row, ok := s.base.Row(change.ID); ok && len(s.base.Versions) != 0 && change.Version != 0 && s.base.Versions[row] > change.Version {
			continue
		}
		proposed[change.ID] = change
	}
	view := View{Base: s.base, Delta: s.delta}
	for id, change := range proposed {
		if change.Deleted {
			continue
		}
		seen := map[uint64]bool{id: true}
		parent := change.ParentID
		for parent != s.base.IDs[0] {
			if seen[parent] {
				return fmt.Errorf("delta parent cycle")
			}
			seen[parent] = true
			record, ok := view.Lookup(parent)
			if next, exists := proposed[parent]; exists {
				record, ok = next.Record, !next.Deleted
			}
			if !ok {
				break
			}
			if record.Flags&Directory == 0 {
				return fmt.Errorf("delta parent is not a directory")
			}
			parent = record.ParentID
		}
	}
	return nil
}

func (s *Store) applyMemory(changes []Change, cursor string) {
	for _, change := range changes {
		if old, ok := s.delta[change.ID]; ok {
			if change.Version != 0 && old.Version > change.Version {
				continue
			}
		} else if row, ok := s.base.Row(change.ID); ok && len(s.base.Versions) != 0 && change.Version != 0 && s.base.Versions[row] > change.Version {
			continue
		}
		change.Name = strings.Clone(change.Name)
		s.delta[change.ID] = change
	}
	s.cursor = cursor
	s.revision++
}

// Lookup overlays mutations by stable entry identity, including directory renames.
func (v View) Lookup(id uint64) (Record, bool) {
	if change, ok := v.Delta[id]; ok {
		return change.Record, !change.Deleted
	}
	row, ok := v.Base.Row(id)
	if !ok {
		return Record{}, false
	}
	return v.Base.Record(row), true
}

// Path follows both layers, so deleting a directory also hides its existing descendants.
func (v View) Path(id uint64) (string, bool) {
	if len(v.Delta) == 0 {
		row, ok := v.Base.Row(id)
		if !ok {
			return "", false
		}
		return v.Base.Path(row)
	}

	var local [32]string
	parts := local[:0]
	for steps := 0; id != v.Base.IDs[0]; steps++ {
		if steps > v.Base.Len()+len(v.Delta) {
			return "", false
		}
		record, ok := v.Lookup(id)
		if !ok || steps > 0 && record.Flags&Directory == 0 {
			return "", false
		}
		parts = append(parts, record.Name)
		id = record.ParentID
	}
	for a, b := 0, len(parts)-1; a < b; a, b = a+1, b-1 {
		parts[a], parts[b] = parts[b], parts[a]
	}
	return filepath.Join(v.Base.Root, strings.Join(parts, string(filepath.Separator))), true
}

// Reachable tests ancestry without allocating paths during compaction and counting.
func (v View) Reachable(id uint64) bool {
	for steps := 0; id != v.Base.IDs[0]; steps++ {
		if steps > v.Base.Len()+len(v.Delta) {
			return false
		}
		record, ok := v.Lookup(id)
		if !ok || steps > 0 && record.Flags&Directory == 0 {
			return false
		}
		id = record.ParentID
	}
	return true
}

// Compact builds from one coherent read view; publication retries are owned by the scheduler.
func (s *Store) Compact(ctx context.Context) error {
	var builder *Builder
	var cursor, policy string
	var revision uint64
	err := s.Read(func(v View) error {
		cursor, policy, revision = v.Cursor, v.Policy, v.Revision
		builder = NewBuilder(v.Base.Root, v.Base.IDs[0])
		for row := 0; row < v.Base.Len(); row++ {
			if row%4096 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			record := v.Base.Record(uint32(row))
			if _, changed := v.Delta[record.ID]; changed {
				continue
			}
			if !v.Reachable(record.ID) {
				continue
			}
			if err := builder.Add(record); err != nil {
				return err
			}
		}
		for _, change := range v.Delta {
			if !change.Deleted && v.Reachable(change.ID) {
				if err := builder.Add(change.Record); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	base, err := builder.Finish()
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return s.Publish(base, cursor, policy, revision)
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	var err error
	if s.base != nil {
		err = s.base.Close()
	}
	if s.log != nil {
		err = errors.Join(err, s.log.Close())
	}
	s.delta = nil
	return err
}
