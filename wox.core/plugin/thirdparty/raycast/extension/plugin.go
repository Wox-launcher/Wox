package extension

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"time"

	"wox/common"
	"wox/common/icons"
	"wox/plugin"
	"wox/util"
)

// extensionPlugin exposes command discovery through ordinary Wox results and retains one command per window.
type extensionPlugin struct {
	mu         sync.Mutex
	queryMu    sync.Mutex
	storageMu  sync.Mutex
	descriptor Descriptor
	node       string
	bundle     string
	api        plugin.API
	sessions   map[string]*commandSession
	closed     bool
}

func (p *extensionPlugin) Init(ctx context.Context, params plugin.InitParams) {
	p.api = params.API
	p.api.OnUnload(ctx, func(context.Context) { p.closeAll() })
	p.api.OnSettingChanged(ctx, func(_ context.Context, key, _ string) {
		if !strings.HasPrefix(key, "preference:") && !strings.HasPrefix(key, "command:") {
			return
		}
		p.mu.Lock()
		defer p.mu.Unlock()
		for key, s := range p.sessions {
			if s.command.Mode == "view" {
				s.close()
				delete(p.sessions, key)
			}
		}
	})
	p.api.OnLeavePluginQuery(ctx, func(ctx context.Context) {
		p.mu.Lock()
		s := p.sessions[util.GetContextSessionId(ctx)]
		if s != nil {
			s.mu.Lock()
			matches := s.query.Id == util.GetContextQueryId(ctx)
			s.mu.Unlock()
			// A delayed leave callback must not kill a new command opened in the same window.
			if matches {
				delete(p.sessions, util.GetContextSessionId(ctx))
				s.close()
			}
		}
		p.mu.Unlock()
	})
}

func (p *extensionPlugin) closeAll() {
	p.mu.Lock()
	p.closed = true
	sessions := p.sessions
	p.sessions = map[string]*commandSession{}
	p.mu.Unlock()
	for _, s := range sessions {
		s.close()
	}
	// Windows keeps the command's working directory locked until Node exits.
	// Uninstall/replacement may remove that directory immediately after this callback.
	for _, s := range sessions {
		select {
		case <-s.done:
		case <-time.After(2 * time.Second):
			util.GetLogger().Warn(s.logCtx, "Raycast command did not exit after termination")
		}
	}
}

// Query reuses the running React state when only search or refinement values changed.
func (p *extensionPlugin) Query(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	p.queryMu.Lock()
	defer p.queryMu.Unlock()
	if ctx.Err() != nil {
		return plugin.QueryResponse{}
	}
	commandName := query.Command
	var command *Command
	for index := range p.descriptor.Commands {
		if p.descriptor.Commands[index].Name == commandName {
			command = &p.descriptor.Commands[index]
			break
		}
	}
	waitingForArguments := command != nil && (command.needsSearchInput(query.Search) ||
		(len(command.Arguments) > 0 && !command.usesSearchArgument() && query.ContextData["raycast.arguments"] == ""))
	if command == nil || command.Mode == "no-view" || waitingForArguments {
		p.mu.Lock()
		if s := p.sessions[query.SessionId]; s != nil {
			delete(p.sessions, query.SessionId)
			s.close()
		}
		p.mu.Unlock()
		return p.commands(ctx, query)
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return plugin.QueryResponse{}
	}
	s := p.sessions[query.SessionId]
	fresh := s == nil || s.command.Name != commandName
	if s != nil {
		s.mu.Lock()
		fresh = fresh || s.closed
		s.mu.Unlock()
	}
	if fresh {
		if s != nil {
			s.close()
		}
		var err error
		s, err = p.startSession(ctx, query, *command)
		if err != nil {
			p.mu.Unlock()
			return p.failure(err)
		}
		p.sessions[query.SessionId] = s
	}
	s.mu.Lock()
	s.query, s.ctx = query, context.WithoutCancel(ctx)
	s.mu.Unlock()
	p.mu.Unlock()
	var reply runtimeMessage
	var err error
	if fresh {
		params, configErr := p.launchConfig(ctx, query, *command)
		if configErr != nil {
			s.close()
			return p.failure(configErr)
		}
		reply, err = s.call(ctx, "launch", params)
	} else {
		reply, err = s.call(ctx, "query", map[string]any{"search": query.Search, "refinements": query.Refinements})
	}
	if err != nil {
		if ctx.Err() != nil {
			return plugin.QueryResponse{}
		}
		s.mu.Lock()
		s.errorText = err.Error()
		s.mu.Unlock()
		return p.failure(err)
	}
	s.mu.Lock()
	if reply.Response != nil {
		s.response = *reply.Response
	}
	response := s.response
	s.published, _ = json.Marshal(response)
	errorText := s.errorText
	s.mu.Unlock()
	if errorText != "" {
		return p.failure(fmt.Errorf("%s", errorText))
	}
	return p.bind(s, response)
}

