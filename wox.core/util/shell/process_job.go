package shell

import (
	"context"
	"os/exec"
)

// PrepareLifetimeBoundCmd marks a command whose whole process tree must not outlive Wox. It has
// to run before the command is started, and AdoptLifetimeBoundCmd has to run once it has started.
//
// The two phases exist because a process tree can only be captured before the child executes any
// code. On Windows the child is therefore created suspended, so a helper it spawns immediately
// cannot slip out of the job that binds the tree to Wox.
func PrepareLifetimeBoundCmd(cmd *exec.Cmd) {
	prepareLifetimeBoundCmd(cmd)
}

// AdoptLifetimeBoundCmd finishes what PrepareLifetimeBoundCmd started: it binds the freshly
// started process tree to Wox's lifetime and lets the child run.
//
// A non-nil error means the child is not usable and has been terminated, so the caller must treat
// the launch as failed. Failing only to bind the tree is reported through the log instead, because
// losing the cleanup guarantee is not a reason to take the feature away from the user.
func AdoptLifetimeBoundCmd(ctx context.Context, cmd *exec.Cmd) error {
	return adoptLifetimeBoundCmd(ctx, cmd)
}
