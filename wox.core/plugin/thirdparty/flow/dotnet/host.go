// Package dotnet runs C# and F# flow plugins.
// Each plugin is its own process. The process loads the plugin assembly and
// calls back into Wox over stdin and stdout.
package dotnet

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"wox/common"
	"wox/i18n"
	"wox/plugin"
	"wox/plugin/thirdparty/flow/brand"
	"wox/plugin/thirdparty/flow/manifest"
	"wox/plugin/thirdparty/settings"
	"wox/util"
	"wox/util/shell"
)

const dotnetInstallURL = "https://dotnet.microsoft.com/download/dotnet/10.0"

// Host runs C# and F# plugins. It is registered by the parent flow layer.
type Host struct {
	mu         sync.Mutex
	started    bool
	dotnetPath string
	dotnetErr  string
	hostDir    string
	hostErr    string
	plugins    map[string]*dotnetPlugin
}

func (h *Host) GetRuntime(ctx context.Context) plugin.Runtime {
	return manifest.RuntimeDotNet
}

// Icon is the Flow Launcher mark shown beside this runtime.
func (h *Host) Icon(ctx context.Context) common.WoxImage {
	return brand.Image()
}

// Start finds the dotnet host and the published loader.
// A missing runtime does not fail Start, because the other plugins of this
// layer use a different host and a failed Start would skip every plugin.
func (h *Host) Start(ctx context.Context) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.started {
		return nil
	}
	if runtime.GOOS != "windows" {
		h.dotnetErr = ".NET flow plugins are only supported on Windows."
		h.started = true
		return nil
	}
	h.dotnetPath, h.dotnetErr = findDotnet(ctx)
	h.hostDir, h.hostErr = prepareHostDirectory()
	if h.plugins == nil {
		h.plugins = map[string]*dotnetPlugin{}
	}
	h.started = true
	return nil
}

func (h *Host) Stop(ctx context.Context) {
	h.mu.Lock()
	plugins := h.plugins
	h.plugins = map[string]*dotnetPlugin{}
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
			StatusMessage: ".NET flow host is not running.",
		}
	}
	if h.dotnetErr != "" {
		return plugin.RuntimeHostStatus{
			StatusCode:    plugin.RuntimeHostStatusExecutableMissing,
			StatusMessage: h.dotnetErr,
			InstallUrl:    dotnetInstallURL,
			CanRestart:    true,
		}
	}
	if h.hostErr != "" {
		return plugin.RuntimeHostStatus{
			StatusCode:     plugin.RuntimeHostStatusStartFailed,
			StatusMessage:  h.hostErr,
			LastStartError: h.hostErr,
			CanRestart:     true,
		}
	}
	return plugin.RuntimeHostStatus{
		StatusCode:     plugin.RuntimeHostStatusRunning,
		StatusMessage:  ".NET flow host is running.",
		ExecutablePath: h.dotnetPath,
		CanRestart:     true,
	}
}

// DiscoverMetadata reads C# and F# plugins from the user flow-jsonrpc directory.
func (h *Host) DiscoverMetadata(ctx context.Context) ([]plugin.Metadata, error) {
	if runtime.GOOS != "windows" {
		return nil, nil
	}
	root := manifest.CollectionDirectory()
	if err := util.GetLocation().EnsureDirectoryExist(root); err != nil {
		return nil, err
	}
	return MetadataFrom(ctx, root)
}

// MetadataFrom lists C# and F# plugins under root.
func MetadataFrom(ctx context.Context, root string) ([]plugin.Metadata, error) {
	descriptors, err := manifest.LoadDirectory(ctx, root)
	if err != nil {
		return nil, err
	}
	var metadata []plugin.Metadata
	for _, descriptor := range descriptors {
		if descriptor.Kind == manifest.KindDotNet {
			metadata = append(metadata, descriptor.Metadata)
		}
	}
	return metadata, nil
}

