package chat

import (
	"context"
	"fmt"
	"strings"
	"time"
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

func (a *AIChatMCPServerConfig) listTool(parentCtx context.Context) error {
	command, args := a.parseCommandArgs()

	c, err := client.NewStdioMCPClient(
		command,
		a.EnvironmentVariables,
		args...,
	)
	if err != nil {
		return err
	}
	defer c.Close()

	// Create context with timeout
	ctx, cancel := context.WithTimeout(parentCtx, 30*time.Second)
	defer cancel()

	// Initialize the client
	initRequest := mcp.InitializeRequest{}
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "Wox",
		Version: "2.0.0",
	}
	initResult, err := c.Initialize(ctx, initRequest)
	if err != nil {
		return err
	}

	util.GetLogger().Debug(ctx, fmt.Sprintf(
		"Initialized with server: %s %s",
		initResult.ServerInfo.Name,
		initResult.ServerInfo.Version,
	))

	// List Tools
	toolsRequest := mcp.ListToolsRequest{}
	tools, err := c.ListTools(ctx, toolsRequest)
	if err != nil {
		return err
	}
	for _, tool := range tools.Tools {
		util.GetLogger().Debug(ctx, fmt.Sprintf("- %s: %s", tool.Name, tool.Description))
	}

	return nil
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
