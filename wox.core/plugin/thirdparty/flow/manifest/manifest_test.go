package manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"wox/plugin"
	"wox/setting/definition"
)

func TestParseFlowPlugin(t *testing.T) {
	directory := t.TempDir()
	writeFlowFile(t, directory, "main.py", "print('ok')\n")
	writeFlowFile(t, directory, "icon.png", "png")
	writeFlowFile(t, directory, "plugin.json", `{
		"ID": "demo",
		"Name": "Demo",
		"Description": "Hello",
		"Language": "PYTHON",
		"ExecuteFileName": "main.py",
		"ActionKeyword": "*",
		"ActionKeywords": ["hi", "*", ""],
		"IcoPath": "icon.png"
	}`)
	writeFlowFile(t, directory, "SettingsTemplate.yaml", `
body:
  - type: checkbox
    attributes:
      name: enabled
      label: Enabled
      defaultValue: true
  - type: textBox
    attributes:
      name: mode
      defaultValue: fast
  - type: dropdown
    attributes:
      name: size
      defaultValue: m
      options:
        - {label: Small, value: s}
        - {label: Medium, value: m}
  - type: textarea
    attributes:
      name: note
  - type: passwordBox
    attributes:
      name: token
  - type: label
    attributes:
      name: ignored
`)

	descriptor, err := Parse(directory)
	if err != nil {
		t.Fatal(err)
	}
	if descriptor.Kind != KindScript || descriptor.Language != "python" {
		t.Fatalf("kind=%s language=%s", descriptor.Kind, descriptor.Language)
	}
	if descriptor.Metadata.Runtime != string(RuntimeJSONRPC) {
		t.Fatalf("runtime %s", descriptor.Metadata.Runtime)
	}
	if descriptor.Metadata.Version != "1.0.0" || descriptor.Metadata.MinWoxVersion != minWoxVersion {
		t.Fatalf("version %s min %s", descriptor.Metadata.Version, descriptor.Metadata.MinWoxVersion)
	}
	if got := descriptor.Metadata.TriggerKeywords; len(got) != 2 || got[0] != "*" || got[1] != "hi" {
		t.Fatalf("keywords %#v", got)
	}
	if !flowHasFeature(descriptor.Metadata, plugin.MetadataFeatureIgnoreAutoScore) {
		t.Fatal("missing ignoreAutoScore")
	}
	if descriptor.Metadata.Icon == "" || descriptor.Metadata.Icon[:9] != "absolute:" {
		t.Fatalf("icon %s", descriptor.Metadata.Icon)
	}
	if descriptor.SettingKinds["enabled"] != SettingBool || descriptor.SettingKinds["mode"] != SettingString {
		t.Fatalf("kinds %#v", descriptor.SettingKinds)
	}
	if _, ok := descriptor.SettingKinds["ignored"]; ok {
		t.Fatal("label setting was emitted")
	}
	if len(descriptor.Metadata.SettingDefinitions) != 5 {
		t.Fatalf("settings %d", len(descriptor.Metadata.SettingDefinitions))
	}
	note := descriptor.Metadata.SettingDefinitions[3].Value.(*definition.PluginSettingValueTextBox)
	if note.GetKey() != "note" || note.MaxLines != 6 {
		t.Fatalf("textarea key=%s lines=%d", note.GetKey(), note.MaxLines)
	}
	if descriptor.Metadata.SettingDefinitions[0].Value.GetDefaultValue() != "true" {
		t.Fatalf("checkbox default %s", descriptor.Metadata.SettingDefinitions[0].Value.GetDefaultValue())
	}
}

func TestParseFlowPluginKeywordsAndDotNet(t *testing.T) {
	root := t.TempDir()
	global := filepath.Join(root, "global")
	dotnet := filepath.Join(root, "dotnet")
	skipped := filepath.Join(root, "rust")
	for _, directory := range []string{global, dotnet, skipped} {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	writeFlowFile(t, global, "main.py", "")
	writeFlowFile(t, global, "plugin.json", `{"ID":"global","Name":"Global","Language":"python","ExecuteFileName":"main.py","ActionKeyword":""}`)
	writeFlowFile(t, dotnet, "Plugin.dll", "")
	writeFlowFile(t, dotnet, "plugin.json", `{"ID":"cs","Name":"CS","Language":"csharp","ExecuteFileName":"Plugin.dll"}`)
	writeFlowFile(t, skipped, "main.py", "")
	writeFlowFile(t, skipped, "plugin.json", `{"ID":"rust","Name":"Rust","Language":"rust","ExecuteFileName":"main.py"}`)

	globalDescriptor, err := Parse(global)
	if err != nil {
		t.Fatal(err)
	}
	if len(globalDescriptor.Metadata.TriggerKeywords) != 1 || globalDescriptor.Metadata.TriggerKeywords[0] != "*" {
		t.Fatalf("keywords %#v", globalDescriptor.Metadata.TriggerKeywords)
	}
	if len(globalDescriptor.Metadata.SupportedOS) != 1 || globalDescriptor.Metadata.SupportedOS[0] != "Windows" {
		t.Fatalf("os %#v", globalDescriptor.Metadata.SupportedOS)
	}

	descriptors, err := LoadDirectory(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(descriptors) != 2 {
		t.Fatalf("loaded %d plugins", len(descriptors))
	}
	var sawDotNet bool
	for _, descriptor := range descriptors {
		if descriptor.Kind == KindDotNet {
			sawDotNet = true
			if descriptor.Metadata.Runtime != string(RuntimeDotNet) {
				t.Fatalf("dotnet runtime %s", descriptor.Metadata.Runtime)
			}
			if len(descriptor.Metadata.SupportedOS) != 1 || descriptor.Metadata.SupportedOS[0] != "Windows" {
				t.Fatalf("dotnet os %#v", descriptor.Metadata.SupportedOS)
			}
		}
	}
	if !sawDotNet {
		t.Fatal("csharp plugin was not classified as dotnet")
	}

	outside := t.TempDir()
	writeFlowFile(t, outside, "plugin.json", `{"ID":"out","Name":"Out","Language":"python","ExecuteFileName":"../main.py"}`)
	if _, err := Parse(outside); err == nil {
		t.Fatal("entry outside the plugin directory was accepted")
	}
}

func flowHasFeature(metadata plugin.Metadata, name plugin.MetadataFeatureName) bool {
	for _, feature := range metadata.Features {
		if feature.Name == name {
			return true
		}
	}
	return false
}

func writeFlowFile(t *testing.T, directory, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(directory, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}
