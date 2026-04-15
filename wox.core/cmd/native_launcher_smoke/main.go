package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"wox/common"
	"wox/launcher"
	"wox/plugin"
	"wox/util/mainthread"
)

type runtimeDebugger interface {
	DebugSnapshot(ctx context.Context) launcher.DebugSnapshot
}

func main() {
	mainthread.Init(run)
}

func run() {
	duration := flag.Duration("duration", 5*time.Second, "how long to keep the smoke window visible before exit")
	width := flag.Int("width", 760, "placeholder shell width")
	resizeWidth := flag.Int("resize-width", 0, "resize the shell to this width before exit")
	resizeAfter := flag.Duration("resize-after", 0, "delay before applying resize-width")
	query := flag.String("query", "", "initial query text")
	updateQuery := flag.String("update-query", "", "query text to apply after update-query-after")
	updateQueryAfter := flag.Duration("update-query-after", 0, "delay before applying update-query")
	refreshAfter := flag.Duration("refresh-after", 0, "delay before replaying the current query state")
	demoResults := flag.Int("demo-results", 4, "number of demo results to render in the shell")
	debug := flag.Bool("debug", false, "print runtime window snapshots during smoke execution")
	x := flag.Int("x", 0, "screen x position for the shell")
	y := flag.Int("y", 0, "screen y position for the shell")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var runtime launcher.Runtime
	runtime, err := launcher.DefaultRuntimeFactoryWithOptions(ctx, launcher.WindowShellRuntimeOptions{
		OnUserQueryChanged: func(queryCtx context.Context, query common.PlainQuery) error {
			pushDemoResults(queryCtx, runtime, query, *demoResults)
			if *debug {
				fmt.Printf("native launcher smoke [user-query] queryId=%s text=%q type=%s\n", query.QueryId, query.QueryText, query.QueryType)
			}
			return nil
		},
		OnSelectedResultAction: func(actionCtx context.Context, queryID string, resultID string, actionID string) error {
			if *debug {
				fmt.Printf("native launcher smoke [submit] queryId=%s resultId=%s actionId=%s\n", queryID, resultID, actionID)
			}
			return nil
		},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create launcher runtime: %v\n", err)
		os.Exit(1)
	}

	if err := runtime.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "failed to start launcher runtime: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		if stopErr := runtime.Stop(context.Background()); stopErr != nil {
			fmt.Fprintf(os.Stderr, "failed to stop launcher runtime: %v\n", stopErr)
		}
	}()

	showCtx := common.ShowContext{
		SelectAll:   true,
		WindowWidth: *width,
	}
	if *x != 0 || *y != 0 {
		showCtx.WindowPosition = &common.WindowPosition{
			X: *x,
			Y: *y,
		}
	}
	if *query != "" {
		runtime.ChangeQuery(ctx, newSmokeQuery(*query))
	}
	runtime.ChangeTheme(ctx, defaultSmokeTheme())
	runtime.Show(ctx, showCtx)
	pushDemoResults(ctx, runtime, currentQuery(runtime, *query), *demoResults)
	fmt.Printf("native launcher smoke shell visible for %s\n", duration.String())
	dumpSnapshot(ctx, runtime, *debug, "show")

	resizeTimer := time.NewTimer(optionalDuration(*resizeAfter, *duration))
	defer resizeTimer.Stop()

	queryTimer := time.NewTimer(optionalDuration(*updateQueryAfter, *duration))
	defer queryTimer.Stop()

	refreshTimer := time.NewTimer(optionalDuration(*refreshAfter, *duration))
	defer refreshTimer.Stop()

	doneTimer := time.NewTimer(*duration)
	defer doneTimer.Stop()

	resizeHandled := *resizeAfter <= 0 || *resizeWidth <= 0
	queryHandled := *updateQueryAfter <= 0 || *updateQuery == ""
	refreshHandled := *refreshAfter <= 0

	for {
		select {
		case <-ctx.Done():
			fmt.Println("native launcher smoke interrupted")
			runtime.Hide(context.Background())
			os.Exit(0)
		case <-resizeTimer.C:
			if !resizeHandled {
				showCtx.WindowWidth = *resizeWidth
				runtime.Show(ctx, showCtx)
				fmt.Printf("native launcher smoke resized to width=%d\n", *resizeWidth)
				dumpSnapshot(ctx, runtime, *debug, "resize")
				resizeHandled = true
			}
		case <-queryTimer.C:
			if !queryHandled {
				runtime.ChangeQuery(ctx, newSmokeQuery(*updateQuery))
				pushDemoResults(ctx, runtime, currentQuery(runtime, *updateQuery), *demoResults)
				fmt.Printf("native launcher smoke updated query=%q\n", *updateQuery)
				dumpSnapshot(ctx, runtime, *debug, "query")
				queryHandled = true
			}
		case <-refreshTimer.C:
			if !refreshHandled {
				runtime.RefreshQuery(ctx, false)
				fmt.Println("native launcher smoke refreshed query state")
				dumpSnapshot(ctx, runtime, *debug, "refresh")
				refreshHandled = true
			}
		case <-doneTimer.C:
			fmt.Println("native launcher smoke completed")
			runtime.Hide(context.Background())
			os.Exit(0)
		}
	}
}

