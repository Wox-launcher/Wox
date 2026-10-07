package ui

import (
	"reflect"
	"testing"

	"wox/plugin"
	"wox/ui/dto"
)

// TestPluginDTOFieldParity guards against silently dropping shared metadata fields
// when either source schema grows. Reflection stays confined to the test binary.
func TestPluginDTOFieldParity(t *testing.T) {
	metadata := plugin.Metadata{}
	manifest := plugin.StorePluginManifest{}
	for _, source := range []any{&metadata, &manifest} {
		value := reflect.ValueOf(source).Elem()
		for i := 0; i < value.NumField(); i++ {
			field := value.Field(i)
			if !field.CanSet() {
				continue
			}
			switch field.Kind() {
			case reflect.String:
				field.SetString(value.Type().Field(i).Name + "-value")
			case reflect.Bool:
				field.SetBool(true)
			case reflect.Slice:
				field.Set(reflect.MakeSlice(field.Type(), 1, 1))
			}
		}
	}
	for _, tc := range []struct {
		name   string
		source any
		got    dto.PluginDto
	}{
		{"metadata", &metadata, pluginMetadataDTO(&metadata)},
		{"store", &manifest, storeManifestDTOs([]plugin.StorePluginManifest{manifest})[0]},
	} {
		t.Run(tc.name, func(t *testing.T) {
			source, target := reflect.ValueOf(tc.source).Elem(), reflect.ValueOf(tc.got)
			for i := 0; i < target.NumField(); i++ {
				name := target.Type().Field(i).Name
				from, to := source.FieldByName(name), target.Field(i)
				if !from.IsValid() || !from.Type().ConvertibleTo(to.Type()) {
					continue
				}
				if !reflect.DeepEqual(from.Convert(to.Type()).Interface(), to.Interface()) {
					t.Errorf("field %s was not preserved", name)
				}
				if from.Kind() == reflect.Slice && from.Pointer() != to.Pointer() {
					t.Errorf("field %s must retain its shared backing array", name)
				}
			}
		})
	}
}

// TestStoreManifestDTOEmptyValues preserves JSON-visible nil versus empty slices.
func TestStoreManifestDTOEmptyValues(t *testing.T) {
	for _, manifests := range [][]plugin.StorePluginManifest{nil, {}} {
		got := storeManifestDTOs(manifests)
		if got == nil || len(got) != 0 {
			t.Fatalf("empty catalog = %#v", got)
		}
	}
	for _, values := range [][]string{nil, {}} {
		got := storeManifestDTOs([]plugin.StorePluginManifest{{ScreenshotUrls: values, SupportedOS: values}})[0]
		if !reflect.DeepEqual(got.ScreenshotUrls, values) || !reflect.DeepEqual(got.SupportedOS, values) {
			t.Fatalf("store slice values changed: %#v", got)
		}
		metadata := plugin.Metadata{SupportedOS: values, TriggerKeywords: values}
		installed := pluginMetadataDTO(&metadata)
		if !reflect.DeepEqual(installed.SupportedOS, values) || !reflect.DeepEqual(installed.TriggerKeywords, values) {
			t.Fatalf("metadata slice values changed: %#v", installed)
		}
	}
}
