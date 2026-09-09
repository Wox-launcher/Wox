package util

import (
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestIsPluginPackagePath(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "plugin.wox", want: true},
		{path: "plugin.WOX", want: true},
		{path: `C:\Downloads\demo.wox`, want: true},
		{path: "/tmp/demo.wox", want: true},
		{path: "plugin.txt", want: false},
		{path: "wox", want: false},
		{path: "", want: false},
	}
	for _, test := range tests {
		if got := IsPluginPackagePath(test.path); got != test.want {
			t.Fatalf("IsPluginPackagePath(%q) = %t, want %t", test.path, got, test.want)
		}
	}
}

func TestCollectStartupDeepLinksConvertsPluginPackages(t *testing.T) {
	fileName := "demo.wox"
	abs, err := filepath.Abs(fileName)
	if err != nil {
		t.Fatal(err)
	}

	got := CollectStartupDeepLinks([]string{
		"--updated",
		"wox://query?q=wpm",
		fileName,
		"notes.txt",
		"wox://query?q=wpm",
	})
	wantLink := PluginPackageInstallDeepLink(abs)
	if len(got) != 2 {
		t.Fatalf("links = %v, want 2 entries", got)
	}
	if got[0] != "wox://query?q=wpm" {
		t.Fatalf("first link = %q, want query deeplink", got[0])
	}
	if got[1] != wantLink {
		t.Fatalf("second link = %q, want %q", got[1], wantLink)
	}
}

func TestPluginPackagePathFromArgAcceptsFileURL(t *testing.T) {
	if runtime.GOOS == "windows" {
		got, ok := PluginPackagePathFromArg("file:///C:/Users/demo/plugin.wox")
		if !ok {
			t.Fatal("expected Windows file URL to be accepted")
		}
		if !strings.EqualFold(got, `C:\Users\demo\plugin.wox`) {
			t.Fatalf("path = %q, want C:\\Users\\demo\\plugin.wox", got)
		}
		return
	}

	got, ok := PluginPackagePathFromArg("file:///tmp/My%20Plugin.wox")
	if !ok {
		t.Fatal("expected file URL to be accepted")
	}
	if got != "/tmp/My Plugin.wox" {
		t.Fatalf("path = %q, want /tmp/My Plugin.wox", got)
	}
}

func TestPluginPackageInstallDeepLinkEscapesPath(t *testing.T) {
	link := PluginPackageInstallDeepLink(`/tmp/My Plugin.wox`)
	if link != "wox://install?path="+url.QueryEscape(`/tmp/My Plugin.wox`) {
		t.Fatalf("deeplink = %q", link)
	}
}
