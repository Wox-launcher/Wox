package dotnet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"

	"wox/common"
)

func TestDotNetPluginRoundTrip(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("dotnet flow host runs on Windows")
	}
	sdk := dotnetSDKPath()
	dotnet, err := exec.LookPath("dotnet")
	if sdk == "" || err != nil {
		t.Skip("dotnet SDK or runtime was not found")
	}
	hostDir := publishedHostDir(t)
	pluginDir := t.TempDir()
	if err := buildDotNetFixture(sdk, pluginDir); err != nil {
		t.Fatal(err)
	}
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{
		Name:       "fixture",
		Directory:  pluginDir,
		Entry:      "Fixture.dll",
		PluginID:   "fixture",
		Language:   "csharp",
		UILanguage: "zh_CN",
		Keyword:    "*",
		DotNetPath: dotnet,
		HostDir:    hostDir,
		Bridge:     bridge,
	})
	defer session.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := session.Start(ctx); err != nil {
		t.Fatal(err)
	}
	results, err := session.Query(ctx, "world", "kw world", "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "hello world" || results[0].Score != 7 || results[0].CopyText != "copied" {
		t.Fatalf("results %+v", results)
	}
	if len(results[0].Actions) != 2 || results[0].Actions[0].Name != "Run" || results[0].Actions[1].Name != "更多" {
		t.Fatalf("actions %+v", results[0].Actions)
	}
	rows := dotnetQueryResults(pluginDir, common.WoxImage{}, results, session)
	if len(rows) != 1 || len(rows[0].Actions) != 3 || !rows[0].Actions[0].PreventHideAfterAction || rows[0].Actions[0].Name != "Run" || rows[0].Actions[2].Name != "Copy" {
		t.Fatalf("mapped %+v", rows)
	}
	if err := session.Action(ctx, results[0].Actions[0].ID); err != nil {
		t.Fatal(err)
	}
	bridge.wait(t)
}

type recordingBridge struct {
	mu           sync.Mutex
	query        string
	hide         bool
	refreshCount int
	updates      []recordedResults
}

type recordedResults struct {
	search  string
	results []dotnetResult
}

func (b *recordingBridge) ChangeQuery(ctx context.Context, query string) {
	b.mu.Lock()
	b.query = query
	b.mu.Unlock()
}

func (b *recordingBridge) HideApp(ctx context.Context) {
	b.mu.Lock()
	b.hide = true
	b.mu.Unlock()
}

func (b *recordingBridge) ShowApp(ctx context.Context)                        {}
func (b *recordingBridge) Notify(ctx context.Context, title, subtitle string) {}
func (b *recordingBridge) CopyText(ctx context.Context, text string)          {}
func (b *recordingBridge) OpenPath(ctx context.Context, target string) error  { return nil }
func (b *recordingBridge) OpenDirectory(ctx context.Context, directory, fileName string) error {
	return nil
}
func (b *recordingBridge) ShellRun(ctx context.Context, program, command string) error { return nil }
func (b *recordingBridge) Refresh(ctx context.Context) {
	b.mu.Lock()
	b.refreshCount++
	b.mu.Unlock()
}
func (b *recordingBridge) UpdateResults(ctx context.Context, search string, results []dotnetResult) {
	b.mu.Lock()
	copied := append([]dotnetResult(nil), results...)
	b.updates = append(b.updates, recordedResults{search: search, results: copied})
	b.mu.Unlock()
}

func (b *recordingBridge) wait(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		done := b.hide && b.query == "next"
		b.mu.Unlock()
		if done {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	t.Fatalf("bridge query=%q hide=%v", b.query, b.hide)
}

func (b *recordingBridge) waitResults(t *testing.T, title string) recordedResults {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		b.mu.Lock()
		var found recordedResults
		ok := false
		for _, update := range b.updates {
			for _, row := range update.results {
				if row.Title == title {
					found = update
					ok = true
				}
			}
		}
		refresh := b.refreshCount
		b.mu.Unlock()
		if ok {
			if refresh != 0 {
				t.Fatalf("result update refreshed the query %d times", refresh)
			}
			return found
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("result update %q was not delivered", title)
	return recordedResults{}
}

func TestDotNetPluginResultUpdate(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("dotnet flow host runs on Windows")
	}
	sdk := dotnetSDKPath()
	dotnet, err := exec.LookPath("dotnet")
	if sdk == "" || err != nil {
		t.Skip("dotnet SDK or runtime was not found")
	}
	hostDir := publishedHostDir(t)
	pluginDir := t.TempDir()
	if err := buildDotNetFixture(sdk, pluginDir); err != nil {
		t.Fatal(err)
	}
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{
		Name:       "fixture",
		Directory:  pluginDir,
		Entry:      "Fixture.dll",
		PluginID:   "fixture",
		Language:   "csharp",
		UILanguage: "zh_CN",
		Keyword:    "*",
		DotNetPath: dotnet,
		HostDir:    hostDir,
		Bridge:     bridge,
	})
	defer session.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := session.Start(ctx); err != nil {
		t.Fatal(err)
	}

	errCh := make(chan error, 1)
	go func() {
		results, queryErr := session.Query(ctx, "preview", "kw preview", "*")
		if queryErr != nil {
			errCh <- queryErr
			return
		}
		if len(results) != 1 || results[0].Title != "final" {
			errCh <- fmt.Errorf("query returned %#v", results)
			return
		}
		errCh <- nil
	}()
	preview := bridge.waitResults(t, "partial")
	if preview.search != "preview" || len(preview.results) != 1 {
		t.Fatalf("preview update %+v", preview)
	}
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	results, err := session.Query(ctx, "later", "kw later", "*")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "placeholder" {
		t.Fatalf("placeholder %+v", results)
	}
	updated := bridge.waitResults(t, "updated")
	if updated.search != "later" || len(updated.results) != 1 || updated.results[0].Score != 3 {
		t.Fatalf("later update %+v", updated)
	}
}

