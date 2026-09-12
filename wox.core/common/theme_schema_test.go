package common

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestThemeSchemaDispatch verifies new codecs do not need dispatcher changes.
func TestThemeSchemaDispatch(t *testing.T) {
	const version = 987
	registerThemeSchema(version, themeSchema{
		parse:   func(data []byte) (Theme, error) { return Theme{SchemaVersion: version, ThemeName: "future"}, nil },
		marshal: func(theme Theme) ([]byte, error) { return []byte(`{"SchemaVersion":987}`), nil },
		resolve: func(theme Theme, platform, variant string) (Theme, error) {
			theme.ThemeName = platform + "/" + variant
			return theme, nil
		},
	})
	t.Cleanup(func() { delete(themeSchemas, version) })
	var theme Theme
	if err := json.Unmarshal([]byte(`{"SchemaVersion":987}`), &theme); err != nil {
		t.Fatal(err)
	}
	resolved, err := theme.ResolveForTarget("linux", "gnome")
	if err != nil || resolved.ThemeName != "linux/gnome" {
		t.Fatalf("dispatch: %+v, %v", resolved, err)
	}
	encoded, err := json.Marshal(resolved)
	if err != nil || string(encoded) != `{"SchemaVersion":987}` {
		t.Fatalf("marshal: %s, %v", encoded, err)
	}
}

// TestThemeMinimumVersion covers legacy omission, both schemas, prereleases and lossless metadata.
func TestThemeMinimumVersion(t *testing.T) {
	for _, input := range []string{`{"ThemeName":"Legacy"}`, minimalV2Theme} {
		for _, minimum := range []string{"", "2.4.3", "2.4.4", "2.4.3-beta.1", "invalid"} {
			data := strings.TrimSuffix(input, "}") + `,"MinWoxVersion":"` + minimum + `"}`
			var theme Theme
			err := json.Unmarshal([]byte(data), &theme)
			if minimum == "invalid" {
				if err == nil {
					t.Fatal("invalid minimum accepted")
				}
				continue
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := theme.EnsureWoxVersionSupported("2.4.3"); (err != nil) != (minimum == "2.4.4") {
				t.Fatalf("minimum %q: %v", minimum, err)
			}
			encoded, err := json.Marshal(theme)
			if err != nil {
				t.Fatal(err)
			}
			var saved Theme
			if err := json.Unmarshal(encoded, &saved); err != nil || saved.MinWoxVersion != minimum {
				t.Fatalf("metadata lost: %s, %v", encoded, err)
			}
			if saved.SchemaVersion < 1 {
				t.Fatal("legacy schema was not normalized")
			}
		}
	}
	if err := (Theme{MinWoxVersion: "2.4.3"}).EnsureWoxVersionSupported("2.4.3-beta.1"); err == nil {
		t.Fatal("prerelease passed release floor")
	}
}
