package ai

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image/png"
	"net"
	"net/http"
	"sync"
	"time"
	"wox/common"
	"wox/util"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/google/uuid"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// installedToolBridge keeps Wox's tool boundary and chat context inside one agent request.
// Two stable MCP tools work even with CLIs that cache tools/list for their entire turn.
type installedToolBridge struct {
	mu      sync.Mutex
	tools   []common.Tool
	options common.ChatOptions
	ctx     context.Context
	emit    func(installedEvent)
	calls   int
}

type installedToolCall struct {
	Name      string         `json:"name" jsonschema:"Exact Wox tool name returned by list_tools"`
	Arguments map[string]any `json:"arguments" jsonschema:"Arguments matching the selected tool's input schema"`
}

// startInstalledToolBridge opens a request-scoped, authenticated loopback MCP endpoint.
func startInstalledToolBridge(ctx context.Context, options common.ChatOptions, emit func(installedEvent)) (string, func(), error) {
	if len(options.Tools) == 0 {
		return "", func() {}, nil
	}
	if options.ExecuteTool == nil {
		return "", nil, fmt.Errorf("installed providers require Wox's tool executor")
	}
	bridge := &installedToolBridge{tools: append([]common.Tool(nil), options.Tools...), options: options, ctx: ctx, emit: emit}
	server := mcp.NewServer(&mcp.Implementation{Name: "wox", Version: "1"}, nil)
	mcp.AddTool(server, &mcp.Tool{Name: "list_tools", Description: "List currently callable Wox tools with their exact names and input schemas. Call again after Wox load_tools to see newly loaded tools."}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, any, error) {
		bridge.mu.Lock()
		defer bridge.mu.Unlock()
		catalog := make([]map[string]any, 0, len(bridge.tools))
		for _, tool := range bridge.tools {
			catalog = append(catalog, map[string]any{"name": tool.Name, "description": tool.Description, "inputSchema": tool.Parameters})
		}
		data, err := json.Marshal(catalog)
		util.GetLogger().Info(ctx, fmt.Sprintf("AI: CLI stage=list_tools tools=%d schemaBytes=%d", len(catalog), len(data)))
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: string(data)}}}, nil, err
	})
	mcp.AddTool(server, &mcp.Tool{Name: "call_tool", Description: "Execute a Wox tool. Use the exact name and schema from list_tools. To discover additional MCP tools, call Wox load_tools, then list_tools again."}, bridge.call)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", nil, err
	}
	path := "/" + rand.Text()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return server }, &mcp.StreamableHTTPOptions{Stateless: true, JSONResponse: true})
	httpServer := &http.Server{ReadHeaderTimeout: 5 * time.Second, BaseContext: func(net.Listener) context.Context { return ctx }, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path || r.Header.Get("Origin") != "" {
			http.NotFound(w, r)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
		handler.ServeHTTP(w, r)
	})}
	go httpServer.Serve(listener)
	return "http://" + listener.Addr().String() + path, func() { _ = httpServer.Close() }, nil
}

// call validates untrusted agent arguments before invoking the shared Wox executor.
func (b *installedToolBridge) call(requestCtx context.Context, _ *mcp.CallToolRequest, input installedToolCall) (*mcp.CallToolResult, any, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	ctx, cancel := context.WithCancel(b.ctx)
	defer cancel()
	stop := context.AfterFunc(requestCtx, cancel)
	defer stop()
	if err := ctx.Err(); err != nil {
		return nil, nil, err
	}
	limit := b.options.LoopPolicy.MaxIterations
	if limit == 0 {
		limit = 25
	}
	if limit > 0 && b.calls >= limit {
		return nil, nil, fmt.Errorf("Wox tool call limit reached (%d)", limit)
	}
	for _, tool := range b.tools {
		if tool.Name != input.Name {
			continue
		}
		data, err := json.Marshal(tool.Parameters)
		if err != nil {
			return nil, nil, err
		}
		var schema jsonschema.Schema
		if err = json.Unmarshal(data, &schema); err != nil {
			return nil, nil, err
		}
		resolved, err := schema.Resolve(nil)
		if err != nil {
			return nil, nil, err
		}
		if input.Arguments == nil {
			input.Arguments = map[string]any{}
		}
		if err = resolved.Validate(input.Arguments); err != nil {
			return nil, nil, fmt.Errorf("invalid %s arguments: %w", tool.Name, err)
		}
		b.calls++
		call := common.ToolCallInfo{Id: uuid.NewString(), Name: tool.Name, Arguments: input.Arguments, Status: common.ToolCallStatusPending, StartTimestamp: time.Now().UnixMilli()}
		ApplyToolOrigin(&call, tool)
		result := b.options.ExecuteTool(ctx, common.AgentToolExecutionOption{Tool: tool, Call: call, OnUpdate: func(call common.ToolCallInfo) { b.emit(installedEvent{call: &call}) }})
		b.tools = AppendRequestedTools(b.tools, []common.ToolCallInfo{result.Call})
		content := []mcp.Content{&mcp.TextContent{Text: result.Call.Response}}
		for _, source := range result.Result.Images {
			img, err := source.ToImageWithContext(ctx)
			if err != nil {
				return nil, nil, err
			}
			var buffer bytes.Buffer
			if err = png.Encode(&buffer, img); err != nil {
				return nil, nil, err
			}
			content = append(content, &mcp.ImageContent{Data: buffer.Bytes(), MIMEType: "image/png"})
		}
		return &mcp.CallToolResult{Content: content, IsError: result.Call.Status == common.ToolCallStatusFailed}, nil, nil
	}
	return nil, nil, fmt.Errorf("tool %q is not loaded; call load_tools first", input.Name)
}
