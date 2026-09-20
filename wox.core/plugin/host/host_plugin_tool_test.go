package host

import (
	"context"
	"testing"
	"time"
	"wox/plugin"
)

func TestPluginToolParentContextIsBoundToLiveHostCall(t *testing.T) {
	w := &WebsocketHost{}
	type key struct{}
	parent, cancel := context.WithTimeout(context.WithValue(t.Context(), key{}, "call chain"), time.Minute)
	defer cancel()
	w.pluginToolCalls.Store("token", hostPluginToolCall{ctx: parent, pluginID: "owner"})
	ctx, err := w.pluginToolCallContext(t.Context(), "owner", "token")
	if err != nil || ctx.Value(key{}) != "call chain" {
		t.Fatalf("parent state not restored: %v", err)
	}
	want, _ := parent.Deadline()
	if got, _ := ctx.Deadline(); got != want {
		t.Fatalf("deadline = %v, want %v", got, want)
	}
	for _, owner := range []string{"different-plugin", "owner"} {
		if owner == "owner" {
			w.pluginToolCalls.Delete("token")
		}
		if _, err := w.pluginToolCallContext(t.Context(), owner, "token"); err == nil || err.Code != plugin.PluginToolErrorPermissionDenied {
			t.Fatalf("foreign/expired token accepted for %s", owner)
		}
	}
	w.pluginToolCalls.Store("token", hostPluginToolCall{ctx: parent, pluginID: "owner"})
	cancel()
	if _, err := w.pluginToolCallContext(t.Context(), "owner", "token"); err == nil || err.Code != plugin.PluginToolErrorCancelled {
		t.Fatalf("cancelled parent accepted: %v", err)
	}
	if _, err := (&WebsocketHost{}).pluginToolCallContext(t.Context(), "owner", "token"); err == nil {
		t.Fatal("token accepted by a different host")
	}
}
