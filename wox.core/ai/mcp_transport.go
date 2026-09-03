package ai

import (
	"context"
	"os/exec"
	"sort"
	"wox/util"
	"wox/util/shell"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcpServerProcesses maps a configured server name to the process Wox started for it. Only the
// root is recorded: every helper below it is found by walking parent links, which is what lets a
// launcher chain be attributed to the server the user configured.
var mcpServerProcesses = util.NewHashMap[string, int]()

// MCPServerProcess identifies the root process of one connected stdio MCP server.
type MCPServerProcess struct {
	Name      string
	ProcessID int
}

// ListMCPServerProcesses reports the root process of every stdio MCP server that has a live
// session. Entries are dropped when sessions are reset, so a caller should still confirm that a
// reported process is a live descendant before charging memory to it.
func ListMCPServerProcesses() []MCPServerProcess {
	var processes []MCPServerProcess
	mcpServerProcesses.Range(func(name string, pid int) bool {
		if pid > 0 {
			processes = append(processes, MCPServerProcess{Name: name, ProcessID: pid})
		}
		return true
	})
	sort.Slice(processes, func(left, right int) bool { return processes[left].Name < processes[right].Name })
	return processes
}

// lifetimeBoundCommandTransport starts a stdio MCP server with its whole process tree bound to
// Wox's lifetime, and records its root process so diagnostics can name the server that owns it.
//
// A single configured command expands into a chain of helpers: `uvx <server>` becomes a launcher
// shim, then uv, then a uv-managed interpreter running the server. Unlike the plugin hosts, none
// of them watch the Wox pid, and closing the session only closes stdin of the first one. An
// abnormal Wox exit therefore used to leave the entire chain resident, holding tens of megabytes
// per orphaned generation.
type lifetimeBoundCommandTransport struct {
	serverName string
	command    *exec.Cmd
}

func (t *lifetimeBoundCommandTransport) Connect(ctx context.Context) (mcp.Connection, error) {
	shell.PrepareLifetimeBoundCmd(t.command)

	connection, err := (&mcp.CommandTransport{Command: t.command}).Connect(ctx)
	if err != nil {
		return nil, err
	}

	// The command transport returns as soon as the process is created, which is the only moment
	// the tree can still be captured whole.
	if err = shell.AdoptLifetimeBoundCmd(ctx, t.command); err != nil {
		_ = connection.Close()
		return nil, err
	}

	mcpServerProcesses.Store(t.serverName, t.command.Process.Pid)
	return connection, nil
}
