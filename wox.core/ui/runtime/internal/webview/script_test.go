package webview

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"
)

type scriptDriver struct {
	fakeDriver
	scripts   []string
	callbacks []func(string, error)
}

func (d *scriptDriver) Evaluate(script string, done func(string, error)) error {
	d.scripts = append(d.scripts, script)
	d.callbacks = append(d.callbacks, done)
	return nil
}
func TestScriptQueueLoadingOrderingAndRedraw(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		d := &scriptDriver{}
		c := New(d, immediateDispatch)
		content := Content{DocumentKey: "plugin/query/result", HTML: "<!doctype html><p>Hello</p>"}
		bounds := Rect{Width: 100, Height: 100}
		if err := c.Show(content, bounds, 1); err != nil {
			t.Fatal(err)
		}
		if !c.UpdateScript(content.DocumentKey, "first()") || !c.UpdateScript(content.DocumentKey, "second()") {
			t.Fatal("not accepted")
		}
		if len(d.scripts) != 1 {
			t.Fatal("commands ran concurrently")
		}
		d.callbacks[0]("pending", nil)
		time.Sleep(51 * time.Millisecond)
		if len(d.scripts) != 2 || d.scripts[0] != d.scripts[1] {
			t.Fatal("loading retry changed command")
		}
		d.callbacks[1]("ok", nil)
		if len(d.scripts) != 3 || !strings.Contains(d.scripts[2], "second()") {
			t.Fatal("FIFO violated")
		}
		d.callbacks[2]("ok", nil)
		before := d.shown[0].HTML
		_ = c.Show(content, bounds, 1)
		if d.shown[1].HTML != before || len(d.scripts) != 3 {
			t.Fatal("redraw replaced/replayed document")
		}
		if !strings.HasPrefix(before, "<!doctype html>") {
			t.Fatal("doctype lost")
		}
		c.Close()
	})
}
func TestScriptTargetsAndInvalidation(t *testing.T) {
	for _, change := range []string{"hide", "reset", "close", "replace", "select", "query"} {
		t.Run(change, func(t *testing.T) {
			d := &scriptDriver{}
			c := New(d, immediateDispatch)
			content := Content{DocumentKey: "query1/row", HTML: "<p>one</p>"}
			bounds := Rect{Width: 100, Height: 100}
			_ = c.Show(content, bounds, 1)
			if c.UpdateScript("other-plugin/row", "wrong()") {
				t.Fatal("wrong target accepted")
			}
			c.UpdateScript(content.DocumentKey, "first()")
			c.UpdateScript(content.DocumentKey, "queued()")
			switch change {
			case "hide":
				_ = c.Hide()
			case "reset":
				_ = c.Reset()
			case "close":
				c.Close()
			case "replace":
				content.HTML = "<p>two</p>"
				_ = c.Show(content, bounds, 1)
			case "select":
				c.RetainScriptTarget("query1/other", content.HTML)
			case "query":
				c.RetainScriptTarget("", "")
			}
			d.callbacks[0]("ok", nil)
			if len(d.scripts) != 1 {
				t.Fatal("stale queue executed")
			}
		})
	}
	d := &scriptDriver{}
	c := New(d, immediateDispatch)
	_ = c.Show(Content{DocumentKey: "url", URL: "https://example.com"}, Rect{Width: 100, Height: 100}, 1)
	if c.UpdateScript("url", "no()") {
		t.Fatal("URL accepted script")
	}
}
func TestScriptFailureDoesNotReplaceDocument(t *testing.T) {
	d := &scriptDriver{}
	c := New(d, immediateDispatch)
	content := Content{DocumentKey: "row", HTML: "<p>original</p>"}
	_ = c.Show(content, Rect{Width: 100, Height: 100}, 1)
	c.UpdateScript("row", "throw Error('test')")
	c.UpdateScript("row", "after()")
	d.callbacks[0]("error:test", nil)
	if len(d.scripts) != 2 {
		t.Fatal("JS error blocked queue")
	}
	d.callbacks[1]("", errors.New("native error"))
	if len(d.shown) != 1 || !c.Visible() || c.scripts.source.HTML != content.HTML {
		t.Fatal("error replaced document")
	}
}

func TestScriptQueueDropsWhenInlinePageNavigatesAway(t *testing.T) {
	d := &scriptDriver{}
	c := New(d, immediateDispatch)
	_ = c.Show(Content{DocumentKey: "row", HTML: "<p>original</p>"}, Rect{Width: 100, Height: 100}, 1)
	c.UpdateScript("row", "first()")
	c.UpdateScript("row", "queued()")
	d.callbacks[0]("ok", nil)
	d.callbacks[1]("stale", nil)
	if len(d.scripts) != 2 || c.UpdateScript("row", "later()") {
		t.Fatal("navigated document accepted work")
	}
}
