package ai

import (
	"os"
	"path/filepath"
	"testing"

	"wox/common"
)

func TestResolveMCPCommandPrefersArgs(t *testing.T) {
	command, args := resolveMCPCommand(common.AIChatMCPServerConfig{
		Command: "npx",
		Args:    []string{"-y", "pkg", "--path", "/Program Files/foo"},
	})
	if command != "npx" || len(args) != 4 || args[3] != "/Program Files/foo" {
		t.Fatalf("command = %q args = %#v", command, args)
	}
}

func TestResolveMCPCommandFallsBackToLegacyString(t *testing.T) {
	command, args := resolveMCPCommand(common.AIChatMCPServerConfig{Command: "uvx duckduckgo-mcp-server"})
	if command != "uvx" || len(args) != 1 || args[0] != "duckduckgo-mcp-server" {
		t.Fatalf("command = %q args = %#v", command, args)
	}
}

func TestInterpolateMCPValue(t *testing.T) {
	t.Setenv("MCP_TOKEN", "secret")
	got := interpolateMCPValue("Bearer ${env:MCP_TOKEN}")
	if got != "Bearer secret" {
		t.Fatalf("interpolated = %q", got)
	}
	got = interpolateMCPValue("x=${MISSING:-fallback}")
	if got != "x=fallback" {
		t.Fatalf("fallback = %q", got)
	}
	home, _ := os.UserHomeDir()
	if got := interpolateMCPValue("${userHome}/bin"); got != filepath.Join(home, "bin") && got != home+"/bin" {
		if !filepath.IsAbs(got) || !containsHome(got, home) {
			t.Fatalf("userHome = %q home = %q", got, home)
		}
	}
}

func containsHome(value, home string) bool {
	return len(home) > 0 && len(value) >= len(home) && value[:len(home)] == home
}

func TestLoadMCPEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("FOO=bar\n# comment\nexport BAZ=\"qux\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	env := loadMCPEnvFile(path)
	if env["FOO"] != "bar" || env["BAZ"] != "qux" {
		t.Fatalf("env file = %#v", env)
	}
}

func TestCachedMCPToolNames(t *testing.T) {
	server := "ddg-search-cache-test"
	if _, ok := CachedMCPToolNames(server); ok {
		t.Fatal("missing server should not be cached")
	}
	SetCachedMCPToolsForTest(server, []common.MCPTool{{Name: "search"}, {Name: "fetch"}})
	names, ok := CachedMCPToolNames(server)
	if !ok || len(names) != 2 || names[0] != "search" || names[1] != "fetch" {
		t.Fatalf("cached names = %#v ok=%v", names, ok)
	}
	ResetMCPClients()
	names, ok = CachedMCPToolNames(server)
	if !ok || len(names) != 2 {
		t.Fatalf("names should survive session reset: %#v ok=%v", names, ok)
	}
}

func TestFilterMCPServerTools(t *testing.T) {
	tools := []common.MCPTool{{Name: "read"}, {Name: "write"}, {Name: "delete"}}
	filtered := FilterMCPServerTools(tools, common.AIChatMCPServerConfig{
		EnabledTools:  []string{"read", "write", "delete"},
		DisabledTools: []string{"delete"},
	})
	if len(filtered) != 2 || filtered[0].Name != "read" || filtered[1].Name != "write" {
		t.Fatalf("filtered = %#v", filtered)
	}
	allowed := FilterMCPServerTools(tools, common.AIChatMCPServerConfig{EnabledTools: []string{"read"}})
	if len(allowed) != 1 || allowed[0].Name != "read" {
		t.Fatalf("allowed = %#v", allowed)
	}
}

func TestResolveMCPHeadersBearerAndStatic(t *testing.T) {
	t.Setenv("MCP_TOKEN", "abc")
	headers := resolveMCPHeaders(t.Context(), common.AIChatMCPServerConfig{
		Headers:           map[string]string{"X-Key": "1"},
		BearerTokenEnvVar: "MCP_TOKEN",
	})
	if headers["X-Key"] != "1" || headers["Authorization"] != "Bearer abc" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestMcpStartupTimeoutUsesAuthDefault(t *testing.T) {
	if got := mcpStartupTimeout(common.AIChatMCPServerConfig{}); got != defaultMCPStartupTimeout {
		t.Fatalf("default timeout = %s", got)
	}
	if got := mcpStartupTimeout(common.AIChatMCPServerConfig{Auth: &common.AIChatMCPServerAuth{ClientID: "id"}}); got != defaultMCPAuthTimeout {
		t.Fatalf("auth timeout = %s", got)
	}
}
