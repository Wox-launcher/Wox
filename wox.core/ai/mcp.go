package ai

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/util"

	"github.com/google/uuid"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/tmc/langchaingo/jsonschema"
)

var mcpClients = util.NewHashMap[string, *client.StdioMCPClient]()
var mcpTools = util.NewHashMap[string, []common.MCPTool]()

func getMCPClient(ctx context.Context, config common.AIChatMCPServerConfig) (c *client.StdioMCPClient, err error) {
	if client, ok := mcpClients.Load(config.Name); ok {
		return client, nil
	}

	command, args := parseCommandArgs(config.Command)
	client, newErr := client.NewStdioMCPClient(command, config.EnvironmentVariables, args...)
	if newErr != nil {
		return nil, newErr
	}

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Wox",
		Version: "2.0.0",
	}
	_, initializeErr := client.Initialize(ctx, initRequest)
	if initializeErr != nil {
		return nil, initializeErr
	}

	mcpClients.Store(config.Name, client)
	return client, nil
}

// MCPListTools lists the tools for a given MCP server config
func MCPListTools(ctx context.Context, config common.AIChatMCPServerConfig) ([]common.MCPTool, error) {
	if tools, ok := mcpTools.Load(config.Name); ok {
		util.GetLogger().Debug(ctx, fmt.Sprintf("Listing tools for MCP server from cache: %s", config.Name))
		return tools, nil
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Listing tools for MCP server: %s", config.Name))
	client, err := getMCPClient(ctx, config)
	if err != nil {
		return nil, err
	}

	// List Tools
	tools, err := client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	var toolsList []common.MCPTool
	for _, tool := range tools.Tools {

		var parameters = jsonschema.Definition{
			Type:       jsonschema.Object,
			Properties: make(map[string]jsonschema.Definition),
			Required:   tool.InputSchema.Required,
		}

		for name, property := range tool.InputSchema.Properties {
			if propertyMap, ok := property.(map[string]any); ok {
				propertyTypeRaw := propertyMap["type"]
				propertyDescriptionRaw := propertyMap["description"]
				if propertyTypeRaw != nil && propertyDescriptionRaw != nil {
					propertyType := propertyTypeRaw.(string)
					propertyDescription := propertyDescriptionRaw.(string)

					switch propertyType {
					case "string":
						parameters.Properties[name] = jsonschema.Definition{
							Type:        jsonschema.String,
							Description: propertyDescription,
						}
					case "integer":
						parameters.Properties[name] = jsonschema.Definition{
							Type:        jsonschema.Integer,
							Description: propertyDescription,
						}
					case "boolean":
						parameters.Properties[name] = jsonschema.Definition{
							Type:        jsonschema.Boolean,
							Description: propertyDescription,
						}
					case "array":
						var itemsDefinition *jsonschema.Definition

						if itemsRaw, hasItems := propertyMap["items"]; hasItems {
							if itemsMap, ok := itemsRaw.(map[string]any); ok {
								itemTypeRaw, hasType := itemsMap["type"]
								if hasType {
									itemType, isString := itemTypeRaw.(string)
									if isString {
										switch itemType {
										case "string":
											itemsDefinition = &jsonschema.Definition{Type: jsonschema.String}
										case "integer":
											itemsDefinition = &jsonschema.Definition{Type: jsonschema.Integer}
										case "boolean":
											itemsDefinition = &jsonschema.Definition{Type: jsonschema.Boolean}
										default:
											// 默认使用字符串类型
											itemsDefinition = &jsonschema.Definition{Type: jsonschema.String}
										}
									}
								}
							}
						}

						if itemsDefinition == nil {
							itemsDefinition = &jsonschema.Definition{Type: jsonschema.String}
						}

						parameters.Properties[name] = jsonschema.Definition{
							Type:        jsonschema.Array,
							Description: propertyDescription,
							Items:       itemsDefinition,
						}
					}
				}
			}
		}

		toolsList = append(toolsList, common.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
			Callback: func(ctx context.Context, args map[string]any) (common.Conversation, error) {
				util.GetLogger().Debug(ctx, fmt.Sprintf("MCP: Tool call: %s, args: %v", tool.Name, args))

				request := mcp.CallToolRequest{
					Request: mcp.Request{
						Method: "tools/call",
					},
				}
				request.Params.Name = tool.Name
				request.Params.Arguments = args

				result, err := client.CallTool(ctx, request)
				if err != nil {
					util.GetLogger().Error(ctx, fmt.Sprintf("MCP: Tool call: %s, error: %s", tool.Name, err))
					return common.Conversation{}, err
				}

				if result.IsError {
					errMsg := "unkown error"
					if len(result.Content) > 0 {
						errMsg = fmt.Sprintf("%v", result.Content[0])
					}

					return common.Conversation{}, fmt.Errorf("MCP: Tool call ended with error: %s ", errMsg)
				}

				content := result.Content
				if len(content) == 0 {
					return common.Conversation{}, fmt.Errorf("MCP: Tool call: %s, no content", tool.Name)
				}

				if v, ok := content[0].(mcp.TextContent); ok {
					return common.Conversation{
						Id:   uuid.New().String(),
						Role: common.ConversationRoleAssistant,
						Text: v.Text,
					}, nil
				} else {
					return common.Conversation{}, fmt.Errorf("MCP: Tool call: %s, unsupported content type: %T", tool.Name, content[0])
				}
			},
			ServerConfig: &config,
		})
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Found %d tools", len(toolsList)))
	mcpTools.Store(config.Name, toolsList)

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
