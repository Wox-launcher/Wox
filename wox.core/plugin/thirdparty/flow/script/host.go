// Package script runs Python, Node, and executable flow plugins.
// Each plugin is its own process. C# and F# plugins belong to the sibling
// dotnet host and are not started here.
package script

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"wox/common"
	"wox/plugin"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/util"
	"wox/util/shell"
)

var (
	pythonResolver func(context.Context) (string, error)
	nodeResolver   func(context.Context) (string, error)
)

// SetInterpreterResolvers supplies the Python and Node lookups owned by the
// shared interpreter hosts. A nil lookup leaves that interpreter unset, which
// does not stop executable plugins from loading.
func SetInterpreterResolvers(python, node func(context.Context) (string, error)) {
	pythonResolver = python
	nodeResolver = node
}

// Host is the process host for script flow plugins.
type Host struct {
	mu         sync.Mutex
	started    bool
	pythonPath string
	pythonErr  string
	nodePath   string
	clientDir  string
	plugins    map[string]*scriptPlugin
}

func (h *Host) GetRuntime(ctx context.Context) plugin.Runtime {
	return manifest.RuntimeJSONRPC
}

// Start resolves interpreters and writes the injected Python client.
// A missing interpreter does not fail the host, because executable plugins
// do not need one and a failed Start would skip every plugin of this runtime.
func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	python := pythonResolver
	if python == nil {
		python = plugin.ResolvePythonPath
	}
	node := nodeResolver
	if node == nil {
		node = plugin.ResolveNodePath
	}
	h.pythonPath, h.pythonErr = resolveInterpreter(ctx, python, "Python")
	h.nodePath, _ = resolveInterpreter(ctx, node, "Node.js")
	if h.clientDir == "" {
		h.clientDir = filepath.Join(util.GetLocation().GetHostDirectory(), "flow-jsonrpc-client")
	}
	if err := materializeFlowClient(h.clientDir); err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow script client was not written: %s", err.Error()))
	}
	if h.plugins == nil {
		h.plugins = map[string]*scriptPlugin{}
	}
	h.started = true
	return nil
}

func (h *Host) Stop(ctx context.Context) {
	h.mu.Lock()
	plugins := h.plugins
	h.plugins = map[string]*scriptPlugin{}
	h.started = false
	h.mu.Unlock()
	for _, item := range plugins {
		if item.session != nil {
			item.session.Close()
		}
	}
}

func (h *Host) IsStarted(ctx context.Context) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.started
}

func (h *Host) RuntimeStatus(ctx context.Context) plugin.RuntimeHostStatus {
	h.mu.Lock()
	defer h.mu.Unlock()
	if !h.started {
		return plugin.RuntimeHostStatus{
			StatusCode:    plugin.RuntimeHostStatusStopped,
			StatusMessage: "Flow script host is not running.",
		}
	}
	message := "Flow script host is running."
	if h.pythonErr != "" {
		message = "Flow script host is running. Python was not found; Python plugins cannot start."
	}
	return plugin.RuntimeHostStatus{
		StatusCode:     plugin.RuntimeHostStatusRunning,
		StatusMessage:  message,
		ExecutablePath: h.pythonPath,
		CanRestart:     true,
	}
}

// DiscoverMetadata reads script plugins from the user flow-jsonrpc directory.
func (h *Host) DiscoverMetadata(ctx context.Context) ([]plugin.Metadata, error) {
	root := manifest.CollectionDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(root); err != nil {
		return nil, err
	}
	return discoverMetadata(ctx, root)
}

func discoverMetadata(ctx context.Context, root string) ([]plugin.Metadata, error) {
	descriptors, err := manifest.LoadDirectory(ctx, root)
	if err != nil {
		return nil, err
	}
	var metadata []plugin.Metadata
	for _, descriptor := range descriptors {
		if descriptor.Kind == manifest.KindScript {
			metadata = append(metadata, descriptor.Metadata)
		}
	}
	return metadata, nil
}