func (p *extensionPlugin) failure(err error) plugin.QueryResponse {
	return plugin.NewQueryResponse([]plugin.QueryResult{{Title: "i18n:plugin_raycast_failed", SubTitle: err.Error(), Icon: p.descriptor.Metadata.GetIconOrDefault(p.descriptor.Metadata.Directory, icons.Get(icons.PluginInstaller))}})
}

// commandQuery keeps the trigger and command visible so deleting them returns to ordinary discovery.
func (p *extensionPlugin) commandQuery(query plugin.Query, command string) string {
	keyword := query.TriggerKeyword
	if keyword == "" || keyword == "*" {
		keyword = p.descriptor.Manifest.Name
		if instance := plugin.GetPluginManager().GetPluginInstanceById(p.descriptor.Metadata.Id); instance != nil {
			keyword = instance.PrimaryTriggerKeyword()
		}
	}
	return strings.TrimSpace(keyword+" "+command) + " "
}

// commands creates ordinary searchable results; activation uses triggerkeyword + command + search.
func (p *extensionPlugin) commands(ctx context.Context, query plugin.Query) plugin.QueryResponse {
	var results []plugin.QueryResult
	for _, command := range p.descriptor.Commands {
		if query.Command != "" && query.Command != command.Name {
			continue
		}
		text := strings.ToLower(command.Title + " " + command.Name + " " + p.descriptor.Manifest.Title)
		match := true
		if query.Command == "" {
			for _, word := range strings.Fields(strings.ToLower(query.Search)) {
				if !strings.Contains(text, word) {
					match = false
				}
			}
		}
		if !match {
			continue
		}
		command := command
		action := plugin.QueryResultAction{Id: "launch", Name: "i18n:plugin_raycast_run", Icon: icons.Get(icons.ActionExecute), PreventHideAfterAction: true}
		launch := func(ctx context.Context, arguments map[string]string) {
			if command.Mode == "no-view" {
				go p.runNoView(context.WithoutCancel(ctx), query, command, arguments)
				return
			}
			raw, _ := json.Marshal(arguments)
			p.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput,
				QueryText:   p.commandQuery(query, command.Name) + query.Search,
				ContextData: common.ContextData{"raycast.arguments": string(raw)}})
		}
		if query.Command == "" || command.needsSearchInput(query.Search) {
			// Discovery always completes the command first, including headless commands.
			action.Action = func(ctx context.Context, _ plugin.ActionContext) {
				p.api.ChangeQuery(ctx, common.PlainQuery{QueryType: plugin.QueryTypeInput, QueryText: p.commandQuery(query, command.Name)})
			}
		} else if len(command.Arguments) > 0 && !command.usesSearchArgument() {
			action.Type = plugin.QueryResultActionTypeForm
			for _, argument := range command.Arguments {
				field, err := preferenceDefinition(argument, argument.Name)
				if err != nil {
					action.Form = nil
					break
				}
				action.Form = append(action.Form, field)
			}
			if len(action.Form) != len(command.Arguments) {
				continue
			}
			action.OnSubmit = func(ctx context.Context, ac plugin.FormActionContext) { launch(ctx, ac.Values) }
		} else {
			action.Action = func(ctx context.Context, _ plugin.ActionContext) { launch(ctx, nil) }
		}
		results = append(results, plugin.QueryResult{Id: "command:" + command.Name, IdentityKey: command.Name, Title: command.Title,
			SubTitle: p.descriptor.Manifest.Title + " · " + command.Description, Icon: assetIcon(p.descriptor.Metadata.Directory, command.Icon), Actions: []plugin.QueryResultAction{action}})
	}
	return plugin.NewQueryResponse(results)
}