func optionalDuration(value time.Duration, fallback time.Duration) time.Duration {
	if value > 0 {
		return value
	}

	return fallback
}

func dumpSnapshot(ctx context.Context, runtime launcher.Runtime, enabled bool, phase string) {
	if !enabled {
		return
	}

	debugger, ok := runtime.(runtimeDebugger)
	if !ok {
		fmt.Printf("native launcher smoke [%s] debug unavailable\n", phase)
		return
	}

	snapshot := debugger.DebugSnapshot(ctx)
	fmt.Printf(
		"native launcher smoke [%s] host(handle=%#x visible=%t) text(parent=%#x host=%#x edit=%#x hostVisible=%t editVisible=%t focused=%t frame=%+v query=%q results=%d selected=%d)\n",
		phase,
		snapshot.Host.NativeWindowHandle,
		snapshot.Host.Visible,
		snapshot.TextInput.ParentWindowHandle,
		snapshot.TextInput.HostWindowHandle,
		snapshot.TextInput.EditControlHandle,
		snapshot.TextInput.HostVisible,
		snapshot.TextInput.EditVisible,
		snapshot.TextInput.Focused,
		snapshot.TextInput.Frame,
		snapshot.Query.QueryText,
		len(snapshot.Results.Items),
		snapshot.Results.SelectedIndex,
	)
}

func defaultSmokeTheme() common.Theme {
	return common.Theme{
		ThemeId:                              "native-launcher-smoke-dark",
		AppBackgroundColor:                   "rgba(35, 41, 51, 0.75)",
		QueryBoxBackgroundColor:              "rgba(49, 56, 68, 0.3)",
		QueryBoxFontColor:                    "#E2E8F0",
		QueryBoxCursorColor:                  "#00A88E",
		QueryBoxBorderRadius:                 8,
		QueryBoxTextSelectionBackgroundColor: "rgba(0, 168, 142, 0.8)",
		QueryBoxTextSelectionColor:           "#FFFFFF",
	}
}

func currentQuery(runtime launcher.Runtime, fallback string) common.PlainQuery {
	debugger, ok := runtime.(runtimeDebugger)
	if !ok {
		return common.PlainQuery{QueryText: fallback}
	}

	return debugger.DebugSnapshot(context.Background()).Query
}

func pushDemoResults(ctx context.Context, runtime launcher.Runtime, query common.PlainQuery, count int) {
	if runtime == nil || query.QueryId == "" {
		return
	}
	if count < 0 {
		count = 0
	}
	if strings.TrimSpace(query.QueryText) == "" {
		runtime.PushResults(ctx, plugin.PushResultsPayload{
			QueryId: query.QueryId,
			Results: []plugin.QueryResultUI{},
		})
		return
	}
	if count == 0 {
		runtime.PushResults(ctx, plugin.PushResultsPayload{
			QueryId: query.QueryId,
			Results: []plugin.QueryResultUI{},
		})
		return
	}

	results := make([]plugin.QueryResultUI, 0, count)
	for index := 0; index < count; index++ {
		results = append(results, plugin.QueryResultUI{
			QueryId:  query.QueryId,
			Id:       fmt.Sprintf("demo-%d", index),
			Title:    fmt.Sprintf("Result %d for %s", index+1, query.QueryText),
			SubTitle: fmt.Sprintf("Demo subtitle %d", index+1),
			Actions: []plugin.QueryResultActionUI{
				{
					Id:        fmt.Sprintf("demo-action-%d", index),
					Name:      "Open",
					IsDefault: true,
				},
			},
		})
	}

	runtime.PushResults(ctx, plugin.PushResultsPayload{
		QueryId: query.QueryId,
		Results: results,
	})
}

func newSmokeQuery(text string) common.PlainQuery {
	return common.PlainQuery{
		QueryId:   fmt.Sprintf("smoke-%d", time.Now().UnixNano()),
		QueryType: string(plugin.QueryTypeInput),
		QueryText: text,
	}
}