// LoadPlugin parses the directory again and keeps the process stopped until Init.
func (h *Host) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	if strings.TrimSpace(pluginDirectory) == "" {
		pluginDirectory = metadata.Directory
	}
	descriptor, err := manifest.Parse(pluginDirectory)
	if err != nil {
		return nil, err
	}
	if descriptor.Kind != manifest.KindScript {
		return nil, fmt.Errorf("language %s is not handled by the flow script host", descriptor.Language)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.plugins == nil {
		h.plugins = map[string]*scriptPlugin{}
	}
	if existing := h.plugins[metadata.Id]; existing != nil && existing.session != nil {
		existing.session.Close()
	}
	item := &scriptPlugin{
		metadata: descriptor.Metadata,
		kinds:    descriptor.SettingKinds,
	}
	item.session = newFlowSession(
		descriptor.Metadata.GetName(ctx),
		pluginDirectory,
		descriptor.Metadata.Entry,
		descriptor.Language,
		detectFlowDialect(pluginDirectory, descriptor.Language, descriptor.Metadata.Entry),
		h.pythonPath,
		h.nodePath,
		h.clientDirectoryLocked(),
		pluginBridge{plugin: item},
	)
	h.plugins[metadata.Id] = item
	return item, nil
}

func (h *Host) UnloadPlugin(ctx context.Context, metadata plugin.Metadata) {
	h.mu.Lock()
	item := h.plugins[metadata.Id]
	delete(h.plugins, metadata.Id)
	h.mu.Unlock()
	if item != nil && item.session != nil {
		item.session.Close()
	}
}

func (h *Host) clientDirectoryLocked() string {
	if h.clientDir != "" {
		return h.clientDir
	}
	return filepath.Join(util.GetLocation().GetHostDirectory(), "flow-jsonrpc-client")
}

func resolveInterpreter(ctx context.Context, resolve func(context.Context) (string, error), name string) (string, string) {
	if resolve == nil {
		return "", ""
	}
	path, err := resolve(ctx)
	if err != nil {
		util.GetLogger().Warn(ctx, fmt.Sprintf("flow script host: %s: %s", name, err.Error()))
		return "", err.Error()
	}
	return path, ""
}

type scriptPlugin struct {
	metadata plugin.Metadata
	kinds    map[string]string
	session  *flowSession

	mu  sync.Mutex
	api plugin.API
	// extra keeps setting keys the plugin writes that are not in the template,
	// so the next query still sends them.
	extra map[string]any
}

func (p *scriptPlugin) Init(ctx context.Context, params plugin.InitParams) {
	if err := p.InitWithError(ctx, params); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[flow-jsonrpc:%s] init: %s", p.metadata.Id, err.Error()))
	}
}

// InitWithError stores the plugin API and starts a long-lived process when the dialect needs one.
func (p *scriptPlugin) InitWithError(ctx context.Context, params plugin.InitParams) error {
	p.mu.Lock()
	p.api = params.API
	session := p.session
	p.mu.Unlock()
	if session == nil {
		return errors.New("plugin process is not configured")
	}
	return session.Start(ctx)
}

func (p *scriptPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	keyword := strings.TrimSpace(query.TriggerKeyword)
	if keyword == "" {
		keyword = "*"
	}
	reply, err := p.session.Invoke(ctx, "query", []any{map[string]any{
		"Search":        query.Search,
		"RawQuery":      query.RawQuery,
		"ActionKeyword": keyword,
	}}, p.snapshotSettings(ctx))
	if err != nil {
		text := err.Error()
		return plugin.QueryResponse{Results: []plugin.QueryResult{{
			Title:    text,
			SubTitle: text,
			Icon:     p.icon(),
		}}}
	}
	p.mergeSettings(ctx, reply)
	return plugin.QueryResponse{Results: flowQueryResults(p.metadata.Directory, p.icon(), reply.Results, p.run)}
}

func (p *scriptPlugin) run(ctx context.Context, method string, params []any) {
	reply, err := p.session.Invoke(ctx, method, params, p.snapshotSettings(ctx))
	if err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[flow-jsonrpc:%s] %s: %s", p.metadata.Id, method, err.Error()))
		return
	}
	p.mergeSettings(ctx, reply)
}

func (p *scriptPlugin) icon() common.WoxImage {
	image, err := common.ParseWoxImage(p.metadata.Icon)
	if err != nil {
		return common.WoxImage{}
	}
	return image
}

