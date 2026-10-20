package extension

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
	"wox/database"
	"wox/plugin"
	"wox/util"
)

// fixture writes a prebuilt CommonJS command, matching the local import contract.
func fixture(t *testing.T, source string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range map[string]string{
		"package.json": `{"name":"example","title":"Example","author":"wox","commands":[{"name":"list","title":"List","mode":"view"}]}`,
		"list.js":      source,
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestManifestAndLocalCopy(t *testing.T) {
	source := fixture(t, "exports.default = () => null")
	d, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	if d.Metadata.Id != "raycast.wox.example" || d.Metadata.Runtime != string(Runtime) || len(d.Metadata.Commands) != 0 || d.Commands[0].Name != "list" {
		t.Fatalf("unexpected metadata: %+v", d.Metadata)
	}
	if keywords := d.Metadata.TriggerKeywords; len(keywords) != 1 || keywords[0] != "example" {
		t.Fatalf("default keywords = %v, want only the manifest name", keywords)
	}
	dest := t.TempDir()
	if err := copyExtension(context.Background(), source, dest); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(dest); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(source, "list.js")); err != nil {
		t.Fatal(err)
	}
	if _, err := Parse(source); err == nil {
		t.Fatal("source-only extension accepted")
	}
	if _, err := Parse(dest); err != nil {
		t.Fatalf("copy depends on original: %v", err)
	}
}

// TestCommandDefaults covers command hints, discovery, icon inheritance and explicit icon overrides.
func TestCommandDefaults(t *testing.T) {
	dir := fixture(t, "exports.default = () => null")
	manifest := `{"name":"example","title":"Example","author":"wox","icon":"plugin.png","commands":[
		{"name":"hidden","mode":"view","disabledByDefault":true},
		{"name":"list","title":"List","mode":"view"},
		{"name":"custom","title":"Custom","mode":"view","icon":"command.png","disabledByDefault":false}
	]}`
	for name, content := range map[string]string{
		"package.json": manifest, "custom.js": "exports.default = () => null",
		"plugin.png": "plugin icon", "command.png": "command icon",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	d, err := Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Commands) != 2 || len(d.Metadata.Commands) != 2 || d.Metadata.Commands[0].Command != "list" || d.Metadata.Commands[1].Command != "custom" {
		t.Fatalf("disabled command exposed in metadata: %+v", d.Metadata.Commands)
	}
	p := &extensionPlugin{descriptor: d}
	response := p.Query(context.Background(), plugin.Query{TriggerKeyword: "example"})
	if len(response.Results) != 2 || response.Results[0].Title != "List" || response.Results[1].Title != "Custom" {
		t.Fatalf("command discovery: %+v", response.Results)
	}
	for index, icon := range []string{"plugin.png", "command.png"} {
		want := common.NewWoxImageAbsolutePath(filepath.Join(dir, icon)).String()
		if got := response.Results[index].Icon.String(); got != want {
			t.Fatalf("command %d icon = %s, want %s", index, got, want)
		}
	}
	if got := p.Query(context.Background(), plugin.Query{TriggerKeyword: "example", Command: "hidden"}); len(got.Results) != 0 {
		t.Fatalf("disabled command exposed by direct query: %+v", got.Results)
	}
}

// TestLocalToothpickCommandDiscovery checks the actual local bundle without invoking Bluetooth actions.
func TestLocalToothpickCommandDiscovery(t *testing.T) {
	source := os.Getenv("WOX_RAYCAST_VERIFY_TOOTHPICK")
	if source == "" {
		t.Skip("set WOX_RAYCAST_VERIFY_TOOTHPICK to the local Toothpick extension directory")
	}
	dir := t.TempDir()
	if err := copyExtension(context.Background(), source, dir); err != nil {
		t.Fatal(err)
	}
	d, err := Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.Manifest.Name != "toothpick" || len(d.Commands) != 1 || len(d.Metadata.Commands) != 0 || d.Commands[0].Name != "manage-bluetooth-connections" {
		t.Fatalf("Toothpick enabled commands: %+v", d.Metadata.Commands)
	}
	a := &recordingAPI{changed: make(chan common.PlainQuery, 1)}
	p := &extensionPlugin{descriptor: d, api: a}
	response := p.commands(context.Background(), plugin.Query{TriggerKeyword: "toothpick"})
	if len(response.Results) != 1 || response.Results[0].Title != "Manage Bluetooth Connections" {
		t.Fatalf("Toothpick discovery: %+v", response.Results)
	}
	iconPath := filepath.Join(dir, "assets", "icon.png")
	if _, err := os.Stat(iconPath); err != nil {
		t.Fatal(err)
	}
	if got, want := response.Results[0].Icon.String(), common.NewWoxImageAbsolutePath(iconPath).String(); got != want {
		t.Fatalf("Toothpick icon = %s, want %s", got, want)
	}
	response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	if query := <-a.changed; query.QueryText != "toothpick " || !query.QueryScope.IsEmpty() {
		t.Fatalf("Toothpick launch query: %+v", query)
	}
	t.Logf("verified local Toothpick: %d manifest commands, one visible entry with the plugin icon and ordinary command activation", len(d.Manifest.Commands))
}

// TestArgumentRequiredFormats covers the installed format alongside source-manifest defaults.
func TestArgumentRequiredFormats(t *testing.T) {
	dir := fixture(t, "exports.default = () => null")
	for _, tc := range []struct {
		fields string
		want   bool
	}{{``, true}, {`,"optional":true`, false}, {`,"optional":false`, true}, {`,"required":false`, false}, {`,"required":true`, true}} {
		raw := `{"name":"example","title":"Example","author":"wox","commands":[{"name":"list","mode":"view","arguments":[{"name":"text","type":"text"` + tc.fields + `}]}]}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		d, err := Parse(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got := d.Commands[0].Arguments[0].Required; got == nil || *got != tc.want {
			t.Fatalf("argument %s: got %v, want %t", tc.fields, got, tc.want)
		}
	}
}

func TestImportOverlapAndRollback(t *testing.T) {
	root := t.TempDir()
	collection := filepath.Join(root, "plugins")
	if err := os.Mkdir(collection, 0700); err != nil {
		t.Fatal(err)
	}
	for _, source := range []string{root, collection} {
		if err := validateImportSource(source, collection); err == nil {
			t.Fatalf("overlapping source accepted: %s", source)
		}
	}
	if err := validateImportSource(t.TempDir(), collection); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "activate", true: "rollback"}[fail], func(t *testing.T) {
			dest, stage := filepath.Join(t.TempDir(), "installed"), fixture(t, "new")
			if err := os.Mkdir(dest, 0700); err != nil {
				t.Fatal(err)
			}
			if err := copyExtension(context.Background(), fixture(t, "old"), dest); err != nil {
				t.Fatal(err)
			}
			restored := false
			cause := errors.New("activation failed")
			err := replaceDirectory(stage, dest, func() error {
				if fail {
					return cause
				}
				return nil
			}, func() error { restored = true; return nil }, func(err error) { t.Error(err) })
			if errors.Is(err, cause) != fail || restored != fail {
				t.Fatalf("unexpected transaction result: %v, restored %v", err, restored)
			}
			raw, err := os.ReadFile(filepath.Join(dest, "list.js"))
			if err != nil || string(raw) != map[bool]string{false: "new", true: "old"}[fail] {
				t.Fatalf("incorrect installed contents: %s, %v", raw, err)
			}
		})
	}
}

func TestManifestRejectsUnsafeCommandsAndFiltersBackgroundModes(t *testing.T) {
	for _, commands := range []string{
		`[{"name":"../outside","mode":"view"}]`,
		`[{"name":"list","mode":"menu-bar"}]`,
		`[{"name":"list","mode":"no-view","interval":"1m"}]`,
		`[{"name":"list","mode":"view"},{"name":"LIST","mode":"view"}]`,
	} {
		dir := fixture(t, "")
		raw := `{"name":"example","title":"Example","author":"wox","commands":` + commands + `}`
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(raw), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Parse(dir); err == nil {
			t.Fatalf("accepted %s", commands)
		}
	}
}

func TestCopyRejectsLinksAndCancellation(t *testing.T) {
	source := fixture(t, "")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := copyExtension(ctx, source, t.TempDir()); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(source, "outside")); err != nil {
		t.Skip("symlink unavailable: " + err.Error())
	}
	if err := copyExtension(context.Background(), source, t.TempDir()); err == nil {
		t.Fatal("followed symlink")
	}
}

type recordingAPI struct {
	plugin.API
	mu       sync.Mutex
	cache    string
	settings map[string]string
	leave    func(context.Context)
	unload   func(context.Context)
	updates  chan plugin.UpdatableResult
	refresh  chan struct{}
	changed  chan common.PlainQuery
	copied   chan string
}

func (a *recordingAPI) OnLeavePluginQuery(_ context.Context, f func(context.Context))         { a.leave = f }
func (a *recordingAPI) OnUnload(_ context.Context, f func(context.Context))                   { a.unload = f }
func (*recordingAPI) OnSettingChanged(context.Context, func(context.Context, string, string)) {}
func (a *recordingAPI) GetCacheFolder(context.Context) string                                 { return a.cache }
func (*recordingAPI) GetThemeColors(context.Context, plugin.GetThemeColorsOption) plugin.GetThemeColorsResult {
	return plugin.GetThemeColorsResult{}
}
func (*recordingAPI) Notify(context.Context, string) {}
func (a *recordingAPI) GetSetting(_ context.Context, key string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settings[key]
}
func (a *recordingAPI) SetSetting(_ context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.settings[option.Key] = option.Value
	return plugin.SetSettingResult{Success: true}
}
func (a *recordingAPI) Copy(_ context.Context, params plugin.CopyParams)       { a.copied <- params.Text }
func (a *recordingAPI) ChangeQuery(_ context.Context, query common.PlainQuery) { a.changed <- query }
func (a *recordingAPI) UpdateResult(_ context.Context, result plugin.UpdatableResult) bool {
	a.updates <- result
	return true
}
func (a *recordingAPI) RefreshQuery(context.Context, plugin.RefreshQueryParam) {
	a.refresh <- struct{}{}
}

func testPlugin(t *testing.T, source string) (*extensionPlugin, *recordingAPI) {
	t.Helper()
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is required for the Raycast process tests")
	}
	bundle, _ := filepath.Abs("../../../../resource/hosts/raycast-host.js")
	if _, err := os.Stat(bundle); err != nil {
		t.Skip("build the Raycast runtime before process tests")
	}
	d, err := Parse(fixture(t, source))
	if err != nil {
		t.Fatal(err)
	}
	a := &recordingAPI{cache: t.TempDir(), settings: map[string]string{}, updates: make(chan plugin.UpdatableResult, 100), refresh: make(chan struct{}, 100), changed: make(chan common.PlainQuery, 10), copied: make(chan string, 10)}
	p := &extensionPlugin{descriptor: d, node: node, bundle: bundle, sessions: map[string]*commandSession{}}
	p.Init(context.Background(), plugin.InitParams{API: a})
	t.Cleanup(p.closeAll)
	return p, a
}

func queryContext(session, query string) context.Context {
	return util.WithQueryIdContext(util.WithSessionContext(util.NewTraceContext(), session), query)
}

func TestReactProcessQueryActionAndSessionIsolation(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react'); const {List,Action,ActionPanel,Clipboard}=require('@raycast/api');const h=R.createElement;
	exports.default=function(){const [n,set]=R.useState(0);return h(List,null,h(List.Item,{id:'counter',title:'Count '+n,actions:h(ActionPanel,null,h(Action,{title:'Increment',onAction:async()=>{await Clipboard.copy('clicked');set(n+1)}}))}));}`)
	ctx := queryContext("first", "q1")
	response := p.Query(ctx, plugin.Query{Id: "q1", SessionId: "first", Command: "list"})
	if len(response.Results) != 1 || response.Results[0].Title != "Count 0" {
		t.Fatalf("query: %+v", response)
	}
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	select {
	case text := <-api.copied:
		if text != "clicked" {
			t.Fatal(text)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no API callback")
	}
	select {
	case result := <-api.updates:
		if result.Title == nil || *result.Title != "Count 1" {
			t.Fatalf("update: %+v", result)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no result update")
	}
	other := p.Query(queryContext("second", "q2"), plugin.Query{Id: "q2", SessionId: "second", Command: "list"})
	if other.Results[0].Title != "Count 0" || other.Results[0].Id == response.Results[0].Id {
		t.Fatal("command state leaked across launcher windows")
	}
	p.mu.Lock()
	first, second := p.sessions["first"], p.sessions["second"]
	p.mu.Unlock()
	api.leave(ctx)
	select {
	case <-first.done:
	case <-time.After(3 * time.Second):
		t.Fatal("leaving did not release process")
	}
	select {
	case <-second.done:
		t.Fatal("leaving one window killed another")
	default:
	}
	if _, err := first.call(ctx, "action", map[string]any{"id": "old"}); err == nil {
		t.Fatal("stale callback accepted")
	}
	api.unload(context.Background())
	select {
	case <-second.done:
	case <-time.After(3 * time.Second):
		t.Fatal("unload did not release process")
	}
}

func TestStructuralUpdatesRefreshAndReuseReactState(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {List,Action,ActionPanel}=require('@raycast/api');const h=R.createElement;
	exports.default=function(){const [count,set]=R.useState(1);return h(List,null,...Array.from({length:count},(_,i)=>h(List.Item,{key:i,id:String(i),title:'Item '+i,actions:h(ActionPanel,null,h(Action,{title:'Add',onAction:()=>set(count+1)}))})));}`)
	ctx := queryContext("window", "q1")
	query := plugin.Query{Id: "q1", SessionId: "window", Command: "list"}
	first := p.Query(ctx, query)
	first.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	select {
	case <-api.refresh:
	case <-time.After(3 * time.Second):
		t.Fatal("list membership change did not refresh")
	}
	query.Id = "q2"
	next := p.Query(queryContext("window", "q2"), query)
	if len(next.Results) != 2 {
		t.Fatalf("React state restarted during refresh: %+v", next)
	}
	api.leave(ctx)
	p.mu.Lock()
	active := p.sessions["window"]
	p.mu.Unlock()
	if active == nil {
		t.Fatal("late leave killed new query")
	}
}

// TestAccessoryUpdates keeps image tails normalized when existing rows refresh through UpdateResult.
func TestAccessoryUpdates(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {List,Icon,Action,ActionPanel}=require('@raycast/api');const h=R.createElement;
	exports.default=()=>{const [value,setValue]=R.useState('0%');return h(List,null,h(List.Item,{id:'process',title:'Process',
	accessories:[{icon:Icon.Clipboard,text:value,tooltip:'Usage'}],
	actions:h(ActionPanel,null,h(Action,{title:'Update',onAction:()=>setValue('5%')}))}));};`)
	ctx := queryContext("tails-window", "tails-query")
	response := p.Query(ctx, plugin.Query{Id: "tails-query", SessionId: "tails-window", TriggerKeyword: "example"})
	if len(response.Results) != 1 || len(response.Results[0].Tails) != 2 {
		t.Fatalf("missing icon/value tails: %+v", response)
	}
	image := response.Results[0].Tails[0].Image
	if image.ImageType != common.WoxImageTypeSvg {
		t.Fatalf("built-in accessory icon was not mapped to Wox: %+v", image)
	}
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	select {
	case update := <-api.updates:
		if update.Tails == nil || len(*update.Tails) != 2 || (*update.Tails)[0].Image != image || (*update.Tails)[1].Text != "5%" ||
			(*update.Tails)[0].Tooltip != "Usage" || (*update.Tails)[1].Tooltip != "Usage" {
			t.Fatalf("accessory update lost its image, text or tooltip: %+v", update.Tails)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("accessory change did not use UpdateResult")
	}
}

func TestCommandActivationAndPreferenceMapping(t *testing.T) {
	p, api := testPlugin(t, "exports.default=()=>null")
	p.descriptor.Commands = append(p.descriptor.Commands, Command{Name: "other", Title: "Other", Mode: "view"})
	response := p.Query(context.Background(), plugin.Query{Search: "list", Type: plugin.QueryTypeInput})
	response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	query := <-api.changed
	if !query.QueryScope.IsEmpty() || query.QueryText != "example list " {
		t.Fatalf("not routed through ordinary Wox input: %+v", query)
	}
	response = p.Query(context.Background(), plugin.Query{TriggerKeyword: "custom", Type: plugin.QueryTypeInput})
	response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
	if query = <-api.changed; !query.QueryScope.IsEmpty() || query.QueryText != "custom list " {
		t.Fatalf("custom trigger lost: %+v", query)
	}
	_, err := p.invokeAPI(context.Background(), plugin.Query{TriggerKeyword: "custom", Command: "list", Search: "filter"}, "clearSearch", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if query = <-api.changed; !query.QueryScope.IsEmpty() || query.QueryText != "custom list " {
		t.Fatalf("clearSearch removed command: %+v", query)
	}
	var pref Preference
	if err := json.Unmarshal([]byte(`{"name":"enabled","title":"Enabled","type":"checkbox","default":false}`), &pref); err != nil {
		t.Fatal(err)
	}
	p.descriptor.Manifest.Preferences = []Preference{pref}
	config, err := p.launchConfig(context.Background(), plugin.Query{}, p.descriptor.Commands[0])
	if err != nil {
		t.Fatal(err)
	}
	if config["preferences"].(map[string]any)["enabled"] != false {
		t.Fatal("checkbox default lost its type")
	}
	if !strings.HasSuffix(config["entry"].(string), "list.js") {
		t.Fatal("wrong command entry")
	}
}

// TestCacheSubscriptionUsesWoxRefresh verifies the whole cache-to-React-to-Wox path and text-query exit.
func TestCacheSubscriptionUsesWoxRefresh(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const fs=require('node:fs');const path=require('node:path');
	const {List,ActionPanel,Action,Cache,environment}=require('@raycast/api');const h=R.createElement;
	const reader=new Cache(),writer=new Cache();writer.set('rows',JSON.stringify(['First']));
	exports.default=function(){const rows=JSON.parse(R.useSyncExternalStore(reader.subscribe,()=>reader.get('rows')));
	R.useEffect(()=>()=>fs.writeFileSync(path.join(environment.supportPath,'unmounted'),'yes'),[]);
	return h(List,null,...rows.map((title,i)=>h(List.Item,{id:String(i),key:i,title,actions:h(ActionPanel,null,
	h(Action,{title:'Rename',onAction:()=>writer.set('rows',JSON.stringify(['Renamed']))}),
	h(Action,{title:'Add',onAction:()=>writer.set('rows',JSON.stringify(['Renamed','Second']))}))})));};`)
	p.descriptor.Commands = append(p.descriptor.Commands, Command{Name: "other", Title: "Other", Mode: "view"})
	ctx := queryContext("cache-window", "cache-query")
	query := plugin.Query{Id: "cache-query", SessionId: "cache-window", TriggerKeyword: "example", Command: "list"}
	response := p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Title != "First" {
		t.Fatalf("initial cache snapshot: %+v", response)
	}
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	select {
	case updated := <-api.updates:
		if updated.Title == nil || *updated.Title != "Renamed" {
			t.Fatalf("cache notification did not update row: %+v", updated)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cache change did not call UpdateResult")
	}
	response.Results[0].Actions[1].Action(ctx, plugin.ActionContext{})
	select {
	case <-api.refresh:
	case <-time.After(3 * time.Second):
		t.Fatal("cache membership change did not call RefreshQuery")
	}
	response = p.Query(ctx, query)
	if len(response.Results) != 2 {
		t.Fatalf("refresh lost cached rows: %+v", response)
	}
	p.mu.Lock()
	session := p.sessions[query.SessionId]
	p.mu.Unlock()
	query.Command, query.Search = "", ""
	response = p.Query(ctx, query)
	if len(response.Results) != 2 || response.Results[0].Id != "command:list" {
		t.Fatalf("deleting command did not return to discovery: %+v", response)
	}
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		t.Fatal("returning to command discovery retained the process")
	}
	if _, err := os.Stat(filepath.Join(api.cache, "unmounted")); err != nil {
		t.Fatalf("React cleanup was skipped: %v", err)
	}
}

// TestOptionalSearchArgumentUsesQueryInput covers activation, direct input and subsequent edits in one React session.
func TestOptionalSearchArgumentUsesQueryInput(t *testing.T) {
	p, _ := testPlugin(t, `const R=require('react');const {List}=require('@raycast/api');const h=R.createElement;
	exports.default=props=>{const [text,setText]=R.useState(props.arguments.query);return h(List,{filtering:false,onSearchTextChange:setText},h(List.Item,{id:'search',title:'Search: '+text,subtitle:'Initial: '+props.arguments.query}));};`)
	required := false
	p.descriptor.Commands[0].Arguments = []Preference{{Name: "query", Type: "text", Required: &required}}
	ctx := queryContext("search-window", "q1")
	query := plugin.Query{Id: "q1", SessionId: "search-window", TriggerKeyword: "custom", Search: "first search"}
	response := p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Title != "Search: first search" || response.Results[0].SubTitle != "Initial: first search" {
		t.Fatalf("initial input was not passed as an argument: %+v", response.Results)
	}
	session := p.sessions[query.SessionId]
	for index, search := range []string{"second search", "", "search again"} {
		query.Id, query.Search = fmt.Sprintf("q%d", index+2), search
		response = p.Query(queryContext(query.SessionId, query.Id), query)
		if len(response.Results) != 1 || response.Results[0].Title != "Search: "+search || p.sessions[query.SessionId] != session {
			t.Fatalf("search edit %q did not update the existing view: %+v", search, response.Results)
		}
	}
}

// TestRequiredSearchArgumentWaitsForInput avoids launching a command until its text argument is present.
func TestRequiredSearchArgumentWaitsForInput(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {List}=require('@raycast/api');const h=R.createElement;
	exports.default=props=>h(List,{filtering:false},h(List.Item,{title:props.arguments.query}));`)
	required := true
	p.descriptor.Commands[0].Arguments = []Preference{{Name: "query", Type: "text", Required: &required}}
	query := plugin.Query{Id: "required", SessionId: "required-window", TriggerKeyword: "example"}
	ctx := queryContext(query.SessionId, query.Id)
	for _, search := range []string{"", "  "} {
		query.Search = search
		response := p.Query(ctx, query)
		if len(response.Results) != 1 || response.Results[0].Actions[0].Type == plugin.QueryResultActionTypeForm || len(p.sessions) != 0 {
			t.Fatalf("empty required text must wait in query input: %+v", response)
		}
		response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
		if completed := <-api.changed; completed.QueryText != "example " {
			t.Fatalf("completion: %+v", completed)
		}
	}
	query.Search = "required text"
	response := p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Title != query.Search {
		t.Fatalf("required text was not supplied: %+v", response)
	}
	session := p.sessions[query.SessionId]
	query.Search = ""
	response = p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Id != "command:list" || len(p.sessions) != 0 {
		t.Fatalf("clearing required text did not return to input: %+v", response)
	}
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		t.Fatal("clearing required text retained the command process")
	}
}

// TestHeadlessCommandCompletesBeforeExecution ensures typing never executes a no-view command.
func TestHeadlessCommandCompletesBeforeExecution(t *testing.T) {
	p, api := testPlugin(t, `const {Clipboard}=require('@raycast/api');exports.default=async props=>Clipboard.copy(props.arguments.query);`)
	required := true
	p.descriptor.Commands[0].Mode = "no-view"
	p.descriptor.Commands[0].Arguments = []Preference{{Name: "query", Type: "text", Required: &required}}
	ctx := queryContext("headless-window", "headless")
	query := plugin.Query{Id: "headless", SessionId: "headless-window", TriggerKeyword: "example"}
	response := p.Query(ctx, query)
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	if completed := <-api.changed; completed.QueryText != "example " {
		t.Fatalf("headless completion: %+v", completed)
	}
	query.Search = "text with spaces"
	response = p.Query(ctx, query)
	select {
	case <-api.copied:
		t.Fatal("headless command executed before Enter")
	default:
	}
	if response.Results[0].Actions[0].Type == plugin.QueryResultActionTypeForm {
		t.Fatal("single text headless command opened a form")
	}
	response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{})
	select {
	case copied := <-api.copied:
		if copied != query.Search {
			t.Fatalf("argument = %q, want %q", copied, query.Search)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("headless command did not execute on Enter")
	}
}

// TestStructuredCommandArgumentsKeepForms preserves validation for multiple and non-text arguments.
func TestStructuredCommandArgumentsKeepForms(t *testing.T) {
	optional := false
	for _, arguments := range [][]Preference{
		{{Name: "first", Type: "text", Required: &optional}, {Name: "second", Type: "text", Required: &optional}},
		{{Name: "secret", Type: "password", Required: &optional}},
	} {
		api := &recordingAPI{changed: make(chan common.PlainQuery, 1)}
		p := &extensionPlugin{api: api, descriptor: Descriptor{Commands: []Command{{Name: "list", Mode: "view", Arguments: arguments}, {Name: "other", Mode: "view"}}}}
		discovery := p.Query(context.Background(), plugin.Query{TriggerKeyword: "example"})
		action := discovery.Results[0].Actions[0]
		if action.Type == plugin.QueryResultActionTypeForm {
			t.Fatal("discovery opened the argument form before completing the command")
		}
		action.Action(context.Background(), plugin.ActionContext{})
		if completed := <-api.changed; completed.QueryText != "example list " {
			t.Fatalf("structured command completion: %+v", completed)
		}
		response := p.Query(context.Background(), plugin.Query{Command: "list"})
		if len(response.Results) != 1 || response.Results[0].Actions[0].Type != plugin.QueryResultActionTypeForm {
			t.Fatalf("arguments %+v lost their form: %+v", arguments, response.Results)
		}
		p.descriptor.Commands = p.descriptor.Commands[:1]
		response = p.Query(context.Background(), plugin.Query{TriggerKeyword: "example"})
		if len(response.Results) != 1 || response.Results[0].Actions[0].Type != plugin.QueryResultActionTypeForm {
			t.Fatalf("single command arguments %+v lost their form: %+v", arguments, response.Results)
		}
	}
}

// TestLocalYouTubeSearchActivation checks the actual manifest and launch arguments without requesting YouTube data.
func TestLocalYouTubeSearchActivation(t *testing.T) {
	directory := os.Getenv("WOX_RAYCAST_VERIFY_YOUTUBE")
	if directory == "" {
		t.Skip("set WOX_RAYCAST_VERIFY_YOUTUBE to the local YouTube extension directory")
	}
	d, err := Parse(directory)
	if err != nil {
		t.Fatal(err)
	}
	a := &recordingAPI{cache: t.TempDir(), changed: make(chan common.PlainQuery, 1), settings: map[string]string{"preference:apikey": "test-only"}}
	p := &extensionPlugin{descriptor: d, api: a}
	for _, name := range []string{"search-videos", "search-channel", "search-live"} {
		response := p.commands(context.Background(), plugin.Query{TriggerKeyword: "youtube", Command: name})
		if len(response.Results) != 1 || response.Results[0].Actions[0].Type == plugin.QueryResultActionTypeForm {
			t.Fatalf("%s must activate without a form: %+v", name, response.Results)
		}
		response.Results[0].Actions[0].Action(context.Background(), plugin.ActionContext{})
		if query := <-a.changed; query.QueryText != "youtube "+name+" " || !query.QueryScope.IsEmpty() {
			t.Fatalf("%s activation: %+v", name, query)
		}
		for _, command := range d.Commands {
			if command.Name != name {
				continue
			}
			params, err := p.launchConfig(context.Background(), plugin.Query{Search: "Wox launcher"}, command)
			if err != nil || params["arguments"].(map[string]string)["query"] != "Wox launcher" {
				t.Fatalf("%s launch arguments: %v", name, err)
			}
		}
	}
}

func TestFormActionUsesWoxDefinitionsAndSubmit(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {Form,Action,ActionPanel,Clipboard}=require('@raycast/api');const h=R.createElement;
	exports.default=()=>h(Form,{actions:h(ActionPanel,null,h(Action.SubmitForm,{onSubmit:values=>Clipboard.copy(JSON.stringify(values))}))},h(Form.TextField,{id:'name',title:'Name'}),h(Form.Checkbox,{id:'enabled',title:'Enabled'}));`)
	ctx := queryContext("form-window", "form-query")
	response := p.Query(ctx, plugin.Query{Id: "form-query", SessionId: "form-window", Command: "list"})
	if len(response.Results) != 1 || len(response.Results[0].Actions) != 1 {
		t.Fatalf("missing form action: %+v", response)
	}
	action := response.Results[0].Actions[0]
	if action.Type != plugin.QueryResultActionTypeForm || len(action.Form) != 2 || action.Form[1].Value == nil {
		t.Fatalf("form definition did not survive the process protocol: %+v", action)
	}
	action.OnSubmit(ctx, plugin.FormActionContext{Values: map[string]string{"name": "Wox", "enabled": "true"}})
	select {
	case text := <-api.copied:
		var values map[string]any
		if err := json.Unmarshal([]byte(text), &values); err != nil || values["enabled"] != true || values["name"] != "Wox" {
			t.Fatalf("incorrect submit values: %s (%v)", text, err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("form submission did not invoke the command")
	}
}

// TestLocalInstallRunAndUninstall imports directly and verifies host loading, settings and normal uninstall.
func TestLocalInstallRunAndUninstall(t *testing.T) {
	// Global settings and logging hold process-lifetime file handles. Isolate them so
	// the parent can remove the entire test data directory after Windows closes those handles.
	const dataEnv = "WOX_RAYCAST_INSTALL_TEST_DATA"
	if os.Getenv(dataEnv) == "" {
		command := exec.Command(os.Args[0], "-test.run=^TestLocalInstallRunAndUninstall$", "-test.v", "-test.timeout=30s")
		command.Env = append(os.Environ(), dataEnv+"="+t.TempDir())
		output, err := command.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated install test: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	p, _ := testPlugin(t, "exports.default=()=>null")
	ctx, cancel := context.WithTimeout(queryContext("installed-window", "installed-query"), 15*time.Second)
	defer cancel()
	data := os.Getenv(dataEnv)
	t.Setenv(util.TestWoxDataDirEnv, data)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(data, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	if err := database.Init(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if db, err := database.GetDB().DB(); err == nil {
			_ = db.Close()
		}
	})
	raw, err := os.ReadFile(p.bundle)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(util.GetLocation().GetHostDirectory(), "raycast-host.js"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	oldResolver, oldHosts := plugin.ResolveNodePath, plugin.AllHosts
	plugin.ResolveNodePath = func(context.Context) (string, error) { return p.node, nil }
	plugin.RegisterThirdParty(Layer{})
	t.Cleanup(func() {
		for _, host := range plugin.AllHosts[len(oldHosts):] {
			host.Stop(context.Background())
		}
		plugin.ResolveNodePath, plugin.AllHosts = oldResolver, oldHosts
	})
	source := fixture(t, `const R=require('react');const {List}=require('@raycast/api');exports.default=()=>R.createElement(List,null,R.createElement(List.Item,{title:'Installed and running'}));`)
	manifest := `{"name":"example","title":"Example","author":"wox","preferences":[{"name":"first","type":"textfield"},{"name":"second","type":"password"}],"commands":[{"name":"list","title":"List","mode":"view"}]}`
	if err := os.WriteFile(filepath.Join(source, "package.json"), []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	store := plugin.GetStoreManager()
	descriptor, err := Parse(source)
	if err != nil {
		t.Fatal(err)
	}
	metadata := descriptor.Metadata
	preferences := map[string]json.RawMessage{"preference:first": json.RawMessage(`"first"`), "preference:second": json.RawMessage(`"fixture-secret"`)}
	if err := database.GetDB().Exec(`CREATE TRIGGER reject_imported_preference BEFORE INSERT ON oplogs WHEN NEW.key = 'preference:second' BEGIN SELECT RAISE(ABORT, 'rejected'); END`).Error; err != nil {
		t.Fatal(err)
	}
	if err := Import(ctx, source, preferences); err == nil {
		t.Fatal("import succeeded despite a settings transaction failure")
	}
	if plugin.GetPluginManager().GetPluginInstanceById(metadata.Id) != nil {
		t.Fatal("failed import left the plugin loaded")
	}
	if _, err := os.Stat(filepath.Join(CollectionDirectory(), metadata.Id)); !os.IsNotExist(err) {
		t.Fatal("failed import left installed files")
	}
	for _, model := range []any{&database.PluginSetting{}, &database.Oplog{}} {
		var count int64
		if err := database.GetDB().Model(model).Where("key IN ?", []string{"preference:first", "preference:second"}).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("failed import left partial preferences or sync records")
		}
	}
	if err := database.GetDB().Exec("DROP TRIGGER reject_imported_preference").Error; err != nil {
		t.Fatal(err)
	}
	if err := Import(ctx, source, preferences); err != nil {
		t.Fatal(err)
	}
	manager := plugin.GetPluginManager()
	instance := manager.GetPluginInstanceById(metadata.Id)
	if instance == nil {
		t.Fatal("installer did not register plugin")
	}
	t.Cleanup(func() {
		if remaining := manager.GetPluginInstanceById(metadata.Id); remaining != nil {
			manager.UnloadPlugin(context.Background(), remaining)
		}
	})
	if err := instance.WaitInit(ctx); err != nil {
		t.Fatal(err)
	}
	if value, exists := instance.Setting.Get("preference:second"); !exists || value != "fixture-secret" {
		t.Fatal("successful import did not persist preferences")
	}
	if instance.PluginDirectory == source {
		t.Fatal("installation still references source")
	}
	response := instance.Plugin.Query(ctx, plugin.Query{Id: "installed-query", SessionId: "installed-window", Command: "list"})
	if len(response.Results) != 1 || response.Results[0].Title != "Installed and running" {
		t.Fatalf("installed command failed: %+v", response)
	}
	if result := instance.API.SetSetting(ctx, plugin.SetSettingOption{Key: "test-kept", Value: "kept", IsLocal: true}); !result.Success {
		t.Fatalf("persist setting: %s", result.ErrMsg)
	}
	if err := os.WriteFile(filepath.Join(source, "list.js"), []byte(`const R=require('react');const {List}=require('@raycast/api');exports.default=()=>R.createElement(List,null,R.createElement(List.Item,{title:'Reimported'}));`), 0600); err != nil {
		t.Fatal(err)
	}
	preferences["preference:first"] = json.RawMessage(`"replacement"`)
	if err := Import(ctx, source, preferences); err != nil {
		t.Fatalf("replace a running extension: %v", err)
	}
	instance = manager.GetPluginInstanceById(metadata.Id)
	if err := instance.WaitInit(ctx); err != nil {
		t.Fatal(err)
	}
	response = instance.Plugin.Query(ctx, plugin.Query{Id: "installed-query", SessionId: "installed-window", Command: "list"})
	if len(response.Results) != 1 || response.Results[0].Title != "Reimported" || instance.API.GetSetting(ctx, "test-kept") != "kept" {
		t.Fatalf("replacement did not preserve settings and reload code: %+v", response)
	}
	if value, _ := instance.Setting.Get("preference:first"); value != "first" {
		t.Fatal("replacement overwrote an existing preference")
	}
	if err := store.UninstallLocal(ctx, instance, false); err != nil {
		t.Fatal(err)
	}
	if manager.GetPluginInstanceById(metadata.Id) != nil {
		t.Fatal("uninstalled instance remains loaded")
	}
	if _, err := os.Stat(instance.PluginDirectory); !os.IsNotExist(err) {
		t.Fatalf("uninstalled files remain: %v", err)
	}
	if _, err := Parse(source); err != nil {
		t.Fatalf("uninstall altered original extension: %v", err)
	}
}
