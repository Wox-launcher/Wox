package flow

import (
	"testing"

	"wox/plugin"
	"wox/plugin/thirdparty/flow/manifest"
)

func TestFlowLayerRegistration(t *testing.T) {
	if !plugin.IsReservedUserPluginDirectory(manifest.DirectoryName) {
		t.Fatal("flow collection directory was not reserved")
	}
	if manifest.Store() == nil {
		t.Fatal("missing store")
	}
	if hosts := (flowLayer{}).Hosts(); len(hosts) != 1 || hosts[0] == nil {
		t.Fatalf("hosts %#v", hosts)
	}
}
