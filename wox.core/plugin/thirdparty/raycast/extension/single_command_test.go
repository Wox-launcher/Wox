package extension

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"wox/common"
	"wox/plugin"
	"wox/util"
)

// TestSingleCommandSearch checks implicit routing, literal command-name searches, clearing and cleanup.
func TestSingleCommandSearch(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {List}=require('@raycast/api');const h=R.createElement;
	exports.default=()=>{const [text,setText]=R.useState('');return h(List,{filtering:false,onSearchTextChange:setText},h(List.Item,{id:'search',title:'Search: '+text}));};`)
	query := plugin.Query{Id: "single", SessionId: "single-window", TriggerKeyword: "custom"}
	ctx := queryContext(query.SessionId, query.Id)
	response := p.Query(ctx, query)
	if len(response.Results) != 1 || response.Results[0].Title != "Search: " {
		t.Fatalf("bare trigger did not open the view: %+v", response)
	}
	session := p.sessions[query.SessionId]
	for _, search := range []string{"list", "list multiple words", ""} {
		query.Search = search
		response = p.Query(ctx, query)
		if len(response.Results) != 1 || response.Results[0].Title != "Search: "+search || p.sessions[query.SessionId] != session {
			t.Fatalf("search %q lost text or restarted the view: %+v", search, response)
		}
	}
	if _, err := p.invokeAPI(ctx, query, "clearSearch", json.RawMessage(`{}`)); err != nil {
		t.Fatal(err)
	}
	if cleared := <-api.changed; cleared.QueryText != "custom " || !cleared.QueryScope.IsEmpty() {
		t.Fatalf("clearSearch reintroduced the command: %+v", cleared)
	}
	api.leave(ctx)
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		t.Fatal("leaving the single-command plugin retained its process")
	}
}

// TestLocalKillProcessSingleCommand runs the installed bundle read-only, without invoking any kill action.
func TestLocalKillProcessSingleCommand(t *testing.T) {
	source := os.Getenv("WOX_RAYCAST_VERIFY_KILL_PROCESS")
	if source == "" {
		t.Skip("set WOX_RAYCAST_VERIFY_KILL_PROCESS to the local Kill Process extension directory")
	}
	// Native icon loading uses process-wide paths and logging; isolate its temporary cache.
	const dataEnv = "WOX_RAYCAST_PROCESS_TEST_DATA"
	if os.Getenv(dataEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLocalKillProcessSingleCommand$", "-test.v", "-test.timeout=60s")
		cmd.Env = append(os.Environ(), dataEnv+"="+t.TempDir())
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated Kill Process test: %v\n%s", err, output)
		}
		t.Log(string(output))
		return
	}
	data := os.Getenv(dataEnv)
	t.Setenv(util.TestWoxDataDirEnv, data)
	t.Setenv(util.TestUserDataDirEnv, filepath.Join(data, "user"))
	if err := util.GetLocation().Init(); err != nil {
		t.Fatal(err)
	}
	p, api := testPlugin(t, "exports.default=()=>null")
	dir := t.TempDir()
	// Stop the process before TempDir cleanup on Windows.
	t.Cleanup(p.closeAll)
	if err := copyExtension(context.Background(), source, dir); err != nil {
		t.Fatal(err)
	}
	d, err := Parse(dir)
	if err != nil {
		t.Fatal(err)
	}
	if d.Manifest.Name != "kill-process" || len(d.Commands) != 1 || d.Commands[0].Name != "index" || len(d.Metadata.Commands) != 0 {
		t.Fatalf("unexpected Kill Process commands: %+v", d.Commands)
	}
	p.descriptor = d
	api.settings["preference:refreshDuration"] = "0"
	ctx, cancel := context.WithTimeout(queryContext("kill-window", "kill-query"), 25*time.Second)
	defer cancel()
	query := plugin.Query{Id: "kill-query", SessionId: "kill-window", TriggerKeyword: "kill-process"}
	// Enumeration is asynchronous; the real view first renders an empty/loading list.
	waitFor := func(search string, wantRows bool) plugin.QueryResponse {
		t.Helper()
		query.Search = search
		for {
			response := p.Query(ctx, query)
			var rows []plugin.QueryResult
			for _, row := range response.Results {
				if row.Title == "i18n:plugin_raycast_failed" {
					t.Fatalf("Kill Process runtime: %s", row.SubTitle)
				}
				if !strings.HasSuffix(row.Id, ":empty") {
					rows = append(rows, row)
				}
			}
			response.Results = rows
			if (len(response.Results) > 0) == wantRows {
				return response
			}
			select {
			case <-ctx.Done():
				t.Fatalf("timed out waiting for search %q", search)
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	initial := waitFor("", true)
	t.Run("CPU and memory accessories", func(t *testing.T) {
		tails := initial.Results[0].Tails
		if len(tails) != 4 {
			t.Fatalf("expected two icon/value pairs, got %d tails", len(tails))
		}
		for index, asset := range []string{"cpu.svg", "memorychip.svg"} {
			icon, value := tails[index*2], tails[index*2+1]
			if icon.Type != plugin.QueryResultTailTypeImage || icon.Image.ImageType != common.WoxImageTypeAbsolutePath ||
				icon.Image.ImageData != filepath.Join(dir, "assets", asset) {
				t.Fatalf("%s accessory image lost: %+v", asset, icon)
			}
			if value.Type != plugin.QueryResultTailTypeText || value.Text == "" || value.Tooltip == "" || icon.Tooltip != value.Tooltip {
				t.Fatalf("%s accessory value or tooltip lost: %+v", asset, value)
			}
			loaded := common.ConvertIcon(ctx, icon.Image, dir)
			pixels, err := loaded.ToImageWithContext(ctx)
			if err != nil || pixels == nil || pixels.Bounds().Empty() {
				t.Fatalf("%s accessory could not be loaded by Wox: %v", asset, err)
			}
			t.Logf("%s: image loaded (%dx%d), value=%s, tooltip retained", asset, pixels.Bounds().Dx(), pixels.Bounds().Dy(), value.Text)
		}
	})
	if runtime.GOOS == "windows" {
		t.Run("Chrome and ordinary process icons", func(t *testing.T) {
			var chrome, ordinary *plugin.QueryResult
			for i := range initial.Results {
				row := &initial.Results[i]
				if strings.EqualFold(row.Title, "chrome") {
					chrome = row
				}
				if row.Icon.ImageType == common.WoxImageTypeEmoji && row.Icon.ImageData == "🖥️" {
					ordinary = row
				}
			}
			if ordinary == nil {
				t.Fatal("ordinary processes did not return the desktop emoji")
			}
			// Use the same loader as Wox, including its existing emoji cache/download path.
			emoji, err := ordinary.Icon.ToImageWithContext(ctx)
			if err != nil || emoji == nil || emoji.Bounds().Empty() {
				t.Fatalf("ordinary process emoji did not load: %v", err)
			}
			t.Logf("ordinary process %s: emoji loaded (%dx%d)", ordinary.Title, emoji.Bounds().Dx(), emoji.Bounds().Dy())
			if chrome == nil {
				t.Skip("start Chrome to verify its executable icon")
			}
			if chrome.Icon.ImageType != common.WoxImageTypeFileIcon || !strings.EqualFold(filepath.Base(chrome.Icon.ImageData), "chrome.exe") {
				t.Fatalf("Chrome fileIcon was lost: %+v", chrome.Icon)
			}
			if _, err := os.Stat(chrome.Icon.ImageData); err != nil {
				t.Fatal(err)
			}
			icon := common.ConvertIcon(ctx, chrome.Icon, dir)
			pixels, err := icon.ToImageWithContext(ctx)
			if err != nil || pixels == nil || pixels.Bounds().Empty() {
				t.Fatalf("Chrome executable icon did not load: %v", err)
			}
			t.Logf("Chrome: executable icon loaded through Wox (%dx%d)", pixels.Bounds().Dx(), pixels.Bounds().Dy())
		})
	}
	session := p.sessions[query.SessionId]
	if session == nil || session.command.Name != "index" {
		t.Fatal("bare trigger did not launch index")
	}
	search := initial.Results[0].Title
	filtered := waitFor(search, true)
	found := false
	for _, row := range filtered.Results {
		found = found || strings.EqualFold(row.Title, search)
	}
	if !found {
		t.Fatalf("real process name search did not retain %q", search)
	}
	waitFor("wox-nonexistent-process-7b521d09", false)
	cleared := waitFor("", true)
	if p.sessions[query.SessionId] != session {
		t.Fatal("search changes restarted Kill Process")
	}
	api.leave(ctx)
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		t.Fatal("leaving Kill Process retained its process")
	}
	t.Logf("verified actual Kill Process: bare trigger returned %d rows, name search, empty matches and clearing (%d rows); no kill actions invoked", len(initial.Results), len(cleared.Results))
}
