package extension

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"wox/plugin"
)

// TestLoadingWaitsForResults keeps the Wox query pending until React has actual results.
func TestLoadingWaitsForResults(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const fs=require('node:fs');const path=require('node:path');
	const {List,Clipboard,environment}=require('@raycast/api');const h=R.createElement;
	exports.default=()=>{const [ready,setReady]=R.useState(false);R.useEffect(()=>{
	const timer=setInterval(()=>{if(fs.existsSync(path.join(environment.supportPath,'ready'))){clearInterval(timer);setReady(true)}},10);
	Clipboard.copy('waiting');return ()=>clearInterval(timer)},[]);
	return h(List,{isLoading:!ready},ready?h(List.Item,{title:'Ready'}):null)};`)
	ctx, cancel := context.WithTimeout(queryContext("loading", "first"), 5*time.Second)
	defer cancel()
	result := make(chan plugin.QueryResponse, 1)
	go func() {
		result <- p.Query(ctx, plugin.Query{Id: "first", SessionId: "loading", TriggerKeyword: "example"})
	}()
	select {
	case <-api.copied:
	case <-ctx.Done():
		t.Fatal("view did not mount")
	}
	select {
	case response := <-result:
		t.Fatalf("loading returned a placeholder instead of waiting: %+v", response)
	case <-api.refresh:
		t.Fatal("loading requested a redundant refresh")
	default:
	}
	if err := os.WriteFile(filepath.Join(api.cache, "ready"), []byte("yes"), 0600); err != nil {
		t.Fatal(err)
	}
	select {
	case response := <-result:
		if len(response.Results) != 1 || response.Results[0].Title != "Ready" {
			t.Fatalf("loaded results: %+v", response)
		}
	case <-ctx.Done():
		t.Fatal("loaded results did not complete the query")
	}
}

// TestLoadingCancellationAllowsNewSearch proves waiting does not block the runtime's request queue.
func TestLoadingCancellationAllowsNewSearch(t *testing.T) {
	p, api := testPlugin(t, `const R=require('react');const {List,Clipboard}=require('@raycast/api');const h=R.createElement;
	exports.default=()=>{const [text,setText]=R.useState('');R.useEffect(()=>{Clipboard.copy('waiting')},[]);
	return h(List,{isLoading:text!=='ready',onSearchTextChange:setText},text==='ready'?h(List.Item,{title:'Ready'}):null)};`)
	ctx, cancel := context.WithCancel(queryContext("loading", "old"))
	defer cancel()
	done := make(chan plugin.QueryResponse, 1)
	go func() {
		done <- p.Query(ctx, plugin.Query{Id: "old", SessionId: "loading", TriggerKeyword: "example"})
	}()
	select {
	case <-api.copied:
	case <-time.After(3 * time.Second):
		t.Fatal("view did not mount")
	}
	cancel()
	select {
	case response := <-done:
		if len(response.Results) != 0 {
			t.Fatalf("canceled loading query returned results: %+v", response)
		}
	case <-time.After(time.Second):
		t.Fatal("loading query ignored cancellation")
	}
	session := p.sessions["loading"]
	next, nextCancel := context.WithTimeout(queryContext("loading", "new"), 3*time.Second)
	defer nextCancel()
	response := p.Query(next, plugin.Query{Id: "new", SessionId: "loading", TriggerKeyword: "example", Search: "ready"})
	if len(response.Results) != 1 || response.Results[0].Title != "Ready" || p.sessions["loading"] != session {
		t.Fatalf("new search blocked or restarted the view: %+v", response)
	}
	api.leave(next)
	select {
	case <-session.done:
	case <-time.After(3 * time.Second):
		t.Fatal("leaving retained the process")
	}
}

// TestBackgroundLoadingKeepsResults allows cached rows to remain usable during refresh.
func TestBackgroundLoadingKeepsResults(t *testing.T) {
	p, _ := testPlugin(t, `const R=require('react');const {List}=require('@raycast/api');
	exports.default=()=>R.createElement(List,{isLoading:true},R.createElement(List.Item,{title:'Cached'}));`)
	ctx, cancel := context.WithTimeout(queryContext("cached", "query"), 3*time.Second)
	defer cancel()
	response := p.Query(ctx, plugin.Query{Id: "query", SessionId: "cached", TriggerKeyword: "example"})
	if len(response.Results) != 1 || response.Results[0].Title != "Cached" {
		t.Fatalf("background loading hid cached results: %+v", response)
	}
}
