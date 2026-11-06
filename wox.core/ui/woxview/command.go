package woxview

import (
	"errors"
	"fmt"

	"wox/ui/widget"
)

// Command names a local operation that cannot be expressed as tree state.
type Command string

const (
	CommandFocus     Command = "focus"
	CommandScrollTo  Command = "scrollTo"
	CommandSelectAll Command = "selectAll"
)

type lazyCommandAncestor struct {
	id         string
	start, end float32
}

type commandTarget struct {
	id   string
	lazy []lazyCommandAncestor
}

// pendingCommand keeps the original identity so key reuse under another parent
// cannot redirect an accepted operation after a tree update.
type pendingCommand struct {
	kind Command
	key  string
	id   string
	host *widget.Host
	done func(error)
}

// findCommandTarget resolves producer keys against the complete tree, including
// virtualized rows. Keys need only be sibling-unique for rendering; commands
// require an unambiguous document-wide key.
func (v *View) findCommandTarget(key string) (commandTarget, error) {
	if key == "" {
		return commandTarget{}, errors.New("command key is required")
	}
	var target commandTarget
	matches := 0
	var walk func(*Node, string, []lazyCommandAncestor)
	walk = func(n *Node, id string, lazy []lazyCommandAncestor) {
		if n.Key == key {
			matches++
			target = commandTarget{id: id, lazy: append([]lazyCommandAncestor(nil), lazy...)}
		}
		for i, child := range n.Children {
			ancestors := lazy
			if p, ok := n.Props.(*LazyListProps); ok {
				ancestors = append(append([]lazyCommandAncestor(nil), lazy...), lazyCommandAncestor{id: id, start: float32(i) * p.ItemHeight.resolve(v.theme), end: float32(i+1) * p.ItemHeight.resolve(v.theme)})
			}
			walk(child, nodeID(id, child, i), ancestors)
		}
	}
	if v.tree != nil && v.tree.Root != nil {
		walk(v.tree.Root, nodeID(v.prefix, v.tree.Root, 0), nil)
	}
	if matches == 0 {
		return commandTarget{}, fmt.Errorf("command key %q was not found", key)
	}
	if matches > 1 {
		return commandTarget{}, fmt.Errorf("command key %q is ambiguous", key)
	}
	return target, nil
}

// EnqueueCommand validates a request on the UI thread. The embedder must request
// a frame; done reports execution failures after layout, never from Build.
// Operations are FIFO and bounded, including when the window is hidden.
func (v *View) EnqueueCommand(host *widget.Host, command Command, key string, done func(error)) error {
	switch command {
	case CommandFocus, CommandScrollTo, CommandSelectAll:
	default:
		return fmt.Errorf("unknown content command %q", command)
	}
	if host == nil {
		return errors.New("content command host is unavailable")
	}
	target, err := v.findCommandTarget(key)
	if err != nil {
		return err
	}
	if len(v.commands) >= 64 {
		return errors.New("window content command queue full")
	}
	v.commands = append(v.commands, &pendingCommand{kind: command, key: key, id: target.id, host: host, done: done})
	// A frame alone may reuse an enclosing Boundary. Dirty the retained owner
	// so its post-layout work also runs when the tree props have not changed.
	v.frameContext.SetState(nil)
	return nil
}

// advanceCommand materializes lazy ancestors from outside in, then operates on
// actual logical bounds. Each scroll change needs a fresh layout, not a guessed
// physical offset. Re-resolving also follows row reordering before execution.
func (v *View) advanceCommand(c *pendingCommand) (bool, error) {
	target, err := v.findCommandTarget(c.key)
	if err != nil {
		return true, err
	}
	if target.id != c.id {
		return true, errors.New("command target replaced")
	}
	if c.host.HasModalScope() {
		return true, errors.New("content command blocked by active modal scope")
	}
	for _, ancestor := range target.lazy {
		controller := v.scrolls[ancestor.id]
		if controller == nil {
			return true, errors.New("command scroll ancestor is not laid out")
		}
		if controller.EnsureVisible(ancestor.start, ancestor.end) {
			return false, nil
		}
	}
	key := widget.Key(target.id + "/target")
	switch c.kind {
	case CommandFocus:
		return true, c.host.FocusWithin(key)
	case CommandSelectAll:
		return true, c.host.SelectAllWithin(key)
	default:
		moved, err := c.host.Reveal(key)
		return !moved, err
	}
}

// flushCommands runs one operation per frame so callbacks cannot make later
// commands observe stale layout. A document's View owns its queue and lifetime.
func (v *View) flushCommands(ctx widget.StateContext) {
	if !ctx.Mounted() || len(v.commands) == 0 {
		return
	}
	c := v.commands[0]
	finished, err := v.advanceCommand(c)
	// A synchronous focus callback may replace the surface and cancel its queue.
	if len(v.commands) == 0 || v.commands[0] != c {
		return
	}
	if finished {
		v.commands[0] = nil
		v.commands = v.commands[1:]
		if c.done != nil {
			c.done(err)
		}
	}
	if len(v.commands) > 0 {
		ctx.SetState(nil)
	}
}
