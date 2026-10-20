package extension

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"wox/plugin"
	"wox/util"
)

// Host owns loaded metadata; Node processes are created only when a command is invoked.
type Host struct {
	mu      sync.Mutex
	started bool
	node    string
	bundle  string
	plugins map[string]*extensionPlugin
}

func (*Host) GetRuntime(context.Context) plugin.Runtime { return Runtime }

// Start reuses Wox's Node resolver and the host bundle built with the rest of Wox.
func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if plugin.ResolveNodePath == nil {
		return fmt.Errorf("Node.js resolver is unavailable")
	}
	node, err := plugin.ResolveNodePath(ctx)
	if err != nil {
		return err
	}
	bundle := filepath.Join(util.GetLocation().GetHostDirectory(), "raycast-host.js")
	if _, err := os.Stat(bundle); err != nil {
		return fmt.Errorf("Raycast runtime bundle: %w", err)
	}
	h.node, h.bundle, h.started = node, bundle, true
	h.plugins = map[string]*extensionPlugin{}
	return nil
}

func (h *Host) IsStarted(context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started
}

// Stop releases every command before the host can be restarted.
func (h *Host) Stop(ctx context.Context) {
	h.mu.Lock()
	plugins := h.plugins
	h.plugins = nil
	h.started = false
	h.mu.Unlock()
	for _, p := range plugins {
		p.closeAll()
	}
}

func (h *Host) RuntimeStatus(context.Context) plugin.RuntimeHostStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	status := plugin.RuntimeHostStatusStopped
	if h.started {
		status = plugin.RuntimeHostStatusRunning
	}
	return plugin.RuntimeHostStatus{StatusCode: status, ExecutablePath: h.node, CanRestart: true}
}

// DiscoverMetadata uses the same reserved collection scan as other compatibility hosts.
func (*Host) DiscoverMetadata(ctx context.Context) ([]plugin.Metadata, error) {
	entries, err := os.ReadDir(CollectionDirectory())
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var metadata []plugin.Metadata
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		d, err := Parse(filepath.Join(CollectionDirectory(), entry.Name()))
		if err != nil {
			util.GetLogger().Warn(ctx, fmt.Sprintf("Raycast extension %s: %v", entry.Name(), err))
			continue
		}
		metadata = append(metadata, d.Metadata)
	}
	return metadata, nil
}

func (h *Host) LoadPlugin(ctx context.Context, metadata plugin.Metadata, directory string) (plugin.Plugin, error) {
	d, err := Parse(directory)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return nil, fmt.Errorf("Raycast host is stopped")
	}
	if old := h.plugins[metadata.Id]; old != nil {
		old.closeAll()
	}
	p := &extensionPlugin{descriptor: d, node: h.node, bundle: h.bundle, sessions: map[string]*commandSession{}}
	h.plugins[metadata.Id] = p
	return p, nil
}

func (h *Host) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	h.mu.Lock()
	p := h.plugins[metadata.Id]
	delete(h.plugins, metadata.Id)
	h.mu.Unlock()
	if p != nil {
		p.closeAll()
	}
}
