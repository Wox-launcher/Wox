package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"gorm.io/gorm"
	"wox/cloudsync"
	"wox/database"
	"wox/setting"
)

// importedPreferences accepts only declared preferences, never runtime metadata or Wox control settings.
func importedPreferences(d Descriptor, raw map[string]json.RawMessage) (map[string]string, error) {
	values := map[string]string{}
	read := func(preferences []Preference, prefix string) error {
		for _, preference := range preferences {
			key := prefix + preference.Name
			value, present := raw[key]
			if !present || strings.TrimSpace(string(value)) == "null" {
				continue
			}
			var text string
			switch preference.Type {
			case "checkbox":
				var checked bool
				if err := json.Unmarshal(value, &checked); err != nil {
					return fmt.Errorf("invalid Raycast preference %q: expected a boolean", key)
				}
				text = strconv.FormatBool(checked)
			case "text", "textfield", "password", "dropdown":
				if err := json.Unmarshal(value, &text); err != nil {
					return fmt.Errorf("invalid Raycast preference %q: expected text", key)
				}
				if preference.Type == "dropdown" {
					found := false
					for _, option := range preference.Data {
						found = found || option.Value == text
					}
					if !found {
						return fmt.Errorf("invalid Raycast preference %q: option is unavailable", key)
					}
				}
			default:
				continue
			}
			values[key] = text
		}
		return nil
	}
	if err := read(d.Manifest.Preferences, "preference:"); err != nil {
		return nil, err
	}
	for _, command := range d.Commands {
		if err := read(command.Preferences, "command:"+command.Name+":"); err != nil {
			return nil, err
		}
	}
	return values, nil
}

// saveImportedPreferences commits values and sync records together. Existing Wox overrides win on replacement.
func saveImportedPreferences(ctx context.Context, pluginID string, values map[string]string) error {
	if len(values) == 0 {
		return nil
	}
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	// Acquire the sync lock before opening the transaction, as ordinary setting writes do.
	return cloudsync.WithLocalSyncMutation(func() error {
		return database.GetDB().WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			store := setting.NewPluginSettingStore(tx, pluginID)
			for _, key := range keys {
				var existing string
				if err := store.Get(key, &existing); err == nil {
					continue
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return fmt.Errorf("cannot read Raycast preference %q", key)
				}
				if err := store.SetWithSync(key, values[key], true); err != nil {
					return fmt.Errorf("cannot save Raycast preference %q", key)
				}
			}
			return nil
		})
	})
}
