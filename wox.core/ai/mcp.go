package ai

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/util"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
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
		toolsList = append(toolsList, common.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
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
