package host

import (
	"context"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wox/util"
	"wox/util/shell"
)

// TestHostExitHelper is a subprocess fixture that dies before opening its websocket.
func TestHostExitHelper(t *testing.T) {
	mode := os.Getenv("WOX_HOST_EXIT_TEST")
	if mode == "" {
		return
	}
	if mode == "error" {
		_, _ = os.Stderr.WriteString(strings.Repeat("x", 20000) + "\nReferenceError: __dirname is not defined in ES module scope\n")
		os.Exit(7)
	}
	if mode == "idle" {
		time.Sleep(30 * time.Second)
	}
	os.Exit(0)
}

// TestLiveHostConnectionDeadline preserves the timeout path and reaps a host that never listens.
func TestLiveHostConnectionDeadline(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd, err := shell.RunWithEnvLifetimeBoundOutput(context.Background(), executable, []string{"WOX_HOST_EXIT_TEST=idle"}, io.Discard, io.Discard, "-test.run=^TestHostExitHelper$")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancelCause(context.Background())
	defer cancel(nil)
	process := watchHostProcess(cmd, &hostStderr{}, cancel)
	host := &WebsocketHost{host: &NodejsHost{}}
	defer host.terminateHostProcess(context.Background(), process)
	port, err := util.GetAvailableTcpPort(ctx)
	if err != nil {
		t.Fatal(err)
	}
	dialCtx, stop := context.WithTimeout(ctx, 300*time.Millisecond)
	defer stop()
	err = host.startWebsocketServer(dialCtx, port)
	defer host.websocketClient().Close(context.Background())
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected connection deadline, got %v", err)
	}
	select {
	case <-process.done:
		t.Fatal("host exited before its connection deadline")
	default:
	}
}

// TestHostStartupEarlyExit checks the actual process/retry path and the error consumed by Runtime Settings.
func TestHostStartupEarlyExit(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"error", "success"} {
		t.Run(mode, func(t *testing.T) {
			host := &WebsocketHost{host: &NodejsHost{}}
			start := time.Now()
			err := host.StartHost(context.Background(), executable, "-test.run=^TestHostExitHelper$", []string{"WOX_HOST_EXIT_TEST=" + mode})
			defer host.StopHost(context.Background())
			if err == nil || !strings.Contains(err.Error(), "host process exited") {
				t.Fatalf("expected early exit, got %v", err)
			}
			if mode == "error" && !strings.Contains(err.Error(), "ReferenceError: __dirname") {
				t.Fatalf("missing stderr: %v", err)
			}
			if mode == "success" && !strings.Contains(err.Error(), "code 0") {
				t.Fatalf("missing successful early exit: %v", err)
			}
			if time.Since(start) > 10*time.Second {
				t.Fatal("startup waited for retries after process exit")
			}
			if host.GetLastStartError() != err.Error() || host.IsHostStarted(context.Background()) {
				t.Fatal("runtime status does not reflect startup failure")
			}
		})
	}
}

// TestNodeHostIgnoresAncestorModuleType runs the staged production bundle under an ESM package scope.
func TestNodeHostIgnoresAncestorModuleType(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js is not installed")
	}
	bundle, err := os.ReadFile("../../resource/hosts/node-host.cjs")
	if os.IsNotExist(err) {
		t.Skip("build the Node.js host to test the production bundle")
	}
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"type":"module"}`), 0600); err != nil {
		t.Fatal(err)
	}
	directory := filepath.Join(root, ".wox", "hosts")
	if err := os.MkdirAll(directory, 0700); err != nil {
		t.Fatal(err)
	}
	entry := filepath.Join(directory, "node-host.cjs")
	if err := os.WriteFile(entry, bundle, 0600); err != nil {
		t.Fatal(err)
	}
	host := &WebsocketHost{host: &NodejsHost{}, requestMap: util.NewHashMap[string, chan JsonRpcResponse]()}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	defer host.StopHost(context.Background())
	if err := host.StartHost(ctx, node, entry, nil); err != nil {
		t.Fatal(err)
	}
	if !host.IsHostStarted(ctx) || host.GetLastStartError() != "" {
		t.Fatal("host did not reach a healthy runtime status")
	}
}

// TestHostStderrBounded checks that a minified source dump cannot obscure the following exception.
func TestHostStderrBounded(t *testing.T) {
	stderr := &hostStderr{}
	_, _ = stderr.Write([]byte(strings.Repeat("x", 50000)))
	_, _ = stderr.Write([]byte("\nReferenceError: missing dirname\n    at example:1\n"))
	if len(stderr.tail) > 16*1024 || stderr.detail() != "ReferenceError: missing dirname" {
		t.Fatalf("unexpected diagnostic: %q", stderr.detail())
	}
}
