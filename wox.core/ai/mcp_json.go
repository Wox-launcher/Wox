package ai

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"wox/common"
)

type claudeMCPConfig struct {
	MCPServers map[string]claudeMCPServer `json:"mcpServers"`
}

// claudeMCPServer is the shared Cursor / Claude Desktop / Claude Code server object.
type claudeMCPServer struct {
	Command           string            `json:"command,omitempty"`
	Args              []string          `json:"args,omitempty"`
	Env               map[string]string `json:"env,omitempty"`
	EnvFile           string            `json:"envFile,omitempty"`
	EnvFileSnake      string            `json:"env_file,omitempty"`
	Cwd               string            `json:"cwd,omitempty"`
	EnvVars           []string          `json:"envVars,omitempty"`
	EnvVarsSnake      []string          `json:"env_vars,omitempty"`
	URL               string            `json:"url,omitempty"`
	Headers           map[string]string `json:"headers,omitempty"`
	HTTPHeaders       map[string]string `json:"http_headers,omitempty"`
	HeadersHelper     string            `json:"headersHelper,omitempty"`
	BearerTokenEnvVar string            `json:"bearerTokenEnvVar,omitempty"`
	BearerTokenSnake  string            `json:"bearer_token_env_var,omitempty"`
	Auth              *claudeMCPAuth    `json:"auth,omitempty"`
	Type              string            `json:"type,omitempty"`
	Disabled          *bool             `json:"disabled,omitempty"`
	Enabled           *bool             `json:"enabled,omitempty"`
	EnabledTools      []string          `json:"enabledTools,omitempty"`
	EnabledToolsSnake []string          `json:"enabled_tools,omitempty"`
	AllowedTools      []string          `json:"allowedTools,omitempty"`
	DisabledTools     []string          `json:"disabledTools,omitempty"`
	DisabledToolsSnk  []string          `json:"disabled_tools,omitempty"`
	DeniedTools       []string          `json:"deniedTools,omitempty"`
	StartupTimeout    *float64          `json:"startupTimeoutSec,omitempty"`
	StartupTimeoutSnk *float64          `json:"startup_timeout_sec,omitempty"`
	ToolTimeout       *float64          `json:"toolTimeoutSec,omitempty"`
	ToolTimeoutSnake  *float64          `json:"tool_timeout_sec,omitempty"`
	Timeout           *float64          `json:"timeout,omitempty"`
}

type claudeMCPAuth struct {
	ClientID     string   `json:"CLIENT_ID,omitempty"`
	ClientSecret string   `json:"CLIENT_SECRET,omitempty"`
	Scopes       []string `json:"scopes,omitempty"`
}

// ParseMCPServersJSON accepts Claude Desktop / Cursor mcpServers JSON and maps
// it onto Wox MCP server configs. A bare servers object without the
// mcpServers wrapper is also accepted. An empty server map is rejected.
func ParseMCPServersJSON(raw string) ([]common.AIChatMCPServerConfig, error) {
	return parseMCPServersJSON(raw, false)
}

// ParseMCPServersDocument accepts the same JSON as ParseMCPServersJSON but
// allows an empty mcpServers object so the settings editor can clear all rows.
func ParseMCPServersDocument(raw string) ([]common.AIChatMCPServerConfig, error) {
	return parseMCPServersJSON(raw, true)
}

func parseMCPServersJSON(raw string, allowEmpty bool) ([]common.AIChatMCPServerConfig, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("MCP JSON is empty")
	}

	var wrapped claudeMCPConfig
	if err := json.Unmarshal([]byte(raw), &wrapped); err != nil {
		return nil, fmt.Errorf("invalid MCP JSON: %w", err)
	}
	servers := wrapped.MCPServers
	if servers == nil {
		if err := json.Unmarshal([]byte(raw), &servers); err != nil {
			return nil, fmt.Errorf("MCP JSON does not contain any servers")
		}
	}

	names := make([]string, 0, len(servers))
	for name := range servers {
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]common.AIChatMCPServerConfig, 0, len(names))
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		config, err := claudeServerToConfig(name, servers[name])
		if err != nil {
			return nil, err
		}
		out = append(out, config)
	}
	if len(out) == 0 && !allowEmpty {
		return nil, fmt.Errorf("MCP JSON does not contain any servers")
	}
	return out, nil
}

