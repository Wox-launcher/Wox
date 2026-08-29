package shell

import (
	"context"
	"testing"
	"wox/common"
	"wox/plugin"
	"wox/setting/definition"
)

type shellQueryTestAPI struct {
	plugin.API
	settings     map[string]string
	changedQuery common.PlainQuery
}

func (a *shellQueryTestAPI) GetSetting(ctx context.Context, key string) string {
	return a.settings[key]
}

func (a *shellQueryTestAPI) GetTranslation(ctx context.Context, key string) string {
	return key
}

func (a *shellQueryTestAPI) SetSetting(ctx context.Context, option plugin.SetSettingOption) plugin.SetSettingResult {
	if a.settings == nil {
		a.settings = map[string]string{}
	}
	a.settings[option.Key] = option.Value
	return plugin.SetSettingResult{Success: true}
}

func (a *shellQueryTestAPI) Log(ctx context.Context, level plugin.LogLevel, msg string) {}

func (a *shellQueryTestAPI) Notify(ctx context.Context, description string) {}

func (a *shellQueryTestAPI) ChangeQuery(ctx context.Context, query common.PlainQuery) {
	a.changedQuery = query
}

func TestBuildEmptyCommandResultSurfacesWorkingDirectory(t *testing.T) {
	plugin := &ShellPlugin{}
	const workingDirectory = `D:\dev\apps`
	result := plugin.buildEmptyCommandResult(context.Background(), "powershell", workingDirectory)

	if result.Title != "i18n:plugin_shell_enter_command" {
		t.Fatalf("title = %q, want empty-command prompt", result.Title)
	}
	if result.SubTitle != "" {
		t.Fatalf("subtitle = %q, want empty so the list row stays title-only", result.SubTitle)
	}
	if len(result.Tails) != 0 {
		t.Fatalf("tails = %#v, want none on the empty-command row", result.Tails)
	}
	if result.Preview.PreviewData != "i18n:plugin_shell_enter_command_preview" {
		t.Fatalf("preview = %q, want enter-command instruction", result.Preview.PreviewData)
	}
	if !emptyCommandResultShowsDirectory(result, workingDirectory) {
		t.Fatalf("empty command result does not show working directory %q", workingDirectory)
	}
	if result.GroupScore <= 100 {
		t.Fatalf("GroupScore = %d, want above saved-command group score 100", result.GroupScore)
	}
	if !resultHasChangeWorkingDirectoryAction(result) {
		t.Fatal("empty command result is missing the change working directory action")
	}
}

func TestBuildHistoryTailsMarksOnlyRunningCommands(t *testing.T) {
	shellPlugin := &ShellPlugin{}
	tails := shellPlugin.buildHistoryTails(context.Background(), "running")
	if len(tails) != 1 || tails[0].Type != plugin.QueryResultTailTypeImage || tails[0].Image != common.RunningIcon || tails[0].Tooltip == "" {
		t.Fatalf("running tails = %#v, want one labeled running dot", tails)
	}
	if tails[0].ImageWidth == nil || *tails[0].ImageWidth != 10 || tails[0].ImageHeight == nil || *tails[0].ImageHeight != 10 {
		t.Fatalf("running tail size = %v x %v, want 10 x 10", tails[0].ImageWidth, tails[0].ImageHeight)
	}
	if tails := shellPlugin.buildHistoryTails(context.Background(), "completed"); len(tails) != 0 {
		t.Fatalf("completed tails = %#v, want none", tails)
	}
}

