package ai

import (
	"context"
	"sync"

	"wox/common"
	"wox/util"
)

// ToolRegistry holds every tool callable by the AI, regardless of source
// (MCP or builtin). It is the single source of truth consumed by ChatOptions
// and the providers.
type ToolRegistry struct {
	mu           sync.RWMutex
	tools        map[string]common.Tool
	builtinTools map[string]common.Tool
	mcpTools     map[string]common.Tool
}

var globalRegistry = &ToolRegistry{
	tools:        make(map[string]common.Tool),
	builtinTools: make(map[string]common.Tool),
	mcpTools:     make(map[string]common.Tool),
}

// GetToolRegistry returns the process-wide registry.
func GetToolRegistry() *ToolRegistry { return globalRegistry }

// Register adds or replaces a tool keyed by its Name. Thread-safe.
// When sources collide, rebuildLocked logs the shadowing and MCP tools take
// precedence over builtin tools.
func (r *ToolRegistry) Register(tool common.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.setLocked(tool)
	r.rebuildLocked()
}

// ReplaceSource atomically replaces all tools from one source while preserving
// other sources. This keeps deleted or disabled MCP tools from lingering after
// a server settings reload.
func (r *ToolRegistry) ReplaceSource(source common.ToolSource, tools []common.Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	switch source {
	case common.ToolSourceBuiltin:
		r.builtinTools = make(map[string]common.Tool)
	case common.ToolSourceMCP:
		r.mcpTools = make(map[string]common.Tool)
	}
	for _, tool := range tools {
		r.setLocked(tool)
	}
	r.rebuildLocked()
}

// Unregister removes a tool by Name. Thread-safe.
func (r *ToolRegistry) Unregister(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.builtinTools, name)
	delete(r.mcpTools, name)
	r.rebuildLocked()
}

// Get looks up a tool by Name.
func (r *ToolRegistry) Get(name string) (common.Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// List returns a snapshot of all registered tools.
func (r *ToolRegistry) List() []common.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]common.Tool, 0, len(r.tools))
	for _, t := range r.tools {
		out = append(out, t)
	}
	return out
}

// FindByName returns tools matching the given names, preserving input order.
// Unknown names are skipped.
func (r *ToolRegistry) FindByName(names []string) []common.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]common.Tool, 0, len(names))
	for _, n := range names {
		if t, ok := r.tools[n]; ok {
			out = append(out, t)
		}
	}
	return out
}

// ListBySource returns a snapshot of tools from a specific source.
func (r *ToolRegistry) ListBySource(source common.ToolSource) []common.Tool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]common.Tool, 0)
	for _, t := range r.tools {
		if t.Source == source {
			out = append(out, t)
		}
	}
	return out
}

func (r *ToolRegistry) setLocked(tool common.Tool) {
	switch tool.Source {
	case common.ToolSourceBuiltin:
		r.builtinTools[tool.Name] = tool
	case common.ToolSourceMCP:
		r.mcpTools[tool.Name] = tool
	default:
		r.tools[tool.Name] = tool
	}
}

func (r *ToolRegistry) rebuildLocked() {
	r.tools = make(map[string]common.Tool, len(r.builtinTools)+len(r.mcpTools))
	for name, tool := range r.builtinTools {
		r.tools[name] = tool
	}
	for name, tool := range r.mcpTools {
		if _, exists := r.tools[name]; exists {
			util.GetLogger().Warn(context.Background(), "AI: tool name collision, overwriting: "+name)
		}
		r.tools[name] = tool
	}
}
