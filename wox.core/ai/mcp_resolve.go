package ai

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"wox/common"
	"wox/util"
	"wox/util/shell"
)

var mcpPlaceholderPattern = regexp.MustCompile(`\$\{(?:env:)?([A-Za-z_][A-Za-z0-9_]*)(?::-([^}]*))?\}`)

const (
	defaultMCPStartupTimeout = 30 * time.Second
	defaultMCPAuthTimeout    = 120 * time.Second
	defaultMCPToolTimeout    = 60 * time.Second
)

// mcpStartupTimeout is how long connect + tool listing may take.
func mcpStartupTimeout(config common.AIChatMCPServerConfig) time.Duration {
	if config.StartupTimeoutSec > 0 {
		return time.Duration(config.StartupTimeoutSec * float64(time.Second))
	}
	if config.Auth != nil && strings.TrimSpace(config.Auth.ClientID) != "" {
		return defaultMCPAuthTimeout
	}
	return defaultMCPStartupTimeout
}

// mcpToolTimeout is how long one MCP tool call may take.
func mcpToolTimeout(config common.AIChatMCPServerConfig) time.Duration {
	if config.ToolTimeoutSec > 0 {
		return time.Duration(config.ToolTimeoutSec * float64(time.Second))
	}
	return defaultMCPToolTimeout
}

func resolveMCPCommand(config common.AIChatMCPServerConfig) (string, []string) {
	command := interpolateMCPValue(strings.TrimSpace(config.Command))
	if args := trimStringList(config.Args); len(args) > 0 {
		resolved := make([]string, 0, len(args))
		for _, arg := range args {
			resolved = append(resolved, interpolateMCPValue(arg))
		}
		return command, resolved
	}
	return parseCommandArgs(command)
}

func resolveMCPProcessEnv(config common.AIChatMCPServerConfig) []string {
	merged := map[string]string{}
	if envFile := interpolateMCPValue(config.EnvFile); envFile != "" {
		for key, value := range loadMCPEnvFile(envFile) {
			merged[key] = interpolateMCPValue(value)
		}
	}
	for _, name := range trimStringList(config.EnvVars) {
		if value, ok := os.LookupEnv(name); ok {
			merged[name] = value
		}
	}
	for key, value := range envListToMap(config.EnvironmentVariables) {
		merged[key] = interpolateMCPValue(value)
	}
	if len(merged) == 0 {
		return nil
	}
	keys := sortedEnvKeys(merged)
	out := make([]string, 0, len(keys))
	for _, key := range keys {
		out = append(out, key+"="+merged[key])
	}
	return out
}

func loadMCPEnvFile(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	out := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		if unquoted, err := unquoteMCPEnvValue(value); err == nil {
			value = unquoted
		}
		out[key] = value
	}
	return out
}

func unquoteMCPEnvValue(value string) (string, error) {
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return value[1 : len(value)-1], nil
		}
	}
	return value, nil
}

func interpolateMCPValue(value string) string {
	if value == "" || !strings.Contains(value, "${") {
		return value
	}
	home, _ := os.UserHomeDir()
	sep := string(os.PathSeparator)
	value = strings.ReplaceAll(value, "${userHome}", home)
	value = strings.ReplaceAll(value, "${pathSeparator}", sep)
	value = strings.ReplaceAll(value, "${/}", sep)
	return mcpPlaceholderPattern.ReplaceAllStringFunc(value, func(match string) string {
		groups := mcpPlaceholderPattern.FindStringSubmatch(match)
		if len(groups) < 2 {
			return match
		}
		name := groups[1]
		fallback := ""
		if len(groups) > 2 {
			fallback = groups[2]
		}
		if env, ok := os.LookupEnv(name); ok {
			return env
		}
		if fallback != "" {
			return fallback
		}
		return match
	})
}

// FilterMCPServerTools applies the server's optional allow and deny lists.
// An allow list is applied first; the deny list then removes names from that set.
func FilterMCPServerTools(tools []common.MCPTool, config common.AIChatMCPServerConfig) []common.MCPTool {
	enabled := stringSet(config.EnabledTools)
	disabled := stringSet(config.DisabledTools)
	if len(enabled) == 0 && len(disabled) == 0 {
		return tools
	}
	out := make([]common.MCPTool, 0, len(tools))
	for _, tool := range tools {
		if len(enabled) > 0 && !enabled[tool.Name] {
			continue
		}
		if disabled[tool.Name] {
			continue
		}
		out = append(out, tool)
	}
	return out
}

