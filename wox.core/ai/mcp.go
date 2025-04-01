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

// MCPListTools lists the tools for a given MCP server config
func MCPListTools(ctx context.Context, config common.AIChatMCPServerConfig) (mcpTools []common.MCPTool, err error) {
	util.GetLogger().Debug(ctx, fmt.Sprintf("Listing tools for MCP server: %s", config.Name))

	command, args := parseCommandArgs(config.Command)

	c, err := client.NewStdioMCPClient(command, config.EnvironmentVariables, args...)
	if err != nil {
		return nil, err
	}
	defer c.Close()

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Wox",
		Version: "2.0.0",
	}
	initResult, err := c.Initialize(ctx, initRequest)
	if err != nil {
		return nil, err
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Initialized with server: %s %s", initResult.ServerInfo.Name, initResult.ServerInfo.Version))

	// List Tools
	tools, err := c.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, err
	}

	for _, tool := range tools.Tools {
		mcpTools = append(mcpTools, common.MCPTool{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf("Found %d tools", len(mcpTools)))

	return
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