func (p *scriptPlugin) snapshotSettings(ctx context.Context) map[string]any {
	p.mu.Lock()
	defer p.mu.Unlock()
	merged := make(map[string]any, len(p.extra)+len(p.metadata.SettingDefinitions))
	for key, value := range p.extra {
		merged[key] = value
	}
	if p.api == nil {
		return merged
	}
	for _, item := range p.metadata.SettingDefinitions {
		if item.Value == nil {
			continue
		}
		key := item.Value.GetKey()
		raw := p.api.GetSetting(ctx, key)
		if p.kinds[key] == manifest.SettingBool {
			merged[key] = strings.EqualFold(raw, "true")
			continue
		}
		merged[key] = raw
	}
	return merged
}

func (p *scriptPlugin) mergeSettings(ctx context.Context, reply flowReply) {
	if !reply.HasSettings {
		return
	}
	p.mu.Lock()
	api := p.api
	if p.extra == nil {
		p.extra = map[string]any{}
	}
	updates := make([]plugin.SetSettingOption, 0, len(reply.Settings))
	for key, value := range reply.Settings {
		normalized := normalizeSettingValue(key, value, p.kinds)
		p.extra[key] = normalized
		updates = append(updates, plugin.SetSettingOption{Key: key, Value: settingText(normalized)})
	}
	p.mu.Unlock()
	if api == nil {
		return
	}
	for _, update := range updates {
		result := api.SetSetting(ctx, update)
		if !result.Success && result.ErrMsg != "" {
			util.GetLogger().Warn(ctx, fmt.Sprintf("[flow-jsonrpc:%s] setting %s: %s", p.metadata.Id, update.Key, result.ErrMsg))
		}
	}
}

func normalizeSettingValue(key string, value any, kinds map[string]string) any {
	if kinds[key] == manifest.SettingBool {
		return settingTruthy(value)
	}
	if typed, ok := value.(bool); ok {
		return typed
	}
	return settingText(value)
}

func settingTruthy(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		return strings.EqualFold(strings.TrimSpace(typed), "true") || typed == "1"
	case float64:
		return typed != 0
	default:
		return false
	}
}

func settingText(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case bool:
		if typed {
			return "true"
		}
		return "false"
	case float64:
		if typed == float64(int64(typed)) {
			return fmt.Sprintf("%d", int64(typed))
		}
		return fmt.Sprint(typed)
	default:
		raw, err := json.Marshal(typed)
		if err != nil {
			return fmt.Sprint(typed)
		}
		return string(raw)
	}
}

type pluginBridge struct {
	plugin *scriptPlugin
}

func (b pluginBridge) ChangeQuery(ctx context.Context, query string) {
	api := b.api()
	if api == nil {
		return
	}
	api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: query})
}

func (b pluginBridge) HideApp(ctx context.Context) {
	if api := b.api(); api != nil {
		api.HideApp(ctx)
	}
}

func (b pluginBridge) ShowApp(ctx context.Context) {
	if api := b.api(); api != nil {
		api.ShowApp(ctx)
	}
}

func (b pluginBridge) Notify(ctx context.Context, title string, subtitle string) {
	api := b.api()
	if api == nil {
		return
	}
	text := title
	if subtitle != "" {
		if text != "" {
			text += "\n"
		}
		text += subtitle
	}
	api.Notify(ctx, text)
}

func (b pluginBridge) CopyText(ctx context.Context, text string) {
	api := b.api()
	if api == nil {
		return
	}
	api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: text})
}

func (b pluginBridge) OpenPath(ctx context.Context, target string) error {
	return shell.Open(target)
}

func (b pluginBridge) OpenDirectory(ctx context.Context, directory string, fileName string) error {
	return openFlowDirectory(directory, fileName)
}

// ShellRun starts a command without binding its output to the Wox log.
func (b pluginBridge) ShellRun(ctx context.Context, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return errors.New("command is empty")
	}
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = shell.BuildCommand("cmd.exe", nil, "/c", command)
	} else {
		cmd = shell.BuildCommand("sh", nil, "-c", command)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (b pluginBridge) Log(ctx context.Context, message string) {
	if api := b.api(); api != nil {
		api.Log(ctx, plugin.LogLevelInfo, message)
		return
	}
	util.GetLogger().Info(ctx, message)
}

func (b pluginBridge) api() plugin.API {
	if b.plugin == nil {
		return nil
	}
	b.plugin.mu.Lock()
	defer b.plugin.mu.Unlock()
	return b.plugin.api
}
