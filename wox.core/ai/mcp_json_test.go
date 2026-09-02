package ai

import (
	"strings"
	"testing"

	"wox/common"
)

func TestParseMCPServersJSONClaudeFormat(t *testing.T) {
	servers, err := ParseMCPServersJSON(`{
		"mcpServers": {
			"ddg-search": {
				"command": "uvx",
				"args": ["duckduckgo-mcp-server"],
				"env": {"FOO": "bar"}
			}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 {
		t.Fatalf("server count = %d, want 1", len(servers))
	}
	server := servers[0]
	if server.Name != "ddg-search" || server.Type != common.AIChatMCPServerTypeSTDIO {
		t.Fatalf("server identity = %#v", server)
	}
	if server.Command != "uvx" || len(server.Args) != 1 || server.Args[0] != "duckduckgo-mcp-server" {
		t.Fatalf("command = %q args = %#v", server.Command, server.Args)
	}
	if len(server.EnvironmentVariables) != 1 || server.EnvironmentVariables[0] != "FOO=bar" {
		t.Fatalf("env = %#v", server.EnvironmentVariables)
	}
}

func TestParseMCPServersJSONHTTPAndBareObject(t *testing.T) {
	servers, err := ParseMCPServersJSON(`{
		"docs": {"url": "https://example.com/mcp", "headers": {"Authorization": "Bearer tok"}}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 1 || servers[0].Name != "docs" || servers[0].Type != common.AIChatMCPServerTypeStreamableHTTP {
		t.Fatalf("http server = %#v", servers)
	}
	if servers[0].Url != "https://example.com/mcp" {
		t.Fatalf("url = %q", servers[0].Url)
	}
	if servers[0].Headers["Authorization"] != "Bearer tok" {
		t.Fatalf("headers = %#v", servers[0].Headers)
	}
}

func TestParseMCPServersJSONAdvancedFields(t *testing.T) {
	servers, err := ParseMCPServersJSON(`{
		"mcpServers": {
			"remote": {
				"type": "http",
				"url": "https://api.example.com/mcp",
				"headers": {"X-Key": "1"},
				"auth": {"CLIENT_ID": "id", "CLIENT_SECRET": "secret", "scopes": ["read"]},
				"enabledTools": ["search"],
				"disabled_tools": ["delete"],
				"startup_timeout_sec": 20,
				"toolTimeoutSec": 45,
				"bearer_token_env_var": "MCP_TOKEN"
			},
			"local": {
				"command": "npx",
				"args": ["-y", "pkg", "--path", "/Program Files/foo"],
				"cwd": "/tmp/work",
				"envFile": ".env",
				"env_vars": ["HOME"],
				"disabled": true
			}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 2 {
		t.Fatalf("server count = %d, want 2", len(servers))
	}
	local, remote := servers[0], servers[1]
	if local.Name != "local" || remote.Name != "remote" {
		t.Fatalf("order = %q %q", local.Name, remote.Name)
	}
	if !local.Disabled || local.WorkingDirectory != "/tmp/work" || local.EnvFile != ".env" || len(local.Args) != 4 {
		t.Fatalf("local = %#v", local)
	}
	if remote.Auth == nil || remote.Auth.ClientID != "id" || remote.EnabledTools[0] != "search" || remote.DisabledTools[0] != "delete" {
		t.Fatalf("remote = %#v", remote)
	}
	if remote.StartupTimeoutSec != 20 || remote.ToolTimeoutSec != 45 || remote.BearerTokenEnvVar != "MCP_TOKEN" {
		t.Fatalf("remote timeouts = %#v", remote)
	}
}

func TestParseMCPServersDocumentAllowsEmpty(t *testing.T) {
	servers, err := ParseMCPServersDocument(`{"mcpServers":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(servers) != 0 {
		t.Fatalf("empty document = %#v", servers)
	}
	if _, err := ParseMCPServersJSON(`{"mcpServers":{}}`); err == nil {
		t.Fatal("empty import should still fail")
	}
}

func TestParseMCPServersJSONRejectsInvalid(t *testing.T) {
	if _, err := ParseMCPServersJSON(""); err == nil {
		t.Fatal("empty JSON should fail")
	}
	if _, err := ParseMCPServersJSON("{"); err == nil {
		t.Fatal("invalid JSON should fail")
	}
	if _, err := ParseMCPServersJSON(`{"mcpServers":{"broken":{}}}`); err == nil {
		t.Fatal("server without command or url should fail")
	}
}

func TestFormatMCPServersJSONRoundTrip(t *testing.T) {
	original := []common.AIChatMCPServerConfig{
		{
			Name: "docs", Type: common.AIChatMCPServerTypeStreamableHTTP, Url: "https://example.com/mcp",
			Headers: map[string]string{"Authorization": "Bearer tok"}, EnabledTools: []string{"read"},
		},
		{
			Name: "local", Type: common.AIChatMCPServerTypeSTDIO, Command: "npx",
			Args: []string{"-y", "pkg"}, WorkingDirectory: "/tmp", EnvironmentVariables: []string{"FOO=bar"},
		},
	}
	raw, err := FormatMCPServersJSON(original)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"mcpServers"`) || !strings.Contains(raw, `"args"`) || !strings.Contains(raw, `"headers"`) {
		t.Fatalf("formatted JSON missing expected keys: %s", raw)
	}
	parsed, err := ParseMCPServersJSON(raw)
	if err != nil {
		t.Fatal(err)
	}
	if len(parsed) != 2 || parsed[0].Name != "docs" || parsed[1].Args[1] != "pkg" || parsed[0].Headers["Authorization"] != "Bearer tok" {
		t.Fatalf("round trip = %#v", parsed)
	}
}

func TestFormatMCPServersJSONOmitsEmptyFields(t *testing.T) {
	raw, err := FormatMCPServersJSON([]common.AIChatMCPServerConfig{{
		Name: "local", Type: common.AIChatMCPServerTypeSTDIO, Command: "uvx", Args: []string{"pkg"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, noise := range []string{`null`, `""`, "bearer_token_env_var", "headersHelper", "enabled", "auth", "envFile"} {
		if strings.Contains(raw, noise) {
			t.Fatalf("formatted JSON should omit empty field %q: %s", noise, raw)
		}
	}
	if !strings.Contains(raw, `"command": "uvx"`) || !strings.Contains(raw, `"type": "stdio"`) {
		t.Fatalf("formatted JSON missing required fields: %s", raw)
	}
}

func TestFormatMCPServersJSONSplitsLegacyCommand(t *testing.T) {
	raw, err := FormatMCPServersJSON([]common.AIChatMCPServerConfig{{
		Name: "ddg-search", Type: common.AIChatMCPServerTypeSTDIO, Command: "uvx duckduckgo-mcp-server",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"command": "uvx"`) || !strings.Contains(raw, `"duckduckgo-mcp-server"`) {
		t.Fatalf("legacy command was not split: %s", raw)
	}
}

func TestUpsertMCPServersReplacesByName(t *testing.T) {
	existing := []common.AIChatMCPServerConfig{
		{Name: "ddg-search", Type: common.AIChatMCPServerTypeSTDIO, Command: "old"},
		{Name: "keep", Type: common.AIChatMCPServerTypeSTDIO, Command: "keep"},
	}
	incoming := []common.AIChatMCPServerConfig{
		{Name: "ddg-search", Type: common.AIChatMCPServerTypeSTDIO, Command: "uvx", Args: []string{"duckduckgo-mcp-server"}},
		{Name: "docs", Type: common.AIChatMCPServerTypeStreamableHTTP, Url: "https://example.com/mcp"},
	}
	got := UpsertMCPServers(existing, incoming)
	if len(got) != 3 {
		t.Fatalf("merged count = %d, want 3", len(got))
	}
	if got[0].Command != "uvx" || got[1].Name != "keep" || got[2].Name != "docs" {
		t.Fatalf("merged = %#v", got)
	}
}
