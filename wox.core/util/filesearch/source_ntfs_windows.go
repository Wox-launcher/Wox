//go:build windows

package filesearch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"wox/util/filesearch/column"
	"wox/util/filesearchservice"
	"wox/util/filesearchservice/protocol"
)

type factStream func(context.Context, protocol.Kind, protocol.Boundary, filesearchservice.FactSink) error

type nativeBuildProgress struct {
	files, entries int64
	finalizing     bool
}

// ntfsSource runs in the ordinary Wox process and consumes facts only. Its store
// cursor advances after durable records, so interrupted reads can be replayed.
type ntfsSource struct {
	root   string
	store  *column.Store
	stream factStream
	// Idle metadata events advance only this volatile cursor. Persisting a
	// checkpoint for our own journal write would create another USN event.
	readCursor         protocol.Boundary
	notify             func(context.Context, []ContentHookNotification)
	wantsNotifications func() bool
	progress           func(nativeBuildProgress)
	changed            func()
}

// build retains the old searchable generation until enumeration and journal
// catch-up have both succeeded. A journal reset abandons the unpublished build.
func (s *ntfsSource) build(ctx context.Context) error {
	revision := s.store.Revision()
	var builder *column.Builder
	reportProgress := func(finalizing bool) {
		if s.progress != nil {
			files, entries := builder.Counts()
			s.progress(nativeBuildProgress{files: files, entries: entries, finalizing: finalizing})
		}
	}
	var boundary, target protocol.Boundary
	err := s.stream(ctx, protocol.Enumerate, protocol.Boundary{Volume: s.root}, func(kind protocol.Kind, payload []byte) error {
		switch kind {
		case protocol.Begin:
			var err error
			boundary, err = protocol.DecodeBoundary(payload)
			if err != nil {
				return err
			}
			if !strings.EqualFold(boundary.Volume, s.root) || boundary.RootID == 0 || boundary.NextUSN < 0 {
				return errors.New("invalid NTFS build boundary")
			}
			builder = column.NewBuilder(s.root, boundary.RootID)
		case protocol.Records:
			if builder == nil {
				return errors.New("NTFS records arrived before build boundary")
			}
			err := protocol.VisitEntries(payload, func(entry protocol.Entry) error {
				if entry.ID == boundary.RootID {
					return nil
				}
				record, err := columnFact(entry)
				if err != nil {
					return err
				}
				if entry.Flags&protocol.Deleted != 0 {
					return builder.Remove(record.ID, record.Version)
				}
				return builder.Add(record)
			})
			if err == nil {
				reportProgress(false)
			}
			return err
		case protocol.End:
			var err error
			target, err = protocol.DecodeBoundary(payload)
			if err != nil {
				return err
			}
			return validateFactCheckpoint(boundary, target)
		default:
			return fmt.Errorf("unexpected NTFS build frame %d", kind)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if builder == nil || target.RootID == 0 {
		return errors.New("incomplete NTFS enumeration")
	}
	for boundary.NextUSN < target.NextUSN {
		previous := boundary.NextUSN
		next, err := s.read(ctx, protocol.Replay, boundary, func(changes []column.Change) error {
			for _, change := range changes {
				if change.Deleted {
					if err := builder.Remove(change.ID, change.Version); err != nil {
						return err
					}
				} else if err := builder.Add(change.Record); err != nil {
					return err
				}
			}
			reportProgress(false)
			return nil
		})
		if err != nil {
			return err
		}
		if next.NextUSN <= previous {
			return errors.New("NTFS catch-up stopped before its target")
		}
		boundary = next
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	reportProgress(true)
	base, err := builder.Finish()
	if err != nil {
		return err
	}
	cursor, err := json.Marshal(boundary)
	if err != nil {
		return err
	}
	if err := s.store.Publish(base, string(cursor), "ntfs-v3", revision); err != nil {
		return err
	}
	s.readCursor = boundary
	return nil
}

// replay applies bounded record batches with the old cursor, then commits the
// checkpoint. A crash between them repeats idempotent source-versioned changes.
func (s *ntfsSource) replay(ctx context.Context, wait bool) error {
	var boundary protocol.Boundary
	var cursor string
	if err := s.store.Read(func(view column.View) error {
		cursor = view.Cursor
		return json.Unmarshal([]byte(cursor), &boundary)
	}); err != nil {
		return err
	}
	if !strings.EqualFold(boundary.Volume, s.root) || boundary.RootID == 0 {
		return filesearchservice.ErrFactsReset
	}
	if s.readCursor.JournalID == boundary.JournalID && s.readCursor.RootID == boundary.RootID && s.readCursor.NextUSN >= boundary.NextUSN {
		boundary = s.readCursor
	}
	kind := protocol.Replay
	if wait {
		kind = protocol.Subscribe
	}
	wrote := false
	defer func() {
		// A failed stream can still have published earlier frames. Notify once
		// per read, after releasing store locks, even when its checkpoint fails.
		if wrote && s.changed != nil {
			s.changed()
		}
	}()
	next, err := s.read(ctx, kind, boundary, func(changes []column.Change) error {
		notifyContent := s.notify != nil && (s.wantsNotifications == nil || s.wantsNotifications())
		var notifications []ContentHookNotification
		if notifyContent {
			if err := s.store.Read(func(view column.View) error {
				for _, change := range changes {
					old, exists := view.Lookup(change.ID)
					if exists && old.Version > change.Version {
						continue
					}
					if !change.Deleted && (!exists || old.ParentID == change.ParentID && old.Name == change.Name) {
						continue
					}
					if !exists {
						old = change.Record
					}
					parent, reachable := view.Path(old.ParentID)
					if !reachable {
						continue
					}
					path := filepath.Join(parent, old.Name)
					if old.Flags&column.Directory != 0 {
						notifications = append(notifications, ContentHookNotification{Kind: ContentHookKindScopeReplaced, ScopePath: path})
					} else {
						notifications = append(notifications, ContentHookNotification{Kind: ContentHookKindDelete, Path: path})
					}
				}
				return nil
			}); err != nil {
				return err
			}
		}
		filtered := changes[:0]
		if notifyContent {
			filtered = make([]column.Change, 0, len(changes))
		}
		if err := s.store.Read(func(view column.View) error {
			// A native read can contain several mutations of one ID. Compare
			// against prior records in this batch, not only the durable view.
			pending := make(map[uint64]column.Change, len(changes))
			for _, change := range changes {
				old, found := view.Lookup(change.ID)
				if previous, ok := pending[change.ID]; ok {
					old, found = previous.Record, !previous.Deleted
				}
				if found && change.Version < old.Version {
					continue
				}
				pending[change.ID] = change
				if !change.Deleted && found && old.ParentID == change.ParentID && old.Name == change.Name && old.Flags == change.Flags && old.Size == change.Size && old.Modified == change.Modified {
					continue
				}
				filtered = append(filtered, change)
			}
			return nil
		}); err != nil {
			return err
		}
		if len(filtered) != 0 {
			if err := s.store.Apply(filtered, cursor); err != nil {
				return err
			}
			wrote = true
		}
		if notifyContent {
			if err := s.store.Read(func(view column.View) error {
				for _, change := range changes {
					if change.Deleted {
						continue
					}
					record, exists := view.Lookup(change.ID)
					if !exists || record.Version > change.Version {
						continue
					}
					path, reachable := view.Path(change.ID)
					if !reachable {
						continue
					}
					if record.Flags&column.Directory == 0 {
						notifications = append(notifications, ContentHookNotification{Kind: ContentHookKindUpsert, Path: path})
					}
				}
				// Directory mutations need reconciliation of descendants at the
				// new path, while metadata-only directory events need no content work.
				for _, change := range filtered {
					if !change.Deleted && change.Flags&column.Directory != 0 {
						if path, ok := view.Path(change.ID); ok {
							notifications = append(notifications, ContentHookNotification{Kind: ContentHookKindScopeReplaced, ScopePath: path})
						}
					}
				}
				return nil
			}); err != nil {
				return err
			}
			s.notify(ctx, notifications)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if !wrote {
		s.readCursor = next
		return nil
	}
	encoded, err := json.Marshal(next)
	if err != nil {
		return err
	}
	if err := s.store.Apply(nil, string(encoded)); err != nil {
		return err
	}
	s.readCursor = next
	return nil
}

// read bounds one native read independently of frame size and validates volume
// identity before handing the exclusive cursor to the durable store.
func (s *ntfsSource) read(ctx context.Context, kind protocol.Kind, boundary protocol.Boundary, apply func([]column.Change) error) (protocol.Boundary, error) {
	var next protocol.Boundary
	bytes := 0
	err := s.stream(ctx, kind, boundary, func(kind protocol.Kind, payload []byte) error {
		bytes += len(payload)
		if bytes > 4<<20 {
			return errors.New("NTFS read exceeded bounded response size")
		}
		switch kind {
		case protocol.Records:
			changes := make([]column.Change, 0, len(payload)/56)
			if err := protocol.VisitEntries(payload, func(entry protocol.Entry) error {
				record, err := columnFact(entry)
				if err != nil {
					return err
				}
				changes = append(changes, column.Change{Record: record, Deleted: entry.Flags&protocol.Deleted != 0})
				return nil
			}); err != nil {
				return err
			}
			return apply(changes)
		case protocol.Checkpoint:
			var err error
			next, err = protocol.DecodeBoundary(payload)
			if err != nil {
				return err
			}
			return validateFactCheckpoint(boundary, next)
		default:
			return fmt.Errorf("unexpected NTFS replay frame %d", kind)
		}
	})
	if err == nil && next.RootID == 0 {
		err = errors.New("incomplete NTFS checkpoint")
	}
	return next, err
}

// columnFact keeps native flags out of the portable storage format.
func columnFact(entry protocol.Entry) (column.Record, error) {
	if entry.USN < 0 {
		return column.Record{}, errors.New("invalid negative NTFS version")
	}
	flags := byte(0)
	if entry.Flags&protocol.Directory != 0 {
		flags |= column.Directory
	}
	if entry.Flags&protocol.SizeKnown != 0 {
		flags |= column.SizeKnown
	}
	if entry.Flags&protocol.ModifiedKnown != 0 {
		flags |= column.ModifiedKnown
	}
	return column.Record{ID: entry.ID, ParentID: entry.ParentID, Name: entry.Name, Version: uint64(entry.USN), Flags: flags, Size: entry.Size, Modified: entry.Modified}, nil
}

func validateFactCheckpoint(previous, next protocol.Boundary) error {
	if previous.RootID != next.RootID || previous.JournalID != next.JournalID || !strings.EqualFold(previous.Volume, next.Volume) || next.NextUSN < previous.NextUSN {
		return filesearchservice.ErrFactsReset
	}
	return nil
}
