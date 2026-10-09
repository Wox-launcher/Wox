package dotnet

import (
	"bufio"
	"encoding/json"
	"io"
	"testing"
	"time"
)

func (b *recordingBridge) resultUpdates() []recordedResults {
	b.mu.Lock()
	defer b.mu.Unlock()
	return append([]recordedResults(nil), b.updates...)
}

func TestResultUpdatePreviewIsDroppedAfterTheQueryReturns(t *testing.T) {
	session := newDotNetSession(dotNetLaunch{Name: "fixture"})
	generation := session.beginQuery()
	message := dotnetMessage{Event: "results", Results: []dotnetResult{{Title: "partial"}}}
	session.noteResultUpdate(&message)
	if !message.Preview || message.Generation != generation {
		t.Fatalf("preview stamp = %+v, generation %d", message, generation)
	}
	if !session.shouldApplyResultUpdate(message) {
		t.Fatal("preview must apply while its query is still running")
	}
	session.endQuery(generation)
	if session.shouldApplyResultUpdate(message) {
		t.Fatal("preview must not replace the query response")
	}

	later := dotnetMessage{Event: "results", Results: []dotnetResult{{Title: "updated"}}}
	session.noteResultUpdate(&later)
	if later.Preview || !session.shouldApplyResultUpdate(later) {
		t.Fatalf("post-query update = %+v", later)
	}
}

func TestResultUpdatePreviewDoesNotApplyToTheNextQuery(t *testing.T) {
	session := newDotNetSession(dotNetLaunch{Name: "fixture"})
	first := session.beginQuery()
	message := dotnetMessage{Event: "results"}
	session.noteResultUpdate(&message)
	second := session.beginQuery()
	if second == first {
		t.Fatal("each query needs its own generation")
	}
	if session.shouldApplyResultUpdate(message) {
		t.Fatal("a preview from the previous query must be dropped")
	}
	session.endQuery(first)
	if !session.queryInFlight {
		t.Fatal("ending the older query must leave the newer query in flight")
	}
	session.endQuery(second)
	if session.queryInFlight {
		t.Fatal("ending the current query must clear the in-flight mark")
	}
}

func TestFastQueryDropsTheHeldResultPreview(t *testing.T) {
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{Name: "fixture", Bridge: bridge})
	generation := session.beginQuery()
	message := dotnetMessage{Event: "results", Query: "hello", Results: []dotnetResult{{Title: "hello"}}}
	session.noteResultUpdate(&message)
	session.handleEvent(message)
	if updates := bridge.resultUpdates(); len(updates) != 0 {
		t.Fatalf("preview painted before the hold: %+v", updates)
	}
	session.endQuery(generation)
	session.releaseHeldPreview()
	if updates := bridge.resultUpdates(); len(updates) != 0 {
		t.Fatalf("fast query painted the one-row preview: %+v", updates)
	}
}

func TestSlowQueryPaintsTheLatestHeldPreview(t *testing.T) {
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{Name: "fixture", Bridge: bridge})
	generation := session.beginQuery()
	defer session.endQuery(generation)
	first := dotnetMessage{Event: "results", Query: "hello", Results: []dotnetResult{{Title: "hello"}}}
	session.noteResultUpdate(&first)
	session.handleEvent(first)
	second := dotnetMessage{Event: "results", Query: "hello", Results: []dotnetResult{{Title: "hello"}, {Title: "hell"}}}
	session.noteResultUpdate(&second)
	session.handleEvent(second)
	session.releaseHeldPreview()
	updates := bridge.resultUpdates()
	if len(updates) != 1 || len(updates[0].results) != 2 || updates[0].results[1].Title != "hell" {
		t.Fatalf("held preview = %+v", updates)
	}
}

func TestHeldPreviewPaintsAfterTheHoldWhileTheQueryRuns(t *testing.T) {
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{Name: "fixture", Bridge: bridge})
	generation := session.beginQuery()
	defer session.endQuery(generation)
	message := dotnetMessage{Event: "results", Query: "hello", Results: []dotnetResult{{Title: "hello"}}}
	session.noteResultUpdate(&message)
	session.handleEvent(message)
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if len(bridge.resultUpdates()) == 1 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("preview was not painted after the hold")
}

func TestHideReportsTheLauncherHiddenAfterItCloses(t *testing.T) {
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{Name: "fixture", Bridge: bridge})
	reader, writer := io.Pipe()
	session.stdin = writer
	t.Cleanup(func() {
		_ = writer.Close()
		_ = reader.Close()
	})
	done := make(chan string, 1)
	go func() {
		line, err := bufio.NewReader(reader).ReadString('\n')
		if err != nil {
			done <- ""
			return
		}
		done <- line
	}()
	session.handleEvent(dotnetMessage{Event: "hide", Epoch: 4})
	select {
	case line := <-done:
		var payload map[string]any
		if err := json.Unmarshal([]byte(line), &payload); err != nil {
			t.Fatal(err)
		}
		epoch, _ := payload["epoch"].(float64)
		if payload["method"] != "visibility" || payload["visible"] != false || epoch != 4 {
			t.Fatalf("visibility report = %s", line)
		}
	case <-time.After(time.Second):
		t.Fatal("visibility was not reported")
	}
	bridge.mu.Lock()
	hidden := bridge.hide
	bridge.mu.Unlock()
	if !hidden {
		t.Fatal("hide must close the launcher before the plugin is told")
	}
}

func TestResultUpdateAfterTheQueryStaysImmediate(t *testing.T) {
	bridge := &recordingBridge{}
	session := newDotNetSession(dotNetLaunch{Name: "fixture", Bridge: bridge})
	message := dotnetMessage{Event: "results", Query: "hello", Results: []dotnetResult{{Title: "hello"}}}
	session.noteResultUpdate(&message)
	if message.Preview {
		t.Fatal("a result update with no query in flight must not be held")
	}
	session.handleEvent(message)
	if updates := bridge.resultUpdates(); len(updates) != 1 {
		t.Fatalf("post-query update = %+v", updates)
	}
}
