package chat

import (
	"context"
	"fmt"
	"strings"
	"wox/common"
	"wox/util"

	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/mcp"
)

type AIChatMCPServerType string

const (
	AIChatMCPServerTypeSTDIO AIChatMCPServerType = "stdio"
	AIChatMCPServerTypeSSE   AIChatMCPServerType = "sse"
)

type AIChatMCPServerConfig struct {
	Name string
	Type AIChatMCPServerType

	// for stdio server
	Command              string
	EnvironmentVariables []string //key=value

	// for sse server
	Url string
}

func (a *AIChatMCPServerConfig) listTool(ctx context.Context) (chatTools []common.Tool, err error) {
	command, args := a.parseCommandArgs()

	c, err := client.NewStdioMCPClient(command, a.EnvironmentVariables, args...)
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
		chatTools = append(chatTools, common.Tool{
			Name:        tool.Name,
			Description: tool.Description,
		})
	}

	return
}

// raw command is like "npx -y @modelcontextprotocol/server-filesystem /tmp"
// we need to split it into command and args
func (a *AIChatMCPServerConfig) parseCommandArgs() (command string, args []string) {
	parts := strings.Split(a.Command, " ")
	if len(parts) <= 1 {
		return a.Command, []string{}
	}

	command = parts[0]
	args = parts[1:]
	return
}
