package extension

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestImportedPreferencesFiltersAndValidates checks values without allowing export metadata to create arbitrary Wox settings.
func TestImportedPreferencesFiltersAndValidates(t *testing.T) {
	var manifest Manifest
	if err := json.Unmarshal([]byte(`{"preferences":[{"name":"text","type":"textfield"},{"name":"flag","type":"checkbox"},{"name":"secret","type":"password"},{"name":"choice","type":"dropdown","data":[{"value":"one"}]}],"commands":[{"name":"list","preferences":[{"name":"limit","type":"textfield"}]}]}`), &manifest); err != nil {
		t.Fatal(err)
	}
	d := Descriptor{Manifest: manifest, Commands: manifest.Commands}
	values, err := importedPreferences(d, map[string]json.RawMessage{
		"preference:text": json.RawMessage(`""`), "preference:flag": json.RawMessage(`false`),
		"preference:secret": json.RawMessage(`"fixture-secret"`), "preference:choice": json.RawMessage(`"one"`),
		"command:list:limit": json.RawMessage(`"20"`), "command:list:ranAt": json.RawMessage(`123`),
		"Disabled": json.RawMessage(`true`), "preference:unknown": json.RawMessage(`"ignored"`),
	})
	if err != nil || len(values) != 5 || values["preference:text"] != "" || values["preference:flag"] != "false" || values["preference:secret"] != "fixture-secret" || values["command:list:limit"] != "20" {
		t.Fatal("declared preferences were not mapped correctly")
	}
	for key, raw := range map[string]json.RawMessage{
		"preference:text": json.RawMessage(`42`), "preference:flag": json.RawMessage(`"fixture-secret"`),
		"preference:secret": json.RawMessage(`{"secret":"fixture-secret"}`), "preference:choice": json.RawMessage(`"fixture-secret"`),
	} {
		if _, err := importedPreferences(d, map[string]json.RawMessage{key: raw}); err == nil || strings.Contains(err.Error(), "fixture-secret") {
			t.Fatalf("invalid preference %s was accepted or leaked its value", key)
		}
	}
	if values, err := importedPreferences(d, map[string]json.RawMessage{"preference:secret": json.RawMessage(`null`)}); err != nil || len(values) != 0 {
		t.Fatal("null preference must keep the default")
	}
}
