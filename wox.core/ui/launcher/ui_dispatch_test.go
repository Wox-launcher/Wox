package launcher

import (
	"errors"
	"testing"
)

func TestRunOnUIUsesInjectedDispatcher(t *testing.T) {
	dispatched := false
	ran := false
	app := &App{
		uiCall: func(fn func()) error {
			dispatched = true
			fn()
			return nil
		},
	}

	if err := app.runOnUI("test operation", func() { ran = true }); err != nil {
		t.Fatalf("runOnUI returned error: %v", err)
	}
	if !dispatched || !ran {
		t.Fatalf("dispatcher=%v ran=%v, want both true", dispatched, ran)
	}
}

func TestRunOnUIRunsInlineWithoutDispatcher(t *testing.T) {
	ran := false
	app := &App{}

	if err := app.runOnUI("test operation", func() { ran = true }); err != nil {
		t.Fatalf("runOnUI returned error: %v", err)
	}
	if !ran {
		t.Fatal("inline UI operation did not run")
	}
}

func TestRunOnUIWrapsDispatcherError(t *testing.T) {
	app := &App{uiCall: func(func()) error { return errors.New("runtime stopped") }}

	err := app.runOnUI("apply query response", func() {})
	if err == nil || err.Error() != "apply query response: runtime stopped" {
		t.Fatalf("runOnUI error = %v", err)
	}
}
