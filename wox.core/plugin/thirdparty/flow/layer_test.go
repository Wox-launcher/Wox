package flow

import (
	"context"
	"testing"

	"wox/plugin"
	"wox/plugin/thirdparty/flow/brand"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/util"
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
	supported := (flowLayer{}).SupportedOS()
	if len(supported) != 1 || supported[0] != "Windows" {
		t.Fatalf("supported os %#v", supported)
	}
	mark := brand.Image()
	scriptIcon, scriptOK := plugin.HostRuntimeIcon(context.Background(), string(manifest.RuntimeJSONRPC))
	dotnetIcon, dotnetOK := plugin.HostRuntimeIcon(context.Background(), string(manifest.RuntimeDotNet))
	if util.IsWindows() {
		if !scriptOK || !dotnetOK || scriptIcon.ImageData != mark.ImageData || dotnetIcon.ImageData != mark.ImageData {
			t.Fatalf("flow runtime icons script=%+v dotnet=%+v", scriptIcon, dotnetIcon)
		}
	} else if scriptOK || dotnetOK {
		t.Fatal("flow hosts must stay unregistered outside Windows")
	}
	label, icon := plugin.PluginStorePresentation("flow")
	if label != "i18n:ui_plugin_store_flow" || icon.IsEmpty() || icon.ImageData != mark.ImageData {
		t.Fatalf("flow store presentation = %q icon=%+v", label, icon)
	}
}