// LoadPlugin parses the directory again. The process starts in Init.
func (h *Host) LoadPlugin(ctx context.Context, metadata plugin.Metadata, pluginDirectory string) (plugin.Plugin, error) {
	if strings.TrimSpace(pluginDirectory) == "" {
		pluginDirectory = metadata.Directory
	}
	descriptor, err := manifest.Parse(pluginDirectory)
	if err != nil {
		return nil, err
	}
	if descriptor.Kind != manifest.KindDotNet {
		return nil, fmt.Errorf("language %s is not handled by the flow .NET host", descriptor.Language)
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.plugins == nil {
		h.plugins = map[string]*dotnetPlugin{}
	}
	if existing := h.plugins[metadata.Id]; existing != nil && existing.session != nil {
		existing.session.Close()
	}
	item := &dotnetPlugin{metadata: descriptor.Metadata}
	keyword := "*"
	if len(descriptor.Metadata.TriggerKeywords) > 0 {
		keyword = descriptor.Metadata.TriggerKeywords[0]
	}
	item.session = newDotNetSession(dotNetLaunch{
		Name:       descriptor.Metadata.GetName(ctx),
		Directory:  pluginDirectory,
		Entry:      descriptor.Metadata.Entry,
		PluginID:   descriptor.Metadata.Id,
		Author:     descriptor.Metadata.Author,
		Version:    descriptor.Metadata.Version,
		Language:   descriptor.Language,
		UILanguage: string(i18n.GetI18nManager().GetCurrentLangCode()),
		Keyword:    keyword,
		IcoPath:    flowFieldRelative(pluginDirectory, descriptor.Metadata.Icon),
		DotNetPath: h.dotnetPath,
		HostDir:    h.hostDir,
		HostErr:    firstText(h.dotnetErr, h.hostErr),
		Bridge:     pluginBridge{plugin: item},
	})
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

func findDotnet(ctx context.Context) (string, string) {
	path, err := exec.LookPath("dotnet")
	if err != nil {
		util.GetLogger().Warn(ctx, "flow dotnet host: dotnet was not found")
		return "", ".NET desktop runtime was not found. C# and F# plugins cannot start."
	}
	return path, ""
}

func firstText(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

// flowFieldRelative keeps a plugin-local icon path for the loader.
// Metadata stores a Wox image string, not a raw filesystem path.
func flowFieldRelative(directory, icon string) string {
	image, err := common.ParseWoxImage(strings.TrimSpace(icon))
	if err != nil || image.ImageType != common.WoxImageTypeAbsolutePath || image.ImageData == "" {
		return ""
	}
	relative, relErr := filepath.Rel(directory, image.ImageData)
	if relErr != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return image.ImageData
	}
	return relative
}

type dotnetPlugin struct {
	metadata plugin.Metadata
	session  *dotnetSession

	mu           sync.Mutex
	api          plugin.API
	activeQuery  plugin.Query
	shownQueryID string
	shown        []shownDotNetRow
}

func (p *dotnetPlugin) Init(ctx context.Context, params plugin.InitParams) {
	if err := p.InitWithError(ctx, params); err != nil {
		util.GetLogger().Error(ctx, fmt.Sprintf("[flow-dotnet:%s] init: %s", p.metadata.Id, err.Error()))
	}
}

// InitWithError stores the plugin API and starts the plugin process.
func (p *dotnetPlugin) InitWithError(ctx context.Context, params plugin.InitParams) error {
	p.mu.Lock()
	p.api = params.API
	session := p.session
	p.mu.Unlock()
	if session == nil {
		return errors.New("plugin process is not configured")
	}
	return session.Start(ctx)
}

func (p *dotnetPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	p.mu.Lock()
	p.activeQuery = query
	p.mu.Unlock()
	keyword := strings.TrimSpace(query.TriggerKeyword)
	if keyword == "" {
		keyword = "*"
	}
	if p.session == nil {
		text := "plugin process is not configured"
		return plugin.QueryResponse{Results: []plugin.QueryResult{{
			Title:    text,
			SubTitle: text,
			Icon:     p.icon(),
		}}}
	}
	results, err := p.session.Query(ctx, query.Search, query.RawQuery, keyword)
	if err != nil {
		text := err.Error()
		return plugin.QueryResponse{Results: []plugin.QueryResult{{
			Title:    text,
			SubTitle: text,
			Icon:     p.icon(),
		}}}
	}
	rows := dotnetQueryResults(p.metadata.Directory, p.icon(), results, p.session)
	shown := assignResultIDs(rows)
	p.mu.Lock()
	if p.activeQuery.Id == query.Id {
		p.shownQueryID = query.Id
		p.shown = shown
	}
	p.mu.Unlock()
	return plugin.QueryResponse{Results: rows}
}

// HasNativeSettings reports the host settings window discovered during init.
func (p *dotnetPlugin) HasNativeSettings() bool {
	if p.session == nil {
		return false
	}
	return p.session.HasSettingPanel()
}

// OpenNativeSettings shows the plugin's own settings window, then reloads the plugin so Init reads the saved values.
// The reload starts Init in the background. Waiting for that Init keeps the settings row, which is only known once the new process reports its panel.
func (p *dotnetPlugin) OpenNativeSettings(ctx context.Context) error {
	if p.session == nil || !p.session.HasSettingPanel() {
		return errors.New("plugin has no settings panel")
	}
	if err := p.session.ShowSettings(ctx); err != nil {
		return err
	}
	if err := plugin.GetPluginManager().ReloadPlugin(ctx, p.metadata); err != nil {
		return err
	}
	return plugin.GetPluginManager().WaitPluginInit(ctx, p.metadata.Id)
}

var _ settings.Native = (*dotnetPlugin)(nil)

func (p *dotnetPlugin) icon() common.WoxImage {
	image, err := common.ParseWoxImage(p.metadata.Icon)
	if err != nil {
		return common.WoxImage{}
	}
	return image
}

type pluginBridge struct {
	plugin *dotnetPlugin
}

func (b pluginBridge) api() plugin.API {
	if b.plugin == nil {
		return nil
	}
	b.plugin.mu.Lock()
	defer b.plugin.mu.Unlock()
	return b.plugin.api
}

func (b pluginBridge) ChangeQuery(ctx context.Context, query string) {
	if api := b.api(); api != nil {
		api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: query})
	}
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
	if api := b.api(); api != nil {
		api.Copy(ctx, plugin.CopyParams{Type: plugin.CopyTypePlainText, Text: text})
	}
}

