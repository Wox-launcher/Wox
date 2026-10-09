package migrate

import (
	"context"
	"testing"

	"wox/common"
)

type testSource struct {
	id           string
	installation Installation
	err          error
}

func (s testSource) ID() string { return s.id }

func (s testSource) Detect(context.Context) (Installation, error) {
	return s.installation, s.err
}

type testInstallation struct {
	id   string
	name string
}

func (i testInstallation) ID() string            { return i.id }
func (i testInstallation) Name() string          { return i.name }
func (i testInstallation) Version() string       { return "" }
func (i testInstallation) Location() string      { return "" }
func (i testInstallation) Icon() common.WoxImage { return common.WoxImage{} }
func (i testInstallation) Hotkey() string        { return "" }
func (i testInstallation) Plugins(context.Context) ([]Plugin, error) {
	return nil, nil
}
func (i testInstallation) Catalog(context.Context) ([]Category, error) {
	return nil, nil
}
func (i testInstallation) Import(context.Context, []string) (ImportResult, error) {
	return ImportResult{}, nil
}

func TestDetectSkipsAbsentAndFailingSources(t *testing.T) {
	registryMu.Lock()
	previous := registry
	registry = nil
	registryMu.Unlock()
	t.Cleanup(func() {
		registryMu.Lock()
		registry = previous
		registryMu.Unlock()
	})

	Register(testSource{id: "missing"})
	Register(testSource{id: "broken", err: context.Canceled})
	Register(testSource{id: "flow", installation: testInstallation{id: "flow", name: "Flow"}})
	Register(testSource{id: "flow", installation: testInstallation{id: "duplicate"}})
	Register(nil)

	found, err := Detect(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].ID() != "flow" || found[0].Name() != "Flow" {
		t.Fatalf("found %#v", found)
	}
}