func TestEffectiveWorkingDirectoryUsesHomeAndConfiguredDefault(t *testing.T) {
	home := userHomeDirectory()
	if home == "" {
		t.Fatal("user home directory is empty")
	}

	plugin := &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{}}}
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != home {
		t.Fatalf("empty setting default = %q, want home %q", got, home)
	}

	configured := t.TempDir()
	plugin.api = &shellQueryTestAPI{settings: map[string]string{
		shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeCustom,
		shellDefaultWorkingDirectorySettingKey:     configured,
	}}
	resolved, ok := plugin.resolveWorkingDirectory(context.Background(), configured, false)
	if !ok {
		t.Fatalf("temp working directory is invalid: %s", configured)
	}
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != resolved {
		t.Fatalf("configured default = %q, want %q", got, resolved)
	}
	plugin.api = &shellQueryTestAPI{settings: map[string]string{
		shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeCustom,
		shellDefaultWorkingDirectorySettingKey:     `D:\missing-shell-default`,
	}}
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != home {
		t.Fatalf("invalid setting default = %q, want home %q", got, home)
	}
	if got := plugin.effectiveWorkingDirectory(context.Background(), `D:\explicit`); got != `D:\explicit` {
		t.Fatalf("explicit directory = %q, want explicit path", got)
	}

	legacyPath := t.TempDir()
	legacyResolved, ok := plugin.resolveWorkingDirectory(context.Background(), legacyPath, false)
	if !ok {
		t.Fatalf("legacy working directory is invalid: %s", legacyPath)
	}
	plugin.api = &shellQueryTestAPI{settings: map[string]string{shellDefaultWorkingDirectorySettingKey: legacyPath}}
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != legacyResolved {
		t.Fatalf("legacy path-only setting = %q, want custom path %q", got, legacyResolved)
	}

	lastUsed := t.TempDir()
	lastUsedResolved, ok := plugin.resolveWorkingDirectory(context.Background(), lastUsed, false)
	if !ok {
		t.Fatalf("last-used working directory is invalid: %s", lastUsed)
	}
	plugin.api = &shellQueryTestAPI{settings: map[string]string{shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeLastUsed}}
	plugin.rememberWorkingDirectory(context.Background(), lastUsed)
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != lastUsedResolved {
		t.Fatalf("last-used default = %q, want %q", got, lastUsedResolved)
	}
	plugin = &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeLastUsed}}}
	if got := plugin.effectiveWorkingDirectory(context.Background(), ""); got != home {
		t.Fatalf("empty last-used default = %q, want home %q", got, home)
	}
}

func TestQueryEmptyShellPromptUsesHomeDirectoryByDefault(t *testing.T) {
	home := userHomeDirectory()
	if home == "" {
		t.Fatal("user home directory is empty")
	}

	shellPlugin := &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{}}}
	response := shellPlugin.Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: ">",
		Search:         "",
	})
	if len(response.Results) == 0 {
		t.Fatal("empty > query returned no results")
	}
	if !emptyCommandResultShowsDirectory(response.Results[0], home) {
		t.Fatalf("empty > query does not show home directory %q", home)
	}
}

func TestBuildDefaultWorkingDirectoryDetailSettingFollowsMode(t *testing.T) {
	plugin := &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{}}}
	if item := plugin.buildDefaultWorkingDirectoryDetailSetting(context.Background()); !item.IsEmpty() {
		t.Fatalf("home mode detail = %+v, want empty setting", item)
	}

	plugin.api = &shellQueryTestAPI{settings: map[string]string{shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeLastUsed}}
	if item := plugin.buildDefaultWorkingDirectoryDetailSetting(context.Background()); !item.IsEmpty() {
		t.Fatalf("last-used mode detail = %+v, want empty setting", item)
	}

	plugin.api = &shellQueryTestAPI{settings: map[string]string{shellDefaultWorkingDirectoryModeSettingKey: shellDefaultWorkingDirectoryModeCustom}}
	item := plugin.buildDefaultWorkingDirectoryDetailSetting(context.Background())
	value, ok := item.Value.(*definition.PluginSettingValueDirPath)
	if !ok || value.Key != shellDefaultWorkingDirectorySettingKey {
		t.Fatalf("custom mode detail = %+v, want dirPath %q", item, shellDefaultWorkingDirectorySettingKey)
	}
}

func TestGetMetadataDefaultWorkingDirectoryIsHome(t *testing.T) {
	for _, item := range (&ShellPlugin{}).GetMetadata().SettingDefinitions {
		value, ok := item.Value.(*definition.PluginSettingValueSelect)
		if !ok || value.Key != shellDefaultWorkingDirectoryModeSettingKey {
			continue
		}
		if value.DefaultValue != shellDefaultWorkingDirectoryModeHome {
			t.Fatalf("default working directory mode = %q, want %q", value.DefaultValue, shellDefaultWorkingDirectoryModeHome)
		}
		return
	}
	t.Fatal("default working directory mode setting is missing")
}

