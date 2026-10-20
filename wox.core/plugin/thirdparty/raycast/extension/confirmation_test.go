package extension

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"wox/plugin"
	"wox/util"
	"wox/util/shell"
)

// awaitConfirmation ignores ordinary async updates and waits for the two local confirmation actions.
func awaitConfirmation(t *testing.T, api *recordingAPI) plugin.UpdatableResult {
	t.Helper()
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	for {
		select {
		case update := <-api.updates:
			if update.Actions != nil && len(*update.Actions) == 2 && (*update.Actions)[0].Id == "raycast:confirm" {
				actions := *update.Actions
				if !actions[0].IsDefault || actions[0].Hotkey != "enter" || actions[1].Hotkey != "ctrl+enter" {
					t.Fatalf("unexpected confirmation hotkeys: %+v", actions)
				}
				return update
			}
		case <-timer.C:
			t.Fatal("confirmation was not shown")
		}
	}
}

// TestConfirmAlertResultActions verifies resolution, query cancellation and stale confirmation callbacks.
func TestConfirmAlertResultActions(t *testing.T) {
	for _, outcome := range []string{"confirm", "cancel", "search", "leave"} {
		t.Run(outcome, func(t *testing.T) {
			p, api := testPlugin(t, `const R=require('react');const {List,Action,ActionPanel,confirmAlert,Clipboard}=require('@raycast/api');const h=R.createElement;
			exports.default=()=>h(List,null,h(List.Item,{id:'item',title:'Original',actions:h(ActionPanel,null,h(Action,{title:'Run',onAction:async()=>{
			const ok=await confirmAlert({title:'Confirm operation?',message:'Only a test',primaryAction:{title:'Proceed',onAction:()=>Clipboard.copy('primary')},
			dismissAction:{title:'Dismiss',onAction:()=>Clipboard.copy('dismiss')}});
			await Clipboard.copy(ok?'accepted':'cancelled')}}))}));`)
			ctx := queryContext("confirm-window", "first")
			response := p.Query(ctx, plugin.Query{Id: "first", SessionId: "confirm-window", TriggerKeyword: "example"})
			done := make(chan struct{})
			go func() { response.Results[0].Actions[0].Action(ctx, plugin.ActionContext{}); close(done) }()
			prompt := awaitConfirmation(t, api)
			if prompt.Id != response.Results[0].Id || prompt.Title == nil || *prompt.Title != "Confirm operation?" ||
				prompt.SubTitle == nil || *prompt.SubTitle != "Only a test" {
				t.Fatalf("confirmation did not replace the invoking row: %+v", prompt)
			}
			actions := *prompt.Actions
			switch outcome {
			case "confirm":
				actions[0].Action(ctx, plugin.ActionContext{})
			case "cancel":
				actions[1].Action(ctx, plugin.ActionContext{})
			case "search":
				next, cancel := context.WithTimeout(queryContext("confirm-window", "next"), 3*time.Second)
				defer cancel()
				p.Query(next, plugin.Query{Id: "next", SessionId: "confirm-window", TriggerKeyword: "example", Search: "changed"})
			case "leave":
				api.leave(ctx)
			}
			select {
			case <-done:
			case <-time.After(3 * time.Second):
				t.Fatal("confirmation did not release the waiting action")
			}
			// A callback retained from the old row must never approve a finished or canceled request.
			actions[0].Action(ctx, plugin.ActionContext{})
			if outcome != "leave" {
				want := []string{"dismiss", "cancelled"}
				if outcome == "confirm" {
					want = []string{"primary", "accepted"}
				}
				for _, text := range want {
					select {
					case got := <-api.copied:
						if got != text {
							t.Fatalf("callback result %q, want %q", got, text)
						}
					case <-time.After(time.Second):
						t.Fatalf("missing callback %q", text)
					}
				}
			}
			for {
				select {
				case text := <-api.copied:
					if text == "accepted" || text == "primary" {
						t.Fatal("stale confirmation was accepted")
					}
				default:
					return
				}
			}
		})
	}
}

