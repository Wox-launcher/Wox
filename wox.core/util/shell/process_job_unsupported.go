//go:build !windows

package shell

import (
	"context"
	"os/exec"
)

// macOS and Linux have no equivalent of a kill-on-close job object. Binding a tree there means
// signalling a process group on shutdown, which is a different mechanism with a different failure
// mode, so it is deliberately left out rather than half implemented here.
func prepareLifetimeBoundCmd(cmd *exec.Cmd) {}

func adoptLifetimeBoundCmd(ctx context.Context, cmd *exec.Cmd) error { return nil }

func closeLifetimeBoundJob() {}