// launchConfig resolves preferences before evaluating a bundle, matching Raycast's synchronous getter.
func (p *extensionPlugin) launchConfig(ctx context.Context, query plugin.Query, command Command) (map[string]any, error) {
	preferences := map[string]any{}
	read := func(items []Preference, prefix string) error {
		for _, pref := range items {
			value := p.api.GetSetting(ctx, prefix+pref.Name)
			if value == "" && pref.Default != nil {
				value = fmt.Sprint(pref.Default)
			}
			if value == "" && pref.Required != nil && *pref.Required {
				return fmt.Errorf("configure Raycast preference %q in Wox plugin settings", pref.Title)
			}
			if pref.Type == "checkbox" {
				preferences[pref.Name] = value == "true"
			} else {
				preferences[pref.Name] = value
			}
		}
		return nil
	}
	if err := read(p.descriptor.Manifest.Preferences, "preference:"); err != nil {
		return nil, err
	}
	if err := read(command.Preferences, "command:"+command.Name+":"); err != nil {
		return nil, err
	}
	arguments := map[string]string{}
	_ = json.Unmarshal([]byte(query.ContextData["raycast.arguments"]), &arguments)
	if command.usesSearchArgument() {
		arguments = map[string]string{command.Arguments[0].Name: query.Search}
	}
	for _, argument := range command.Arguments {
		if _, ok := arguments[argument.Name]; !ok {
			arguments[argument.Name] = ""
		}
	}
	colors := p.api.GetThemeColors(ctx, plugin.GetThemeColorsOption{})
	return map[string]any{"entry": filepath.Join(p.descriptor.Metadata.Directory, command.Name+".js"), "command": command,
		"extensionName": p.descriptor.Manifest.Name, "author": p.descriptor.Manifest.Author, "preferences": preferences, "arguments": arguments,
		"assetsPath": filepath.Join(p.descriptor.Metadata.Directory, "assets"), "supportPath": p.api.GetCacheFolder(ctx), "dark": colors.Dark,
		"search": query.Search, "refinements": query.Refinements}, nil
}

func (p *extensionPlugin) runNoView(ctx context.Context, query plugin.Query, command Command, arguments map[string]string) {
	raw, _ := json.Marshal(arguments)
	query.ContextData = common.ContextData{"raycast.arguments": string(raw)}
	params, err := p.launchConfig(ctx, query, command)
	if err != nil {
		p.api.Notify(ctx, err.Error())
		return
	}
	p.mu.Lock()
	if p.closed {
		p.mu.Unlock()
		return
	}
	s, err := p.startSession(ctx, query, command)
	if err != nil {
		p.mu.Unlock()
		p.api.Notify(ctx, err.Error())
		return
	}
	// Track headless processes too so disabling or uninstalling the plugin terminates them.
	p.sessions[s.prefix] = s
	p.mu.Unlock()
	defer func() { s.close(); p.mu.Lock(); delete(p.sessions, s.prefix); p.mu.Unlock() }()
	if _, err := s.call(ctx, "launch", params); err != nil {
		p.api.Notify(ctx, err.Error())
	}
}