// TestLocalKillProcessConfirmation runs the real extension only against a uniquely named owned process.
func TestLocalKillProcessConfirmation(t *testing.T) {
	source := os.Getenv("WOX_RAYCAST_VERIFY_KILL_PROCESS")
	if source == "" || runtime.GOOS != "windows" {
		t.Skip("requires Windows and WOX_RAYCAST_VERIFY_KILL_PROCESS")
	}
	const dataEnv = "WOX_RAYCAST_CONFIRM_TEST_DATA"
	if os.Getenv(dataEnv) == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestLocalKillProcessConfirmation$", "-test.v", "-test.timeout=90s")
		cmd.Env = append(os.Environ(), dataEnv+"="+t.TempDir())
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("isolated confirmation test: %v\n%s", err, output)
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
	for _, outcome := range []string{"confirm", "cancel", "leave"} {
		t.Run(outcome, func(t *testing.T) {
			p, api := testPlugin(t, "exports.default=()=>null")
			dir := t.TempDir()
			t.Cleanup(p.closeAll)
			if err := copyExtension(context.Background(), source, dir); err != nil {
				t.Fatal(err)
			}
			d, err := Parse(dir)
			if err != nil {
				t.Fatal(err)
			}
			p.descriptor = d
			api.settings["preference:refreshDuration"] = "0"
			api.settings["preference:closeWindowAfterKill"] = "false"
			api.settings["preference:clearSearchBarAfterKill"] = "false"
			api.settings["preference:goToRootAfterKill"] = "false"
			api.settings["preference:skipConfirmation"] = "false"
			api.settings["preference:shouldShowPID"] = "true"
			// A private executable name avoids accidentally selecting another user's Node process or a group.
			name := fmt.Sprintf("wox-confirm-%s-%d", outcome, time.Now().UnixNano())
			executable := filepath.Join(t.TempDir(), name+".exe")
			node, err := os.ReadFile(p.node)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(executable, node, 0700); err != nil {
				t.Fatal(err)
			}
			target := exec.Command(executable, "-e", "setInterval(()=>{},1000)")
			shell.HideWindowCmd(target)
			if err = target.Start(); err != nil {
				t.Fatal(err)
			}
			exited := make(chan struct{})
			go func() { _ = target.Wait(); close(exited) }()
			t.Cleanup(func() { _ = target.Process.Kill(); <-exited })
			ctx, cancel := context.WithTimeout(queryContext("kill-confirm", "query"), 25*time.Second)
			defer cancel()
			query := plugin.Query{Id: "query", SessionId: "kill-confirm", TriggerKeyword: "kill-process", Search: name}
			response := p.Query(ctx, query)
			// This bundle derives visible rows in a separate effect after enumeration completes.
			for len(response.Results) == 0 || (len(response.Results) == 1 && strings.HasSuffix(response.Results[0].Id, ":empty")) {
				select {
				case <-exited:
					t.Fatal("test process exited before enumeration")
				case <-ctx.Done():
					t.Fatal("owned test process was not listed")
				case <-time.After(100 * time.Millisecond):
				}
				response = p.Query(ctx, query)
			}
			if len(response.Results) != 1 || response.Results[0].Title != name ||
				response.Results[0].SubTitle != strconv.Itoa(target.Process.Pid) {
				t.Fatalf("refusing to execute: result is not exclusively the owned PID %d: %+v", target.Process.Pid, response.Results)
			}
			var kill *plugin.QueryResultAction
			for i := range response.Results[0].Actions {
				if response.Results[0].Actions[i].Name == "Force Kill" {
					kill = &response.Results[0].Actions[i]
					break
				}
			}
			if kill == nil {
				t.Fatal("real plugin did not expose Force Kill")
			}
			done := make(chan struct{})
			go func() { kill.Action(ctx, plugin.ActionContext{}); close(done) }()
			prompt := awaitConfirmation(t, api)
			select {
			case <-exited:
				t.Fatal("process exited before confirmation")
			default:
			}
			switch outcome {
			case "confirm":
				(*prompt.Actions)[0].Action(ctx, plugin.ActionContext{})
			case "cancel":
				(*prompt.Actions)[1].Action(ctx, plugin.ActionContext{})
			case "leave":
				api.leave(ctx)
			}
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("real action did not finish")
			}
			if outcome == "confirm" {
				select {
				case <-exited:
				case <-time.After(5 * time.Second):
					t.Fatal("confirmed test process was not terminated")
				}
			} else {
				(*prompt.Actions)[0].Action(ctx, plugin.ActionContext{})
				select {
				case <-exited:
					t.Fatal("canceled/exited confirmation terminated the test process")
				default:
				}
			}
			t.Logf("%s: verified owned PID %d; Enter confirms, Ctrl+Enter cancels", outcome, target.Process.Pid)
		})
	}
}
