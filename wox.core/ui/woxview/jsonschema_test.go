package woxview

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"
)

func TestJSONSchemaGolden(t *testing.T) {
	got := JSONSchema()
	if !json.Valid(got) {
		t.Fatal("invalid generated schema")
	}
	const path = "testdata/woxview.schema.json"
	if os.Getenv("WOX_UPDATE_SCHEMA") == "1" {
		if err := os.MkdirAll("testdata", 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("schema changed; run WOX_UPDATE_SCHEMA=1 go test ./ui/woxview -run TestJSONSchemaGolden from wox.core")
	}
}

func TestParseRejectsInvalidFieldContracts(t *testing.T) {
	for _, props := range []string{`"revision":-1`, `"revision":0.5`, `"revision":18446744073709551616`, `"error":true`, `"required":"yes"`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"text-field","props":{` + props + `}}}`)); err == nil {
			t.Fatalf("accepted %s", props)
		}
	}
	for _, props := range []string{`"disabled":"yes"`, `"submitting":1`} {
		if _, err := Parse([]byte(`{"version":1,"root":{"type":"form","props":{` + props + `}}}`)); err == nil {
			t.Fatalf("accepted %s", props)
		}
	}
}
