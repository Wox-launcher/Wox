package ai

import (
	"context"
	"fmt"
	"strings"
	"time"
	"wox/common"
	"wox/util"
	"wox/util/shell"

	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/tmc/langchaingo/jsonschema"
)

var mcpSessions = util.NewHashMap[string, *mcp.ClientSession]()
var mcpTools = util.NewHashMap[string, []common.MCPTool]()
var mcpToolNames = util.NewHashMap[string, []string]()

func getMCPSession(ctx context.Context, config common.AIChatMCPServerConfig) (*mcp.ClientSession, error) {
	if session, ok := mcpSessions.Load(config.Name); ok {
		return session, nil
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "Wox",
		Version: "2.0.0",
	}, nil)

	var transport mcp.Transport
	if config.Type == common.AIChatMCPServerTypeSTDIO {
		command, args := resolveMCPCommand(config)
		cmd := shell.BuildCommand(command, nil, args...)
		cmd.Env = append(cmd.Env, resolveMCPProcessEnv(config)...)
		if cwd := mcpWorkingDirectory(config); cwd != "" {
			cmd.Dir = cwd
		}
		transport = &lifetimeBoundCommandTransport{serverName: config.Name, command: cmd}
	}
	if config.Type == common.AIChatMCPServerTypeStreamableHTTP {
		endpoint := interpolateMCPValue(strings.TrimSpace(config.Url))
		transport = &mcp.StreamableClientTransport{
			Endpoint:   endpoint,
			HTTPClient: newMCPHTTPClient(ctx, config),
		}
	}
	if transport == nil {
		return nil, fmt.Errorf("unsupported MCP server type: %s", config.Type)
	}

	// Connect to the server (handles initialization automatically)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}

	mcpSessions.Store(config.Name, session)
	return session, nil
}

// ResetMCPClients drops cached sessions and tool lists so the next connect uses current settings.
func ResetMCPClients() {
	mcpSessions.Range(func(_ string, session *mcp.ClientSession) bool {
		if session != nil {
			_ = session.Close()
		}
		return true
	})
	mcpSessions.Clear()
	mcpTools.Clear()
	mcpOAuthTokens.Clear()
	mcpServerProcesses.Clear()
}

func rememberMCPToolNames(serverName string, tools []common.MCPTool) {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		if name := strings.TrimSpace(tool.Name); name != "" {
			names = append(names, name)
		}
	}
	mcpToolNames.Store(serverName, names)
}

// SetCachedMCPToolsForTest stores discovered tools so settings-table tests can overlay names.
func SetCachedMCPToolsForTest(serverName string, tools []common.MCPTool) {
	mcpTools.Store(serverName, tools)
	rememberMCPToolNames(serverName, tools)
}

// CachedMCPToolNames returns the last discovered tool names for a server.
// Names survive session reset so the settings table can keep showing them
// while a server is reconnecting.
func CachedMCPToolNames(serverName string) ([]string, bool) {
	if names, ok := mcpToolNames.Load(serverName); ok {
		return append([]string(nil), names...), true
	}
	tools, ok := mcpTools.Load(serverName)
	if !ok {
		return nil, false
	}
	rememberMCPToolNames(serverName, tools)
	return CachedMCPToolNames(serverName)
}

