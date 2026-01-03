package ai

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"wox/util"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rs/cors"
)

var (
	mcpPluginDevServer *mcp.Server
	mcpHTTPServer      *http.Server
	mcpServerMu        sync.Mutex
	mcpServerRunning   bool
	mcpServerPort      int
)

// StartMCPServer starts the MCP server on the specified port
func StartMCPServer(ctx context.Context, port int) error {
	mcpServerMu.Lock()
	defer mcpServerMu.Unlock()

	if mcpServerRunning {
		util.GetLogger().Info(ctx, "MCP: Server already running, skipping start")
		return nil
	}

	return startMCPServerInternal(ctx, port)
}

func startMCPServerInternal(ctx context.Context, port int) error {
	util.GetLogger().Info(ctx, fmt.Sprintf("MCP: Starting MCP server on port %d", port))

	// Create MCP server
	mcpPluginDevServer = mcp.NewServer(&mcp.Implementation{
		Name:    "wox-plugin-dev",
		Version: "1.0.0",
	}, nil)

	// Register tools
	registerMCPTools(ctx)

	// Create HTTP handler using NewStreamableHTTPHandler
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return mcpPluginDevServer
	}, &mcp.StreamableHTTPOptions{
		Stateless: true,
	})

	// Create HTTP mux
	mux := http.NewServeMux()
	mux.Handle("/mcp", handler)

	// Add CORS support
	corsHandler := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"*"},
	}).Handler(mux)

	// Create and start HTTP server
	mcpHTTPServer = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: corsHandler,
	}

	mcpServerPort = port
	mcpServerRunning = true

	util.Go(ctx, "mcp server", func() {
		util.GetLogger().Info(ctx, fmt.Sprintf("MCP: Server listening on http://127.0.0.1:%d/mcp", port))
		if err := mcpHTTPServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			util.GetLogger().Error(ctx, fmt.Sprintf("MCP: Server error: %s", err.Error()))
		}
		mcpServerMu.Lock()
		mcpServerRunning = false
		mcpServerMu.Unlock()
	})

	return nil
}

// StopMCPServer stops the MCP server
func StopMCPServer(ctx context.Context) error {
	mcpServerMu.Lock()
	defer mcpServerMu.Unlock()

	return stopMCPServerInternal(ctx)
}

func stopMCPServerInternal(ctx context.Context) error {
	if !mcpServerRunning || mcpHTTPServer == nil {
		return nil
	}

	util.GetLogger().Info(ctx, "MCP: Stopping MCP server")
	err := mcpHTTPServer.Shutdown(ctx)
	if err != nil {
		return err
	}

	mcpServerRunning = false
	mcpHTTPServer = nil
	mcpPluginDevServer = nil
	return nil
}

// RestartMCPServer restarts the MCP server with a new port
func RestartMCPServer(ctx context.Context, port int) error {
	mcpServerMu.Lock()
	defer mcpServerMu.Unlock()

	// Stop existing server if running
	if err := stopMCPServerInternal(ctx); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("MCP: Failed to stop server: %s", err.Error()))
	}

	// Start new server
	return startMCPServerInternal(ctx, port)
}

// IsMCPServerRunning returns whether the MCP server is running
func IsMCPServerRunning() bool {
	mcpServerMu.Lock()
	defer mcpServerMu.Unlock()
	return mcpServerRunning
}

func registerMCPTools(ctx context.Context) {
	util.GetLogger().Info(ctx, "MCP: Registering tools")

	// Tool: plugin_overview
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "plugin_overview",
		Description: "Overview of Wox plugin types and how the plugin system works",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handlePluginOverview)

	// Tool: get_plugin_sdk_docs
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "get_plugin_sdk_docs",
		Description: "Get Wox plugin SDK documentation and type definitions for developing plugins",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"runtime": map[string]any{
					"type":        "string",
					"description": "The plugin runtime: nodejs, python, script-nodejs, script-python, or script-bash",
					"enum":        []string{"nodejs", "python", "script-nodejs", "script-python", "script-bash"},
				},
			},
			"required": []string{"runtime"},
		},
	}, handleGetPluginSDKDocs)

	// Tool: get_plugin_json_schema
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "get_plugin_json_schema",
		Description: "Get the complete plugin.json schema definition for Wox plugins",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handleGetPluginJsonSchema)

	// Tool: generate_plugin_scaffold
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "generate_plugin_scaffold",
		Description: "Generate a complete plugin scaffold with all necessary files",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"runtime": map[string]any{
					"type":        "string",
					"description": "The plugin runtime: nodejs, python, script-nodejs, script-python, or script-bash",
					"enum":        []string{"nodejs", "python", "script-nodejs", "script-python", "script-bash"},
				},
				"name": map[string]any{
					"type":        "string",
					"description": "The plugin name",
				},
				"trigger_keywords": map[string]any{
					"type":        "array",
					"description": "List of trigger keywords for the plugin",
					"items": map[string]any{
						"type": "string",
					},
				},
				"description": map[string]any{
					"type":        "string",
					"description": "The plugin description",
				},
			},
			"required": []string{"runtime", "name", "trigger_keywords"},
		},
	}, handleGeneratePluginScaffold)

	// Tool: get_wox_directories
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "get_wox_directories",
		Description: "Get Wox directory paths for plugin development",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handleGetWoxDirectories)

	// Tool: plugin_i18n
	mcpPluginDevServer.AddTool(&mcp.Tool{
		Name:        "get_plugin_i18n",
		Description: "Guidelines for implementing multi-language support in Wox plugins",
		InputSchema: map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}, handlePluginI18n)

	util.GetLogger().Info(ctx, "MCP: Tools registered successfully")
}