func TestQueryEmptyShellPromptLeadsWithDirectoryAwarePlaceholder(t *testing.T) {
	shellPlugin := &ShellPlugin{
		api: &shellQueryTestAPI{settings: map[string]string{}},
	}
	const workingDirectory = `D:\dev\apps`
	response := shellPlugin.Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: ">",
		Search:         "",
		ContextData: common.ContextData{
			QueryContextWorkingDirectoryKey: workingDirectory,
		},
	})
	if len(response.Results) == 0 {
		t.Fatal("empty > query returned no results")
	}

	first := response.Results[0]
	if first.Title != "i18n:plugin_shell_enter_command" {
		t.Fatalf("first result title = %q, want empty-command prompt", first.Title)
	}
	if first.SubTitle != "" || len(first.Tails) != 0 {
		t.Fatalf("first result must be title-only, subtitle=%q tails=%#v", first.SubTitle, first.Tails)
	}
	if first.Preview.PreviewData != "i18n:plugin_shell_enter_command_preview" {
		t.Fatalf("preview = %q, want enter-command instruction", first.Preview.PreviewData)
	}
	if !emptyCommandResultShowsDirectory(first, workingDirectory) {
		t.Fatalf("first result does not show working directory %q", workingDirectory)
	}
	if !resultHasChangeWorkingDirectoryAction(first) {
		t.Fatal("empty > query is missing the change working directory action")
	}
}

func TestBuildChangeWorkingDirectoryActionUsesDirPathForm(t *testing.T) {
	api := &shellQueryTestAPI{settings: map[string]string{}}
	shellPlugin := &ShellPlugin{api: api}
	workingDirectory := t.TempDir()
	action := shellPlugin.buildChangeWorkingDirectoryAction(shellContextData{
		Command:          "git status",
		WorkingDirectory: workingDirectory,
	}, ">")
	if action.Type != plugin.QueryResultActionTypeForm || action.Id != "change_working_directory" {
		t.Fatalf("action = %+v, want form change_working_directory", action)
	}
	if len(action.Form) != 1 {
		t.Fatalf("form field count = %d, want 1", len(action.Form))
	}
	value, ok := action.Form[0].Value.(*definition.PluginSettingValueDirPath)
	if !ok || value.Key != shellFormWorkingDirKey || value.DefaultValue != workingDirectory {
		t.Fatalf("form field = %+v, want dirPath %q", action.Form[0], workingDirectory)
	}

	action.OnSubmit(context.Background(), plugin.FormActionContext{
		ActionContext: plugin.ActionContext{ContextData: action.ContextData},
		Values:        map[string]string{shellFormWorkingDirKey: workingDirectory},
	})
	if api.changedQuery.QueryText != "> git status" {
		t.Fatalf("changed query = %q, want command kept in the query box", api.changedQuery.QueryText)
	}
	if got := api.changedQuery.ContextData[QueryContextWorkingDirectoryKey]; got == "" {
		t.Fatalf("changed context = %#v, want selected working directory", api.changedQuery.ContextData)
	}
}

func TestQueryCommandResultIncludesChangeWorkingDirectoryAction(t *testing.T) {
	shellPlugin := &ShellPlugin{api: &shellQueryTestAPI{settings: map[string]string{}}}
	response := shellPlugin.Query(context.Background(), plugin.Query{
		Type:           plugin.QueryTypeInput,
		TriggerKeyword: ">",
		Search:         "ls",
	})
	if len(response.Results) == 0 {
		t.Fatal("command query returned no results")
	}
	if !resultHasChangeWorkingDirectoryAction(response.Results[0]) {
		t.Fatal("command result is missing the change working directory action")
	}
}

func TestShellQueryTextKeepsTriggerAndCommand(t *testing.T) {
	if got := shellQueryText(">", ""); got != "> " {
		t.Fatalf("empty command query = %q, want \"> \"", got)
	}
	if got := shellQueryText(">", "ls"); got != "> ls" {
		t.Fatalf("command query = %q, want \"> ls\"", got)
	}
}

func emptyCommandResultShowsDirectory(result plugin.QueryResult, workingDirectory string) bool {
	for _, tag := range result.Preview.PreviewTags {
		if tag.Label == workingDirectory {
			return true
		}
	}
	return false
}

func resultHasChangeWorkingDirectoryAction(result plugin.QueryResult) bool {
	for _, action := range result.Actions {
		if action.Id == "change_working_directory" && action.Type == plugin.QueryResultActionTypeForm {
			return true
		}
	}
	return false
}
