package snippets

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"wox/common"
	"wox/plugin/system/snippets/keyword"
	"wox/setting"
)

const pluginID = common.SnippetsPluginID

var (
	ErrKeywordConflict = errors.New("snippet keyword is already in use")
)

// Record is one row of the synchronized snippets table.
type Record struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Keyword           string `json:"keyword"`
	Text              string `json:"text"`
	ImportSource      string `json:"importSource,omitempty"`
	ImportID          string `json:"importId,omitempty"`
	ImportFingerprint string `json:"importFingerprint,omitempty"`
}

type Repository struct{ store *setting.PluginSettingStore }

func NewRepository(store *setting.PluginSettingStore) *Repository { return &Repository{store: store} }

func (r *Repository) List() ([]Record, error) { return listRecords(r.store) }

func listRecords(store *setting.PluginSettingStore) ([]Record, error) {
	var raw string
	if err := store.Get(snippetsSetting, &raw); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return []Record{}, nil
		}
		return nil, err
	}
	if strings.TrimSpace(raw) == "" {
		return []Record{}, nil
	}
	var records []Record
	if err := json.Unmarshal([]byte(raw), &records); err != nil {
		return nil, err
	}
	if records == nil {
		records = []Record{}
	}
	return records, nil
}

// saveTable stores one complete setting, matching normal plugin-table sync semantics.
func (r *Repository) saveTable(raw string) error {
	var records []Record
	if err := json.Unmarshal([]byte(raw), &records); err != nil || records == nil {
		return errors.New("invalid snippet table")
	}
	ids := make(map[string]bool, len(records))
	keywords := make(map[string]bool, len(records))
	for i := range records {
		record := &records[i]
		if strings.TrimSpace(record.Name) == "" || !keyword.Valid(record.Keyword) {
			return errors.New("invalid snippet")
		}
		if record.Keyword != "" && keywords[record.Keyword] {
			return ErrKeywordConflict
		}
		keywords[record.Keyword] = true
		if record.ID == "" || ids[record.ID] {
			if ids[record.ID] {
				record.ImportSource, record.ImportID, record.ImportFingerprint = "", "", ""
			}
			record.ID = uuid.NewString()
		}
		ids[record.ID] = true
	}
	encoded, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return r.store.SetWithSync(snippetsSetting, string(encoded), true)
}