// bind creates callbacks for this exact process; stale results cannot dispatch into a later command.
func (p *extensionPlugin) bind(s *commandSession, source plugin.QueryResponse) plugin.QueryResponse {
	// Deep-copy the wire snapshot before adding Go callbacks or normalizing icons.
	raw, _ := json.Marshal(source)
	var response plugin.QueryResponse
	_ = json.Unmarshal(raw, &response)
	for i := range response.Results {
		row := &response.Results[i]
		row.Id = s.prefix + row.Id
		row.Icon = mappedIcon(row.Icon, false)
		for j := range row.Actions {
			action := &row.Actions[j]
			id := action.Id
			action.Icon = mappedIcon(action.Icon, true)
			action.Action = func(ctx context.Context, _ plugin.ActionContext) { p.dispatch(ctx, s, id, nil) }
			action.OnSubmit = func(ctx context.Context, ac plugin.FormActionContext) { p.dispatch(ctx, s, id, ac.Values) }
		}
	}
	for i := range response.Refinements {
		for j := range response.Refinements[i].Options {
			option := &response.Refinements[i].Options[j]
			option.Icon = mappedIcon(option.Icon, false)
		}
	}
	return response
}

func mappedIcon(image common.WoxImage, action bool) common.WoxImage {
	if image.ImageType != "raycast" && image.ImageType != "" {
		return image
	}
	if image.ImageType == "raycast" {
		name := map[string]string{"Clipboard": icons.ActionCopy, "CopyClipboard": icons.ActionCopy, "Folder": icons.PluginFolder,
			"MagnifyingGlass": icons.ActionSearch, "Star": icons.ActionStar, "ArrowNe": icons.ActionOpen}[image.ImageData]
		if name != "" {
			return icons.Get(name)
		}
	}
	if action {
		return icons.Get(icons.ActionExecute)
	}
	return icons.Get(icons.PluginInstaller)
}

func (p *extensionPlugin) dispatch(ctx context.Context, s *commandSession, id string, values map[string]string) {
	if _, err := s.call(ctx, "action", map[string]any{"id": id, "values": values}); err != nil {
		p.api.Notify(ctx, err.Error())
	}
}

// publish uses UpdateResult for stable rows and RefreshQuery when membership, order or controls change.
func (p *extensionPlugin) publish(s *commandSession) {
	p.mu.Lock()
	s.mu.Lock()
	if s.closed || p.sessions[s.query.SessionId] != s || len(s.published) == 0 {
		s.mu.Unlock()
		p.mu.Unlock()
		return
	}
	next, _ := json.Marshal(s.response)
	if bytes.Equal(next, s.published) && s.errorText == "" {
		s.mu.Unlock()
		p.mu.Unlock()
		return
	}
	var previous plugin.QueryResponse
	_ = json.Unmarshal(s.published, &previous)
	response, ctx, errorText := s.response, s.ctx, s.errorText
	s.published = next
	s.mu.Unlock()
	p.mu.Unlock()
	stable := errorText == "" && reflect.DeepEqual(previous.Layout, response.Layout) && reflect.DeepEqual(previous.Refinements, response.Refinements) && len(previous.Results) == len(response.Results)
	if stable {
		for i, row := range response.Results {
			old := previous.Results[i]
			if row.Id != old.Id || row.Group != old.Group || row.Score != old.Score || row.GroupScore != old.GroupScore {
				stable = false
				break
			}
		}
	}
	if stable {
		for _, row := range p.bind(s, response).Results {
			if !p.api.UpdateResult(ctx, plugin.UpdatableResult{Id: row.Id, Title: &row.Title, SubTitle: &row.SubTitle, Icon: &row.Icon, Preview: &row.Preview, Tails: &row.Tails, Actions: &row.Actions}) {
				stable = false
				break
			}
		}
	}
	if !stable {
		p.api.RefreshQuery(ctx, plugin.RefreshQueryParam{PreserveSelectedIndex: true})
	}
}
