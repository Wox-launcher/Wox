package plugin

import (
	"context"
	"strings"
	"testing"

	"wox/util"
)

type osLayer struct {
	name string
	os   []string
	host Host
}

func (l osLayer) Name() string          { return l.name }
func (l osLayer) Directory() string     { return l.name }
func (l osLayer) Hosts() []Host         { return []Host{l.host} }
func (l osLayer) Store() ExternalStore  { return nil }
func (l osLayer) SupportedOS() []string { return l.os }

func TestRegisterThirdPartySkipsHostsOutsideSupportedOS(t *testing.T) {
	previousHosts := AllHosts
	previousStores := append([]ExternalStore(nil), externalStores...)
	t.Cleanup(func() {
		AllHosts = previousHosts
		externalStores = previousStores
		delete(reservedUserPluginDirectories, "os-limited")
		delete(reservedUserPluginDirectories, "os-any")
	})

	RegisterThirdParty(osLayer{name: "os-limited", os: []string{"Windows"}, host: iconHost{runtime: "OSLIMITED"}})
	RegisterThirdParty(osLayer{name: "os-any", host: iconHost{runtime: "OSANY"}})

	if !isRegisteredReservedPluginDirectory("os-limited") || !isRegisteredReservedPluginDirectory("os-any") {
		t.Fatal("collection directories must stay reserved when a layer is registered")
	}
	if !hostRuntimeRegistered("OSANY") {
		t.Fatal("a layer with no OS limit must register its host")
	}
	limited := hostRuntimeRegistered("OSLIMITED")
	if util.IsWindows() {
		if !limited {
			t.Fatal("windows-only layer must register its host on Windows")
		}
		return
	}
	if limited {
		t.Fatal("windows-only layer must not register its host on this system")
	}
}

func hostRuntimeRegistered(runtime string) bool {
	for _, host := range AllHosts {
		if host != nil && strings.EqualFold(string(host.GetRuntime(context.Background())), runtime) {
			return true
		}
	}
	return false
}
