package flow

import (
	"context"
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
	hosts := (flowLayer{}).Hosts()
	if len(hosts) != 2 || hosts[0] == nil || hosts[1] == nil {
		t.Fatalf("hosts %#v", hosts)
	}
	if hosts[0].GetRuntime(context.Background()) != manifest.RuntimeJSONRPC || hosts[1].GetRuntime(context.Background()) != manifest.RuntimeDotNet {
		t.Fatalf("runtimes %s %s", hosts[0].GetRuntime(context.Background()), hosts[1].GetRuntime(context.Background()))
	}
	scriptIcon, scriptOK := plugin.HostRuntimeIcon(context.Background(), string(manifest.RuntimeJSONRPC))
	dotnetIcon, dotnetOK := plugin.HostRuntimeIcon(context.Background(), string(manifest.RuntimeDotNet))
	if !scriptOK || !dotnetOK || scriptIcon.IsEmpty() || scriptIcon.ImageData != dotnetIcon.ImageData {
		t.Fatalf("flow runtime icons script=%+v dotnet=%+v", scriptIcon, dotnetIcon)
	}
}
