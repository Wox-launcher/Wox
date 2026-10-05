package window

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/godbus/dbus/v5"
)

const (
	kwinBusName             = "org.kde.KWin"
	kwinScriptingPath       = dbus.ObjectPath("/Scripting")
	kwinScriptingIface      = "org.kde.kwin.Scripting"
	kwinScriptIface         = "org.kde.kwin.Script"
	kdeForegroundIface      = "org.wox.WindowForeground"
	kdeForegroundPath       = dbus.ObjectPath("/org/wox/WindowForeground")
	kdeForegroundMethod     = "Report"
	kdeForegroundUnloadWait = 200 * time.Millisecond
)

var (
	kdeForegroundMu  sync.Mutex
	kdeForegroundSeq atomic.Uint64
)

// kwinForegroundPayload is the focused window as KWin exposes it to a script.
// desktopFileName is already the .desktop basename. resourceClass and
// resourceName are the WM_CLASS pair used when that name is empty.
type kwinForegroundPayload struct {
	DesktopFileName string `json:"desktopFileName"`
	ResourceClass   string `json:"resourceClass"`
	ResourceName    string `json:"resourceName"`
	Caption         string `json:"caption"`
	Pid             int    `json:"pid"`
}

type kdeForegroundReport struct {
	payload chan string
}

// Report receives the focused window from the one-shot KWin script.
func (r *kdeForegroundReport) Report(payload string) *dbus.Error {
	select {
	case r.payload <- payload:
	default:
	}
	return nil
}

// queryKDEForeground reads the focused window from KWin.
// The public KWin D-Bus API cannot name that window without an id or a click,
// so a one-shot script reads workspace.activeWindow and answers over the session bus.
func queryKDEForeground(ctx context.Context) (linuxForegroundApp, error) {
	if err := ctx.Err(); err != nil {
		return linuxForegroundApp{}, err
	}
	kdeForegroundMu.Lock()
	defer kdeForegroundMu.Unlock()
	if err := ctx.Err(); err != nil {
		return linuxForegroundApp{}, err
	}
	return readKDEForeground(ctx)
}

func readKDEForeground(ctx context.Context) (linuxForegroundApp, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return linuxForegroundApp{}, fmt.Errorf("kwin foreground bus: %w", err)
	}
	defer conn.Close()

	seq := kdeForegroundSeq.Add(1)
	busName := fmt.Sprintf("org.wox.WindowForeground.P%d.N%d", os.Getpid(), seq)
	pluginName := fmt.Sprintf("woxforeground%dn%d", os.Getpid(), seq)
	script, err := kwinForegroundScript(busName, string(kdeForegroundPath), kdeForegroundIface, kdeForegroundMethod)
	if err != nil {
		return linuxForegroundApp{}, err
	}

	report := &kdeForegroundReport{payload: make(chan string, 1)}
	if err := conn.Export(report, kdeForegroundPath, kdeForegroundIface); err != nil {
		return linuxForegroundApp{}, fmt.Errorf("kwin foreground export: %w", err)
	}
	reply, err := conn.RequestName(busName, dbus.NameFlagDoNotQueue)
	if err != nil {
		return linuxForegroundApp{}, fmt.Errorf("kwin foreground bus name: %w", err)
	}
	if reply != dbus.RequestNameReplyPrimaryOwner && reply != dbus.RequestNameReplyAlreadyOwner {
		return linuxForegroundApp{}, fmt.Errorf("kwin foreground bus name %s was not acquired", busName)
	}

	file, err := os.CreateTemp("", "wox-kwin-foreground-*.js")
	if err != nil {
		return linuxForegroundApp{}, err
	}
	scriptPath := file.Name()
	defer os.Remove(scriptPath)
	if _, err := file.WriteString(script); err != nil {
		file.Close()
		return linuxForegroundApp{}, err
	}
	if err := file.Close(); err != nil {
		return linuxForegroundApp{}, err
	}

	var scriptID int32
	if err := conn.Object(kwinBusName, kwinScriptingPath).CallWithContext(ctx, kwinScriptingIface+".loadScript", 0, scriptPath, pluginName).Store(&scriptID); err != nil {
		return linuxForegroundApp{}, fmt.Errorf("kwin loadScript: %w", err)
	}
	if scriptID < 0 {
		return linuxForegroundApp{}, errors.New("kwin loadScript failed")
	}
	// Unload on every return, including timeout. waitKWinForeground returns after
	// Script.run finishes, so this stop/unload does not cut off the callDBus reply.
	defer unloadKWinForegroundScript(conn, scriptID, pluginName)

	payload, err := waitKWinForeground(ctx, conn, scriptID, report.payload)
	if err != nil {
		return linuxForegroundApp{}, err
	}
	return parseKWinForegroundPayload(payload)
}

