package notes

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"github.com/google/uuid"
	"wox/common"
	"wox/setting"
)

const noteGroupPrefix = "group:"
const notePlacementPrefix = "placement:"

// readNotesTree reads organization only; malformed records cannot hide document records.
func readNotesTree(store *setting.PluginSettingStore) (common.NotesTree, error) {
	var tree common.NotesTree
	groups, err := store.ListByPrefix(noteGroupPrefix)
	if err != nil {
		return tree, err
	}
	for key, raw := range groups {
		var group common.NoteGroup
		if json.Unmarshal([]byte(raw), &group) == nil && group.ID != "" && key == noteGroupPrefix+group.ID {
			tree.Groups = append(tree.Groups, group)
		}
	}
	positions, err := store.ListByPrefix(notePlacementPrefix)
	if err != nil {
		return tree, err
	}
	for key, raw := range positions {
		var position common.NotePlacement
		if json.Unmarshal([]byte(raw), &position) == nil && position.NoteID != "" && key == notePlacementPrefix+position.NoteID {
			tree.Placements = append(tree.Placements, position)
		}
	}
	return tree, nil
}

func (p *Plugin) NotesTree(context.Context) (common.NotesTree, error) {
	return readNotesTree(p.repository.store)
}

// NotesOrganize returns a fresh snapshot only after the organization transaction succeeds.
func (p *Plugin) NotesOrganize(_ context.Context, change common.NotesTreeChange) (common.NotesTree, error) {
	if err := p.repository.Organize(change); err != nil {
		return common.NotesTree{}, err
	}
	return readNotesTree(p.repository.store)
}

