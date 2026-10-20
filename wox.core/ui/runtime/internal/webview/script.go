package webview

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync/atomic"
	"time"
	"wox/util"
)

// ScriptDriver is the optional native capability; lifecycle policy stays in Controller.
type ScriptDriver interface {
	Evaluate(script string, done func(string, error)) error
}

type scriptState struct {
	source       Content
	token        string
	generation   uint64
	queue        []string
	busy         bool
	blocked      bool
	documentSeen bool
}

var documentSequence atomic.Uint64

// invalidateScripts drops queued commands whenever their visible target changes.
func (c *Controller) invalidateScripts() {
	c.scripts.generation++
	c.scripts.queue = nil
	c.scripts.busy = false
	c.scripts.blocked = false
	c.scripts.documentSeen = false
	c.scripts.source = Content{}
	c.scripts.token = ""
}

// prepareDocument gives each inline document an identity independent of its HTML text.
func (c *Controller) prepareDocument(content Content) Content {
	source := content
	source.CornerRadius = 0
	if source != c.scripts.source {
		c.invalidateScripts()
		c.scripts.source = source
		c.scripts.token = fmt.Sprintf("wox-%d", documentSequence.Add(1))
	}
	if content.HTML != "" && content.DocumentKey != "" {
		token, _ := json.Marshal(c.scripts.token)
		marker := "<script>window.__woxPreviewDocument=" + string(token) + ";</script>"
		// Preserve standards mode when an HTML document starts with a doctype.
		end := 0
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(content.HTML)), "<!doctype") {
			end = strings.Index(content.HTML, ">") + 1
		}
		content.HTML = content.HTML[:end] + marker + content.HTML[end:]
	}
	return content
}

// UpdateScript accepts only the currently visible inline document. Success is acceptance,
// not a JavaScript return value; no command is retained as renderable content.
func (c *Controller) UpdateScript(key, script string) bool {
	accepted := false
	_ = c.call(func() error {
		if _, ok := c.driver.(ScriptDriver); !ok || c.scripts.blocked || !c.visible.Load() || c.scripts.source.HTML == "" || c.scripts.source.URL != "" || key == "" || key != c.scripts.source.DocumentKey {
			return nil
		}
		c.scripts.queue = append(c.scripts.queue, script)
		accepted = true
		c.pumpScripts()
		return nil
	})
	return accepted
}

// pumpScripts serializes native evaluations and waits for the matching document to load.
func (c *Controller) pumpScripts() {
	if c.scripts.busy || len(c.scripts.queue) == 0 {
		return
	}
	c.scripts.busy = true
	c.evaluateHead(c.scripts.generation)
}

func (c *Controller) evaluateHead(generation uint64) {
	if generation != c.scripts.generation || len(c.scripts.queue) == 0 {
		return
	}
	source, _ := json.Marshal(c.scripts.queue[0])
	token, _ := json.Marshal(c.scripts.token)
	// Ignore promises and results. Exceptions stay diagnostic and never replace the page.
	mismatch := "pending"
	if c.scripts.documentSeen {
		mismatch = "stale"
	}
	script := "(()=>{if(window.__woxPreviewDocument!==" + string(token) + ")return '" + mismatch + "';if(document.readyState==='loading')return 'pending';try{(0,eval)(" + string(source) + ");return 'ok'}catch(e){return 'error:'+String(e)}})()"
	done := func(status string, err error) {
		_ = c.call(func() error {
			if generation != c.scripts.generation {
				return nil
			}
			if err == nil && status == "stale" {
				// An inline page navigated away. Do not inject into its destination or
				// replay pending work when the browser later returns in history.
				c.scripts.queue = nil
				c.scripts.busy = false
				c.scripts.blocked = true
				return nil
			}
			if err == nil && status == "pending" {
				time.AfterFunc(50*time.Millisecond, func() { _ = c.call(func() error { c.evaluateHead(generation); return nil }) })
				return nil
			}
			if status == "ok" || strings.HasPrefix(status, "error:") {
				c.scripts.documentSeen = true
			}
			if err != nil {
				util.GetLogger().Error(context.Background(), "WebView script update: "+err.Error())
			} else if status != "ok" {
				util.GetLogger().Error(context.Background(), "WebView script update: "+status)
			}
			c.scripts.queue = c.scripts.queue[1:]
			c.scripts.busy = false
			c.pumpScripts()
			return nil
		})
	}
	if err := c.driver.(ScriptDriver).Evaluate(script, done); err != nil {
		done("", err)
	}
}

// RetainScriptTarget invalidates pending work immediately on a selection/document change,
// before the next layout frame reaches Show.
func (c *Controller) RetainScriptTarget(key, html string) {
	_ = c.call(func() error {
		if c.scripts.source.DocumentKey != key || c.scripts.source.HTML != html {
			c.invalidateScripts()
		}
		return nil
	})
}