// waitKWinForeground lets the script finish after the reply is sent.
// Unloading while callDBus is still blocked drops the focused-window message.
func waitKWinForeground(ctx context.Context, conn *dbus.Conn, scriptID int32, reports <-chan string) (string, error) {
	runDone := make(chan error, 1)
	go func() {
		runDone <- conn.Object(kwinBusName, kwinScriptObjectPath(scriptID)).CallWithContext(ctx, kwinScriptIface+".run", 0).Err
	}()

	var payload string
	select {
	case payload = <-reports:
	case err := <-runDone:
		select {
		case payload = <-reports:
		default:
			if err != nil {
				return "", fmt.Errorf("kwin foreground script: %w", err)
			}
			return "", errLinuxForegroundMissing
		}
	case <-ctx.Done():
		return "", ctx.Err()
	}

	select {
	case <-runDone:
	case <-ctx.Done():
	}
	return payload, nil
}

func unloadKWinForegroundScript(conn *dbus.Conn, scriptID int32, pluginName string) {
	ctx, cancel := context.WithTimeout(context.Background(), kdeForegroundUnloadWait)
	defer cancel()
	conn.Object(kwinBusName, kwinScriptObjectPath(scriptID)).CallWithContext(ctx, kwinScriptIface+".stop", 0)
	conn.Object(kwinBusName, kwinScriptingPath).CallWithContext(ctx, kwinScriptingIface+".unloadScript", 0, pluginName)
}

func kwinScriptObjectPath(scriptID int32) dbus.ObjectPath {
	return dbus.ObjectPath("/Scripting/Script" + strconv.FormatInt(int64(scriptID), 10))
}

// kwinForegroundScript builds a script that reports the focused window once.
// The bus name is interpolated, so it has to be a plain D-Bus token.
func kwinForegroundScript(busName, objectPath, iface, method string) (string, error) {
	for _, token := range []string{busName, objectPath, iface, method} {
		if !kwinDBusTokenOK(token) {
			return "", fmt.Errorf("invalid kwin dbus token %q", token)
		}
	}
	return fmt.Sprintf(kwinForegroundScriptTemplate, busName, objectPath, iface, method), nil
}

func kwinDBusTokenOK(token string) bool {
	if token == "" || len(token) > 255 {
		return false
	}
	for _, r := range token {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '.', r == '/':
		default:
			return false
		}
	}
	return true
}

const kwinForegroundScriptTemplate = `(function () {
  var info = {desktopFileName: "", resourceClass: "", resourceName: "", caption: "", pid: 0};
  try {
    var w = workspace.activeWindow;
    if (w) {
      info.desktopFileName = String(w.desktopFileName || "");
      info.resourceClass = String(w.resourceClass || "");
      info.resourceName = String(w.resourceName || "");
      info.caption = String(w.caption || "");
      info.pid = Number(w.pid) || 0;
    }
  } catch (err) {
  }
  callDBus("%s", "%s", "%s", "%s", JSON.stringify(info));
})();
`

func parseKWinForegroundPayload(payload string) (linuxForegroundApp, error) {
	var info kwinForegroundPayload
	if err := json.Unmarshal([]byte(payload), &info); err != nil {
		return linuxForegroundApp{}, fmt.Errorf("kwin foreground: %w", err)
	}
	app := linuxForegroundApp{Pid: info.Pid, Title: info.Caption}
	desktopID := normalizeLinuxAppID(info.DesktopFileName)
	classID := normalizeLinuxAppID(info.ResourceClass)
	nameID := normalizeLinuxAppID(info.ResourceName)
	if desktopID != "" {
		app.AppID = desktopID
		app.AltID = firstLinuxName(classID, nameID)
	} else {
		app.AppID = classID
		app.AltID = nameID
	}
	if app.AppID == "" && app.AltID == "" && app.Pid <= 0 {
		return linuxForegroundApp{}, errLinuxForegroundMissing
	}
	return app, nil
}
