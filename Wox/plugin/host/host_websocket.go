package host

import (
	"context"
	"fmt"
	"net"
	"os/exec"
	"wox/plugin"
	"wox/util"
)

var logger = util.GetLogger()

type WebsocketHost struct {
	this plugin.Host
}

func (w *WebsocketHost) logIdentity(ctx context.Context) string {
	return fmt.Sprintf("[%s host]", w.this.GetRuntime(ctx))
}

func (w *WebsocketHost) StartHost(ctx context.Context, executablePath string, entry string) error {
	port, portErr := w.getAvailableTcpPort(ctx)
	if portErr != nil {
		return fmt.Errorf("failed to get available port: %w", portErr)
	}

	logger.Info(ctx, fmt.Sprintf("%s Starting host on port %d", w.logIdentity(ctx), port))
	cmd := exec.Command(executablePath, entry, fmt.Sprintf("%d", port))
	err := cmd.Run()
	if err != nil {
		return fmt.Errorf("failed to start host: %w", err)
	}

	ctx.Value("trace")
	ctx.Done()

	return nil
}

func (w *WebsocketHost) getAvailableTcpPort(ctx context.Context) (port int, err error) {
	var a *net.TCPAddr
	if a, err = net.ResolveTCPAddr("tcp", "localhost:0"); err == nil {
		var l *net.TCPListener
		if l, err = net.ListenTCP("tcp", a); err == nil {
			defer l.Close()
			return l.Addr().(*net.TCPAddr).Port, nil
		}
	}
	return
}
