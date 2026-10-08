package dotnet

import "testing"

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
