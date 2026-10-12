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
		for _, runtime := range []plugin.Runtime{manifest.RuntimeJSONRPC, manifest.RuntimeDotNet} {
			host := registeredHost(runtime)
			if host == nil || plugin.RuntimeSettingsVisible(host) {
				t.Fatalf("%s must stay off the runtime settings page", runtime)
			}
		}
	} else if scriptOK || dotnetOK {
		t.Fatal("flow hosts must stay unregistered outside Windows")
	}
}

func registeredHost(runtime plugin.Runtime) plugin.Host {
	for _, host := range plugin.AllHosts {
		if host != nil && host.GetRuntime(context.Background()) == runtime {
			return host
		}
	}
	return nil
}
