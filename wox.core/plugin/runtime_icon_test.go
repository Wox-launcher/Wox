package plugin

import (
	"context"
	"testing"

	"wox/common"
)

type iconHost struct {
	runtime Runtime
	icon    common.WoxImage
}

func (h iconHost) GetRuntime(context.Context) Runtime { return h.runtime }
func (h iconHost) Start(context.Context) error        { return nil }
func (h iconHost) Stop(context.Context)               {}
func (h iconHost) IsStarted(context.Context) bool     { return false }
func (h iconHost) RuntimeStatus(context.Context) RuntimeHostStatus {
	return RuntimeHostStatus{}
}
func (h iconHost) LoadPlugin(context.Context, Metadata, string) (Plugin, error) {
	return nil, nil
}
func (h iconHost) UnloadPlugin(context.Context, Metadata) {}
func (h iconHost) Icon(context.Context) common.WoxImage   { return h.icon }

func TestHostRuntimeIconUsesTheHostMark(t *testing.T) {
	previous := AllHosts
	t.Cleanup(func() { AllHosts = previous })
	mark := common.NewWoxImageSvg(`<svg xmlns="http://www.w3.org/2000/svg"/>`)
	AllHosts = []Host{iconHost{runtime: "EXAMPLE", icon: mark}}

	icon, ok := HostRuntimeIcon(context.Background(), "example")
	if !ok || icon.ImageData != mark.ImageData {
		t.Fatalf("runtime icon = %+v, ok %t", icon, ok)
	}
	if _, ok := HostRuntimeIcon(context.Background(), "nodejs"); ok {
		t.Fatal("a runtime without a host mark was reported as present")
	}
}