func stringSet(values []string) map[string]bool {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]bool, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		out[value] = true
	}
	return out
}

func resolveMCPHeaders(ctx context.Context, config common.AIChatMCPServerConfig) map[string]string {
	headers := map[string]string{}
	for key, value := range config.Headers {
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		headers[key] = interpolateMCPValue(value)
	}
	if helper := interpolateMCPValue(config.HeadersHelper); helper != "" {
		for key, value := range runMCPHeadersHelper(ctx, helper, interpolateMCPValue(config.WorkingDirectory)) {
			headers[key] = interpolateMCPValue(value)
		}
	}
	if envName := strings.TrimSpace(config.BearerTokenEnvVar); envName != "" {
		if token := strings.TrimSpace(os.Getenv(envName)); token != "" {
			if _, exists := headers["Authorization"]; !exists {
				headers["Authorization"] = "Bearer " + token
			}
		}
	}
	return headers
}

func runMCPHeadersHelper(ctx context.Context, command, cwd string) map[string]string {
	timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = shell.BuildCommandContext(timeoutCtx, "cmd", nil, "/c", command)
	} else {
		cmd = shell.BuildCommandContext(timeoutCtx, "sh", nil, "-c", command)
	}
	if cwd != "" {
		cmd.Dir = cwd
	}
	output, err := cmd.Output()
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("MCP headersHelper failed: %s", err))
		return nil
	}
	output = bytes.TrimSpace(output)
	if len(output) == 0 {
		return nil
	}
	var headers map[string]string
	if err := json.Unmarshal(output, &headers); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("MCP headersHelper returned invalid JSON: %s", err))
		return nil
	}
	return headers
}

func newMCPHTTPClient(ctx context.Context, config common.AIChatMCPServerConfig) *http.Client {
	headers := resolveMCPHeaders(ctx, config)
	var oauth *mcpOAuthTransport
	if config.Auth != nil && strings.TrimSpace(config.Auth.ClientID) != "" {
		oauth = newMCPOAuthTransport(ctx, config)
	}
	return &http.Client{
		Timeout: 0,
		Transport: &mcpHeaderRoundTripper{
			base:    http.DefaultTransport,
			headers: headers,
			oauth:   oauth,
		},
	}
}

type mcpHeaderRoundTripper struct {
	base    http.RoundTripper
	headers map[string]string
	oauth   *mcpOAuthTransport
}

func (t *mcpHeaderRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	cloned := req.Clone(req.Context())
	for key, value := range t.headers {
		cloned.Header.Set(key, value)
	}
	staticAuth := cloned.Header.Get("Authorization") != ""
	if !staticAuth && t.oauth != nil {
		if token := t.oauth.cachedToken(); token != "" {
			cloned.Header.Set("Authorization", "Bearer "+token)
		}
	}
	resp, err := base.RoundTrip(cloned)
	if err != nil || t.oauth == nil || staticAuth {
		return resp, err
	}
	if resp.StatusCode != http.StatusUnauthorized && resp.StatusCode != http.StatusForbidden {
		return resp, nil
	}
	mcpOAuthTokens.Delete(t.oauth.config.Name)
	token, authErr := t.oauth.authorize(req.Context(), cloned, resp)
	if authErr != nil {
		return resp, nil
	}
	resp.Body.Close()
	retry := req.Clone(req.Context())
	for key, value := range t.headers {
		retry.Header.Set(key, value)
	}
	retry.Header.Set("Authorization", "Bearer "+token)
	return base.RoundTrip(retry)
}

func mcpWorkingDirectory(config common.AIChatMCPServerConfig) string {
	cwd := interpolateMCPValue(strings.TrimSpace(config.WorkingDirectory))
	if cwd == "" {
		return ""
	}
	if filepath.IsAbs(cwd) {
		return cwd
	}
	abs, err := filepath.Abs(cwd)
	if err != nil {
		return cwd
	}
	return abs
}