// publishedHostDir is the loader make host copies into resource/hosts/flow.
// A local publish under hostapp/out still works before that copy.
func publishedHostDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test file path was not found")
	}
	base := filepath.Dir(file)
	candidates := []string{
		filepath.Join(base, "..", "..", "..", "..", "resource", "hosts", "flow"),
		filepath.Join(base, "hostapp", "out"),
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, hostDLLName)); err == nil {
			return dir
		}
	}
	t.Skip("published dotnet host was not found")
	return ""
}

func dotnetSDKPath() string {
	var candidates []string
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		candidates = append(candidates, filepath.Join(local, "wox-dotnet-sdk", "dotnet.exe"))
	}
	if path, err := exec.LookPath("dotnet"); err == nil {
		candidates = append(candidates, path)
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate) == "" {
			continue
		}
		out, err := exec.Command(candidate, "--list-sdks").Output()
		if err == nil && strings.TrimSpace(string(out)) != "" {
			return candidate
		}
	}
	return ""
}

func buildDotNetFixture(sdk, pluginDir string) error {
	projectDir, err := os.MkdirTemp("", "wox-flow-fixture-src")
	if err != nil {
		return err
	}
	defer os.RemoveAll(projectDir)
	if err := os.WriteFile(filepath.Join(projectDir, "Fixture.csproj"), []byte(fixtureProject), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(projectDir, "FixturePlugin.cs"), []byte(fixtureSource), 0o644); err != nil {
		return err
	}
	cmd := exec.Command(sdk, "build", "Fixture.csproj", "-c", "Release", "-o", pluginDir, "--nologo")
	cmd.Dir = projectDir
	cmd.Env = append(os.Environ(), "DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return &fixtureBuildError{err: err, output: string(out)}
	}
	langDir := filepath.Join(pluginDir, "Languages")
	if err := os.MkdirAll(langDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(langDir, "en.xaml"), []byte(fixtureEnglish), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(langDir, "zh-cn.xaml"), []byte(fixtureChinese), 0o644); err != nil {
		return err
	}
	return nil
}

type fixtureBuildError struct {
	err    error
	output string
}

func (e *fixtureBuildError) Error() string {
	return e.err.Error() + "\n" + e.output
}

const fixtureProject = `<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net9.0-windows10.0.19041.0</TargetFramework>
    <UseWPF>true</UseWPF>
    <ImplicitUsings>enable</ImplicitUsings>
    <IncludeWindowsSDKRefFrameworkReferences>false</IncludeWindowsSDKRefFrameworkReferences>
    <Nullable>disable</Nullable>
    <EnableDynamicLoading>true</EnableDynamicLoading>
    <AssemblyName>Fixture</AssemblyName>
  </PropertyGroup>
  <ItemGroup>
    <PackageReference Include="Flow.Launcher.Plugin" Version="5.3.2" />
  </ItemGroup>
</Project>
`

const fixtureSource = `using System.Collections.Generic;
using Flow.Launcher.Plugin;

namespace Fixture;

public class FixturePlugin : IPlugin, IContextMenu, IResultUpdated
{
    IPublicAPI api;
    public event ResultUpdatedEventHandler ResultsUpdated;

    public void Init(PluginInitContext context)
    {
        api = context.API;
    }

    public List<Result> Query(Query query)
    {
        if (query.Search == "preview")
        {
            ResultsUpdated?.Invoke(this, new ResultUpdatedEventArgs
            {
                Query = query,
                Results = new List<Result> { Row("partial", 1) }
            });
            System.Threading.Thread.Sleep(800);
            return new List<Result> { Row("final", 9) };
        }
        if (query.Search == "later")
        {
            var captured = query;
            System.Threading.Tasks.Task.Run(async () =>
            {
                await System.Threading.Tasks.Task.Delay(150);
                ResultsUpdated?.Invoke(this, new ResultUpdatedEventArgs
                {
                    Query = captured,
                    Results = new List<Result> { Row("updated", 3) }
                });
            });
            return new List<Result> { Row("placeholder", 1) };
        }
        return new List<Result>
        {
            new Result
            {
                Title = "hello " + (query.Search ?? ""),
                SubTitle = "row",
                Score = 7,
                CopyText = "copied",
                IcoPath = "icon.png",
                Action = _ =>
                {
                    api.ChangeQuery("next", false);
                    return true;
                }
            }
        };
    }

    static Result Row(string title, int score)
    {
        return new Result { Title = title, SubTitle = "row", Score = score, IcoPath = "icon.png" };
    }

    public List<Result> LoadContextMenus(Result selected)
    {
        return new List<Result>
        {
            new Result
            {
                Title = api.GetTranslation("fixture_extra"),
                Action = _ => false
            }
        };
    }
}
`

const fixtureEnglish = `<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" xmlns:system="clr-namespace:System;assembly=mscorlib">
  <system:String x:Key="fixture_extra">Extra</system:String>
</ResourceDictionary>
`

const fixtureChinese = `<ResourceDictionary xmlns="http://schemas.microsoft.com/winfx/2006/xaml/presentation" xmlns:x="http://schemas.microsoft.com/winfx/2006/xaml" xmlns:system="clr-namespace:System;assembly=mscorlib">
  <system:String x:Key="fixture_extra">更多</system:String>
</ResourceDictionary>
`
