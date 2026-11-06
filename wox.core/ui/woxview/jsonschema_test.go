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

// TestEventSchemaMatchesWirePayload checks the event contract generated for SDKs.
func TestEventSchemaMatchesWirePayload(t *testing.T) {
	var schema struct {
		Defs map[string]struct {
			Required   []string `json:"required"`
			Properties map[string]struct {
				Type string `json:"type"`
			} `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(JSONSchema(), &schema); err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(Event{Handler: "change", Args: json.RawMessage(`["value"]`), Key: "field"})
	if err != nil {
		t.Fatal(err)
	}
	var event map[string]any
	if err := json.Unmarshal(payload, &event); err != nil {
		t.Fatal(err)
	}
	contract := schema.Defs["EventData"]
	if len(contract.Required) != len(event) || len(contract.Properties) != len(event) || contract.Properties["Args"].Type != "array" {
		t.Fatalf("event contract does not match payload: schema=%+v event=%s", contract, payload)
	}
	for _, name := range contract.Required {
		if _, ok := event[name]; !ok {
			t.Errorf("schema requires missing event field %q", name)
		}
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
