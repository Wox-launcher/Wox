package system

import (
	"archive/zip"
	"context"
	"os"
	"path/filepath"
	"testing"
	"wox/plugin"
	"wox/util/selection"
)

// TestThemeInstallerPriority keeps install results and package errors above generic selection actions.
func TestThemeInstallerPriority(t *testing.T) {
	for _, valid := range []bool{true, false} {
		filePath := filepath.Join(t.TempDir(), "theme.wox-theme")
		file, err := os.Create(filePath)
		if err != nil {
			t.Fatal(err)
		}
		archive := zip.NewWriter(file)
		entry, err := archive.Create("theme.json")
		if err != nil {
			t.Fatal(err)
		}
		document := `{"SchemaVersion":2,"ThemeId":"test","ThemeName":"Knit","BaseBackgroundColor":"#F5F1E9","BaseTextColor":"#293F50","BaseAccentColor":"#526F89"}`
		if !valid {
			document = "invalid"
		}
		if _, err := entry.Write([]byte(document)); err != nil {
			t.Fatal(err)
		}
		if err := archive.Close(); err != nil {
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
		response := (&PluginInstallerPlugin{}).Query(context.Background(), plugin.Query{
			Type:      plugin.QueryTypeSelection,
			Selection: selection.Selection{Type: selection.SelectionTypeFile, FilePaths: []string{filePath}},
		})
		if len(response.Results) != 1 || response.Results[0].Score != 3000 {
			t.Fatalf("valid=%t: expected one top-priority installer result: %+v", valid, response.Results)
		}
		if valid && (len(response.Results[0].Actions) != 1 || response.Results[0].Actions[0].Name != "i18n:plugin_theme_install_theme") {
			t.Fatal("default install action missing")
		}
		if !valid && len(response.Results[0].Actions) != 0 {
			t.Fatal("invalid package must not be installable")
		}
	}
}