func (b pluginBridge) OpenPath(ctx context.Context, target string) error {
	return shell.Open(target)
}

func (b pluginBridge) OpenDirectory(ctx context.Context, directory string, fileName string) error {
	fileName = strings.TrimSpace(fileName)
	if fileName != "" {
		target := fileName
		if !filepath.IsAbs(fileName) {
			target = filepath.Join(directory, fileName)
		}
		if err := shell.OpenFileInFolder(target); err == nil {
			return nil
		}
	}
	if strings.TrimSpace(directory) == "" {
		return errors.New("directory is empty")
	}
	return shell.Open(directory)
}

func (b pluginBridge) ShellRun(ctx context.Context, program string, command string) error {
	program = strings.TrimSpace(program)
	command = strings.TrimSpace(command)
	if program == "" {
		program = "cmd.exe"
	}
	var args []string
	switch strings.ToLower(filepath.Base(program)) {
	case "powershell.exe", "pwsh.exe":
		args = []string{"-NoProfile", "-Command", command}
	case "cmd.exe":
		args = []string{"/c", command}
	default:
		if command != "" {
			args = []string{command}
		}
	}
	cmd := shell.BuildCommand(program, nil, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

func (b pluginBridge) Refresh(ctx context.Context) {
	if api := b.api(); api != nil {
		api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
	}
}

// UpdateResults applies one result list to the query that produced it.
// Rows already returned for that query are updated in place. Additional rows are pushed.
// A list from another search is ignored. Rows that disappeared stay until the next query,
// because UpdateResult cannot remove a visible row.
func (b pluginBridge) UpdateResults(ctx context.Context, search string, results []dotnetResult) {
	if b.plugin == nil {
		return
	}
	b.plugin.mu.Lock()
	api := b.plugin.api
	query := b.plugin.activeQuery
	session := b.plugin.session
	shownQueryID := b.plugin.shownQueryID
	shown := append([]shownDotNetRow(nil), b.plugin.shown...)
	b.plugin.mu.Unlock()
	if api == nil || query.Id == "" || query.SessionId == "" || query.Search != search {
		return
	}
	// Rows that arrive before this query's response is stored are all additional.
	// The response is the complete list and replaces what is on screen.
	published := shownQueryID == query.Id
	if !published {
		shown = nil
	}
	rows := dotnetQueryResults(b.plugin.metadata.Directory, b.plugin.icon(), results, session)
	updates, extra, next := splitResultUpdate(shown, rows)
	ctx = util.WithQueryIdContext(util.WithSessionContext(ctx, query.SessionId), query.Id)
	for _, update := range updates {
		api.UpdateResult(ctx, update)
	}
	if len(extra) > 0 {
		api.PushResults(ctx, query, extra)
	}
	if !published {
		return
	}
	b.plugin.mu.Lock()
	if b.plugin.activeQuery.Id == query.Id && b.plugin.shownQueryID == query.Id {
		b.plugin.shown = next
	}
	b.plugin.mu.Unlock()
}