// Organize serializes all touched siblings with their oplogs, without changing any note revision.
func (r *Repository) Organize(change common.NotesTreeChange) error {
	err := r.store.UpdateWithSync(func(store *setting.PluginSettingStore) ([]setting.PluginSettingChange, error) {
		tree, err := readNotesTree(store)
		if err != nil {
			return nil, err
		}
		groups := map[string]common.NoteGroup{}
		for _, g := range tree.Groups {
			groups[g.ID] = g
		}
		positions := map[string]common.NotePlacement{}
		for _, p := range tree.Placements {
			positions[p.NoteID] = p
		}
		var changes []setting.PluginSettingChange
		putGroup := func(g common.NoteGroup) {
			groups[g.ID] = g
			changes = append(changes, setting.PluginSettingChange{Key: noteGroupPrefix + g.ID, Value: g, Syncable: true})
		}
		putPosition := func(p common.NotePlacement) {
			positions[p.NoteID] = p
			changes = append(changes, setting.PluginSettingChange{Key: notePlacementPrefix + p.NoteID, Value: p, Syncable: true})
		}
		if change.Action == "create" {
			change.Group = true
			change.ID = uuid.NewString()
		}
		if change.ID == "" {
			return nil, errors.New("organization ID is empty")
		}
		if change.ParentID != "" {
			parent, ok := groups[change.ParentID]
			if !ok || parent.Deleted {
				return nil, errors.New("destination group is unavailable")
			}
		}
		switch change.Action {
		case "create":
			if strings.TrimSpace(change.Name) == "" {
				return nil, errors.New("group name is empty")
			}
			groups[change.ID] = common.NoteGroup{ID: change.ID, Name: strings.TrimSpace(change.Name), ParentID: change.ParentID}
		case "rename", "delete":
			g, ok := groups[change.ID]
			if !ok || g.Deleted {
				return nil, errors.New("group is unavailable")
			}
			if change.Action == "rename" {
				if strings.TrimSpace(change.Name) == "" {
					return nil, errors.New("group name is empty")
				}
				g.Name = strings.TrimSpace(change.Name)
				putGroup(g)
				return changes, nil
			}
			// Preserve the tombstone's original parent so children arriving later can still be recovered.
			parent := common.ResolveNoteParent(g.ParentID, groups)
			maxGroup, maxNote := 0, 0
			for _, item := range groups {
				if common.ResolveNoteParent(item.ParentID, groups) == parent {
					maxGroup = max(maxGroup, item.Order+1)
				}
			}
			for _, item := range positions {
				if common.ResolveNoteParent(item.ParentID, groups) == parent {
					maxNote = max(maxNote, item.Order+1)
				}
			}
			// Deletion is also an organization operation: freeze legacy siblings before appending promoted notes.
			records, err := (&Repository{store: store}).List(false)
			if err != nil {
				return nil, err
			}
			sortNotesForTree(records, positions)
			for _, record := range records {
				if _, placed := positions[record.ID]; !placed && parent == "" {
					putPosition(common.NotePlacement{NoteID: record.ID, Order: maxNote})
					maxNote++
				}
			}
			sort.Slice(tree.Groups, func(i, j int) bool {
				if tree.Groups[i].Order != tree.Groups[j].Order {
					return tree.Groups[i].Order < tree.Groups[j].Order
				}
				return tree.Groups[i].ID < tree.Groups[j].ID
			})
			sort.Slice(tree.Placements, func(i, j int) bool {
				if tree.Placements[i].Order != tree.Placements[j].Order {
					return tree.Placements[i].Order < tree.Placements[j].Order
				}
				return tree.Placements[i].NoteID < tree.Placements[j].NoteID
			})
			for _, item := range tree.Groups {
				if item.ParentID == g.ID && !item.Deleted {
					item.ParentID = parent
					item.Order = maxGroup
					maxGroup++
					putGroup(item)
				}
			}
			for _, item := range tree.Placements {
				if item.ParentID == g.ID {
					item.ParentID = parent
					item.Order = maxNote
					maxNote++
					putPosition(item)
				}
			}
			g.Deleted = true
			putGroup(g)
			return changes, nil
		case "move":
			if change.Group {
				g, ok := groups[change.ID]
				if !ok || g.Deleted {
					return nil, errors.New("group is unavailable")
				}
				seen := map[string]bool{}
				for parent := change.ParentID; parent != ""; parent = groups[parent].ParentID {
					if parent == change.ID || seen[parent] {
						return nil, errors.New("cannot move a group into its descendants")
					}
					seen[parent] = true
				}
			} else {
				var record common.NoteRecord
				if err := store.Get(noteSettingPrefix+change.ID, &record); err != nil {
					return nil, err
				}
				if record.DeletedAt > 0 {
					return nil, errors.New("cannot move a deleted note")
				}
			}
		default:
			return nil, errors.New("unknown organization action")
		}
		// Freeze the visible order of affected destination siblings on the first manual move.
		ids := []string{}
		if change.Group {
			for id, g := range groups {
				if id != change.ID && !g.Deleted && common.ResolveNoteParent(g.ParentID, groups) == change.ParentID {
					ids = append(ids, id)
				}
			}
			sort.Slice(ids, func(i, j int) bool {
				a, b := groups[ids[i]], groups[ids[j]]
				if a.Order != b.Order {
					return a.Order < b.Order
				}
				return a.ID < b.ID
			})
		} else {
			records, err := (&Repository{store: store}).List(false)
			if err != nil {
				return nil, err
			}

			sortNotesForTree(records, positions)
			sourceParent := common.ResolveNoteParent(positions[change.ID].ParentID, groups)
			sourceOrder := 0
			for _, n := range records {
				if n.ID == change.ID {
					continue
				}
				parent := common.ResolveNoteParent(positions[n.ID].ParentID, groups)
				if parent == change.ParentID {
					ids = append(ids, n.ID)
				} else if parent == sourceParent {
					putPosition(common.NotePlacement{NoteID: n.ID, ParentID: sourceParent, Order: sourceOrder})
					sourceOrder++
				}
			}

		}
		index := len(ids)
		if change.Action == "create" || change.First {
			index = 0
		}
		if change.BeforeID != "" {
			index = -1
			for i, id := range ids {
				if id == change.BeforeID {
					index = i
					break
				}
			}
			if index < 0 {
				return nil, errors.New("destination sibling is unavailable")
			}
		}
		ids = append(ids, "")
		copy(ids[index+1:], ids[index:])
		ids[index] = change.ID
		for order, id := range ids {
			if change.Group {
				g := groups[id]
				g.ParentID = change.ParentID
				g.Order = order
				putGroup(g)
			} else {
				putPosition(common.NotePlacement{NoteID: id, ParentID: change.ParentID, Order: order})
			}
		}
		return changes, nil
	})
	if err == nil {
		go r.notify("")
	}
	return err
}

// sortNotesForTree preserves manual order across edits, including tied orders received from sync.
func sortNotesForTree(records []common.NoteRecord, positions map[string]common.NotePlacement) {
	sort.SliceStable(records, func(i, j int) bool {
		a, oka := positions[records[i].ID]
		b, okb := positions[records[j].ID]
		if oka != okb {
			return oka
		}
		if oka && a.Order != b.Order {
			return a.Order < b.Order
		}
		if !oka && records[i].UpdatedAt != records[j].UpdatedAt {
			return records[i].UpdatedAt > records[j].UpdatedAt
		}
		return records[i].ID < records[j].ID
	})
}