// FormatMCPServersJSON writes Wox MCP configs as a Claude/Cursor mcpServers document.
func FormatMCPServersJSON(configs []common.AIChatMCPServerConfig) (string, error) {
	servers := make(map[string]claudeMCPServer, len(configs))
	for _, config := range configs {
		name := strings.TrimSpace(config.Name)
		if name == "" {
			continue
		}
		servers[name] = configToClaudeServer(config)
	}
	encoded, err := json.MarshalIndent(claudeMCPConfig{MCPServers: servers}, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

// UpsertMCPServers replaces existing servers with the same name and appends new ones.
func UpsertMCPServers(existing, incoming []common.AIChatMCPServerConfig) []common.AIChatMCPServerConfig {
	if len(incoming) == 0 {
		return append([]common.AIChatMCPServerConfig(nil), existing...)
	}
	indexByName := make(map[string]int, len(existing))
	out := append([]common.AIChatMCPServerConfig(nil), existing...)
	for index, server := range out {
		indexByName[server.Name] = index
	}
	for _, server := range incoming {
		if index, ok := indexByName[server.Name]; ok {
			out[index] = server
			continue
		}
		indexByName[server.Name] = len(out)
		out = append(out, server)
	}
	return out
}

func claudeServerToConfig(name string, server claudeMCPServer) (common.AIChatMCPServerConfig, error) {
	config := common.AIChatMCPServerConfig{
		Name:              name,
		Args:              trimStringList(server.Args),
		WorkingDirectory:  firstNonEmpty(server.Cwd),
		EnvFile:           firstNonEmpty(server.EnvFile, server.EnvFileSnake),
		EnvVars:           firstStringList(server.EnvVars, server.EnvVarsSnake),
		Headers:           firstStringMap(server.Headers, server.HTTPHeaders),
		HeadersHelper:     strings.TrimSpace(server.HeadersHelper),
		BearerTokenEnvVar: firstNonEmpty(server.BearerTokenEnvVar, server.BearerTokenSnake),
		EnabledTools:      firstStringList(server.EnabledTools, server.EnabledToolsSnake, server.AllowedTools),
		DisabledTools:     firstStringList(server.DisabledTools, server.DisabledToolsSnk, server.DeniedTools),
	}
	if server.Disabled != nil {
		config.Disabled = *server.Disabled
	} else if server.Enabled != nil {
		config.Disabled = !*server.Enabled
	}
	if envNames := sortedEnvKeys(server.Env); len(envNames) > 0 {
		config.EnvironmentVariables = make([]string, 0, len(envNames))
		for _, key := range envNames {
			config.EnvironmentVariables = append(config.EnvironmentVariables, key+"="+server.Env[key])
		}
	}
	if server.Auth != nil && (strings.TrimSpace(server.Auth.ClientID) != "" || strings.TrimSpace(server.Auth.ClientSecret) != "" || len(server.Auth.Scopes) > 0) {
		config.Auth = &common.AIChatMCPServerAuth{
			ClientID:     strings.TrimSpace(server.Auth.ClientID),
			ClientSecret: strings.TrimSpace(server.Auth.ClientSecret),
			Scopes:       trimStringList(server.Auth.Scopes),
		}
	}
	if timeout := firstFloat(server.StartupTimeout, server.StartupTimeoutSnk, server.Timeout); timeout > 0 {
		config.StartupTimeoutSec = timeout
	}
	if timeout := firstFloat(server.ToolTimeout, server.ToolTimeoutSnake); timeout > 0 {
		config.ToolTimeoutSec = timeout
	}

	url := strings.TrimSpace(server.URL)
	command := strings.TrimSpace(server.Command)
	switch normalizeMCPServerType(server.Type) {
	case common.AIChatMCPServerTypeStreamableHTTP:
		if url == "" {
			return common.AIChatMCPServerConfig{}, fmt.Errorf("MCP server %q needs a url", name)
		}
		config.Type = common.AIChatMCPServerTypeStreamableHTTP
		config.Url = url
		return config, nil
	case common.AIChatMCPServerTypeSTDIO:
		if command == "" {
			return common.AIChatMCPServerConfig{}, fmt.Errorf("MCP server %q needs a command", name)
		}
		config.Type = common.AIChatMCPServerTypeSTDIO
		config.Command = command
		return config, nil
	}
	if url != "" {
		config.Type = common.AIChatMCPServerTypeStreamableHTTP
		config.Url = url
		return config, nil
	}
	if command != "" {
		config.Type = common.AIChatMCPServerTypeSTDIO
		config.Command = command
		return config, nil
	}
	return common.AIChatMCPServerConfig{}, fmt.Errorf("MCP server %q needs a command or url", name)
}

func configToClaudeServer(config common.AIChatMCPServerConfig) claudeMCPServer {
	command, args := exportMCPCommand(config)
	server := claudeMCPServer{
		Command:           command,
		Args:              args,
		Env:               envListToMap(config.EnvironmentVariables),
		EnvFile:           strings.TrimSpace(config.EnvFile),
		Cwd:               strings.TrimSpace(config.WorkingDirectory),
		EnvVars:           trimStringList(config.EnvVars),
		URL:               strings.TrimSpace(config.Url),
		Headers:           cloneStringMap(config.Headers),
		HeadersHelper:     strings.TrimSpace(config.HeadersHelper),
		BearerTokenEnvVar: strings.TrimSpace(config.BearerTokenEnvVar),
		EnabledTools:      trimStringList(config.EnabledTools),
		DisabledTools:     trimStringList(config.DisabledTools),
	}
	if config.Disabled {
		disabled := true
		server.Disabled = &disabled
	}
	if config.Type != "" {
		if config.Type == common.AIChatMCPServerTypeStreamableHTTP {
			server.Type = "http"
		} else {
			server.Type = string(config.Type)
		}
	}
	if config.Auth != nil && (config.Auth.ClientID != "" || config.Auth.ClientSecret != "" || len(config.Auth.Scopes) > 0) {
		server.Auth = &claudeMCPAuth{
			ClientID:     config.Auth.ClientID,
			ClientSecret: config.Auth.ClientSecret,
			Scopes:       trimStringList(config.Auth.Scopes),
		}
	}
	if config.StartupTimeoutSec > 0 {
		timeout := config.StartupTimeoutSec
		server.StartupTimeout = &timeout
	}
	if config.ToolTimeoutSec > 0 {
		timeout := config.ToolTimeoutSec
		server.ToolTimeout = &timeout
	}
	return server
}

func normalizeMCPServerType(raw string) common.AIChatMCPServerType {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "stdio":
		return common.AIChatMCPServerTypeSTDIO
	case "http", "streamable-http", "streamable_http", "streamablehttp":
		return common.AIChatMCPServerTypeStreamableHTTP
	default:
		return ""
	}
}

func exportMCPCommand(config common.AIChatMCPServerConfig) (string, []string) {
	command := strings.TrimSpace(config.Command)
	if args := trimStringList(config.Args); len(args) > 0 {
		return command, args
	}
	command, args := parseCommandArgs(command)
	return command, args
}

func envListToMap(values []string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for _, item := range values {
		key, value, ok := strings.Cut(item, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func firstStringList(lists ...[]string) []string {
	for _, list := range lists {
		if trimmed := trimStringList(list); len(trimmed) > 0 {
			return trimmed
		}
	}
	return nil
}

func firstStringMap(maps ...map[string]string) map[string]string {
	for _, items := range maps {
		if cloned := cloneStringMap(items); len(cloned) > 0 {
			return cloned
		}
	}
	return nil
}

func firstFloat(values ...*float64) float64 {
	for _, value := range values {
		if value != nil && *value > 0 {
			return *value
		}
	}
	return 0
}

func trimStringList(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out = append(out, value)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]string, len(values))
	for key, value := range values {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func sortedEnvKeys(env map[string]string) []string {
	if len(env) == 0 {
		return nil
	}
	keys := make([]string, 0, len(env))
	for key := range env {
		if strings.TrimSpace(key) == "" {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