// MCPListTools lists the tools for a given MCP server config with timeout protection
func MCPListTools(ctx context.Context, config common.AIChatMCPServerConfig) ([]common.MCPTool, error) {
	if tools, ok := mcpTools.Load(config.Name); ok {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Listing tools for MCP server from cache: %s", config.Name))
		return tools, nil
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Listing tools for MCP server: %s", config.Name))

	// Use channel and goroutine to implement timeout protection
	type listToolsResult struct {
		tools []common.MCPTool
		err   error
	}

	resultChan := make(chan listToolsResult, 1)

	// Start the actual tool listing in a separate goroutine
	go func() {
		defer func() {
			if r := recover(); r != nil {
				util.GetLogger().Error(ctx, fmt.Sprintf("Panic in MCPListTools for server %s: %v", config.Name, r))
				resultChan <- listToolsResult{
					tools: nil,
					err:   fmt.Errorf("panic occurred while listing tools: %v", r),
				}
			}
		}()

		startupTimeout := mcpStartupTimeout(config)
		timeoutCtx, cancel := context.WithTimeout(ctx, startupTimeout)
		defer cancel()

		session, err := getMCPSession(timeoutCtx, config)
		if err != nil {
			resultChan <- listToolsResult{tools: nil, err: err}
			return
		}

		// Process tools and send result
		processedTools, processErr := processToolsFromSession(timeoutCtx, session, config)
		resultChan <- listToolsResult{tools: processedTools, err: processErr}
	}()

	startupTimeout := mcpStartupTimeout(config)
	select {
	case result := <-resultChan:
		if result.err != nil {
			return nil, result.err
		}

		util.GetLogger().Debug(ctx, fmt.Sprintf("Found %d tools", len(result.tools)))
		mcpTools.Store(config.Name, result.tools)
		rememberMCPToolNames(config.Name, result.tools)
		return result.tools, nil

	case <-time.After(startupTimeout + 5*time.Second):
		util.GetLogger().Error(ctx, fmt.Sprintf("Timeout listing tools for MCP server: %s", config.Name))
		return nil, fmt.Errorf("timeout listing tools for server: %s", config.Name)
	}
}

// processToolsFromSession processes tools from a session and converts to MCPTool format
func processToolsFromSession(ctx context.Context, session *mcp.ClientSession, config common.AIChatMCPServerConfig) ([]common.MCPTool, error) {
	var toolsList []common.MCPTool

	// Use the Tools iterator to get all tools
	for tool, err := range session.Tools(ctx, nil) {
		if err != nil {
			return nil, fmt.Errorf("error iterating tools: %w", err)
		}

		parameters := jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: make(map[string]jsonschema.Definition),
		}

		// Process InputSchema if available (InputSchema is of type any, need to parse)
		if tool.InputSchema != nil {
			if schemaMap, ok := tool.InputSchema.(map[string]any); ok {
				// Extract required fields
				if requiredRaw, hasRequired := schemaMap["required"]; hasRequired {
					if requiredSlice, ok := requiredRaw.([]any); ok {
						for _, r := range requiredSlice {
							if s, ok := r.(string); ok {
								parameters.Required = append(parameters.Required, s)
							}
						}
					}
				}

				// Extract properties
				if propertiesRaw, hasProps := schemaMap["properties"]; hasProps {
					if propertiesMap, ok := propertiesRaw.(map[string]any); ok {
						for name, property := range propertiesMap {
							if propertyMap, ok := property.(map[string]any); ok {
								propTypeRaw := propertyMap["type"]
								propDescriptionRaw := propertyMap["description"]

								propType := ""
								propDescription := ""
								if propTypeRaw != nil {
									propType, _ = propTypeRaw.(string)
								}
								if propDescriptionRaw != nil {
									propDescription, _ = propDescriptionRaw.(string)
								}

								switch propType {
								case "string":
									parameters.Properties[name] = jsonschema.Definition{
										Type:        jsonschema.String,
										Description: propDescription,
									}
								case "integer", "number":
									parameters.Properties[name] = jsonschema.Definition{
										Type:        jsonschema.Integer,
										Description: propDescription,
									}
								case "boolean":
									parameters.Properties[name] = jsonschema.Definition{
										Type:        jsonschema.Boolean,
										Description: propDescription,
									}
								case "array":
									itemsDefinition := &jsonschema.Definition{Type: jsonschema.String}
									if itemsRaw, hasItems := propertyMap["items"]; hasItems {
										if itemsMap, ok := itemsRaw.(map[string]any); ok {
											if itemTypeRaw, hasType := itemsMap["type"]; hasType {
												if itemType, ok := itemTypeRaw.(string); ok {
													switch itemType {
													case "string":
														itemsDefinition = &jsonschema.Definition{Type: jsonschema.String}
													case "integer", "number":
														itemsDefinition = &jsonschema.Definition{Type: jsonschema.Integer}
													case "boolean":
														itemsDefinition = &jsonschema.Definition{Type: jsonschema.Boolean}
													}
												}
											}
										}
									}
									parameters.Properties[name] = jsonschema.Definition{
										Type:        jsonschema.Array,
										Description: propDescription,
										Items:       itemsDefinition,
									}
								}
							}
						}
					}
				}
			}
		}

		// Capture tool name for closure
		toolName := tool.Name
		toolDescription := tool.Description

		toolsList = append(toolsList, common.MCPTool{
			Name:        toolName,
			Description: toolDescription,
			Parameters:  parameters,
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				util.GetLogger().Debug(ctx, fmt.Sprintf("MCP: Tool call: %s, args: %v", toolName, args))

				callCtx := ctx
				var cancel context.CancelFunc
				if timeout := mcpToolTimeout(config); timeout > 0 {
					callCtx, cancel = context.WithTimeout(ctx, timeout)
					defer cancel()
				}
				result, err := session.CallTool(callCtx, &mcp.CallToolParams{
					Name:      toolName,
					Arguments: args,
				})
				if err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("MCP: Tool call: %s, error: %s", toolName, err))
					return common.Conversation{}, err
				}

				if result.IsError {
					errMsg := "unknown error"
					if len(result.Content) > 0 {
						errMsg = fmt.Sprintf("%v", result.Content[0])
					}
					return common.Conversation{}, fmt.Errorf("MCP: Tool call ended with error: %s", errMsg)
				}

				if len(result.Content) == 0 {
					return common.Conversation{}, fmt.Errorf("MCP: Tool call: %s, no content", toolName)
				}

				if v, ok := result.Content[0].(*mcp.TextContent); ok {
					return common.Conversation{
						Id:   uuid.New().String(),
						Role: common.ConversationRoleAssistant,
						Text: v.Text,
					}, nil
				}

				return common.Conversation{}, fmt.Errorf("MCP: Tool call: %s, unsupported content type: %T", toolName, result.Content[0])
			},
			ServerConfig: &config,
		})
	}

	toolsList = FilterMCPServerTools(toolsList, config)
	util.GetLogger().Debug(ctx, fmt.Sprintf("Found %d tools", len(toolsList)))
	mcpTools.Store(config.Name, toolsList)
	rememberMCPToolNames(config.Name, toolsList)

	return toolsList, nil
}

// raw command is like "npx -y @modelcontextprotocol/server-filesystem /tmp"
// we need to split it into command and args
func parseCommandArgs(commands string) (command string, args []string) {
	parts := strings.Split(commands, " ")
	if len(parts) <= 1 {
		return commands, []string{}
	}

	command = parts[0]
	args = parts[1:]
	return
}
