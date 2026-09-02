package common

import "testing"

func TestFormatToolOrigin(t *testing.T) {
	cases := []struct {
		source ToolSource
		server string
		name   string
		want   string
	}{
		{source: ToolSourceBuiltin, name: "web_search", want: "builtin/web_search"},
		{source: ToolSourceMCP, server: "ddg-search", name: "search", want: "ddg-search/search"},
		{source: ToolSourceMCP, name: "search", want: "mcp/search"},
		{name: "search", want: "search"},
		{source: ToolSourceBuiltin, want: ""},
	}
	for _, testCase := range cases {
		if got := FormatToolOrigin(testCase.source, testCase.server, testCase.name); got != testCase.want {
			t.Fatalf("FormatToolOrigin(%q, %q, %q) = %q, want %q", testCase.source, testCase.server, testCase.name, got, testCase.want)
		}
	}

	info := ToolCallInfo{Name: "search", Source: ToolSourceMCP, Server: "ddg-search"}
	if got := info.OriginLabel(); got != "ddg-search/search" {
		t.Fatalf("OriginLabel = %q", got)
	}
}
