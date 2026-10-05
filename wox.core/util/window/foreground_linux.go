package window

import (
	"context"
	"errors"
	"image"
	"sync"
	"time"
	"wox/util"
)

const linuxForegroundTimeout = 200 * time.Millisecond
const linuxForegroundCacheTTL = 200 * time.Millisecond

var (
	errLinuxForegroundUnavailable = errors.New("focused application is unavailable on this Linux session")
	errLinuxForegroundMissing     = errors.New("focused application is missing")
)

// linuxForegroundApp is the focused window plus the desktop entry matched to it.
// Identity is the .desktop basename, which is what the ignore-application picker stores.
type linuxForegroundApp struct {
	Pid      int
	Title    string
	AppID    string
	AltID    string
	Identity string
	Name     string
	IconPath string
}

type linuxKnownApp struct {
	Name     string
	Identity string
	IconPath string
}

type linuxForegroundCacheState struct {
	at  time.Time
	app linuxForegroundApp
	err error
	ok  bool
}

var (
	linuxForegroundMu    sync.Mutex
	linuxForegroundCache linuxForegroundCacheState

	linuxKnownMu   sync.Mutex
	linuxKnownApps = map[int]linuxKnownApp{}
)

// linuxForegroundBackend chooses who can answer "which application is focused".
// Hyprland exposes it through hyprctl. X11 exposes it through EWMH. GNOME and
// KDE Wayland do not give a normal client the focused window's application id.
func linuxForegroundBackend(wayland bool, hyprland bool) string {
	if !wayland {
		return "x11"
	}
	if hyprland {
		return "hyprland"
	}
	return ""
}

func linuxActiveWindowIcon() (image.Image, error) {
	app, err := currentLinuxForeground()
	if err != nil {
		return nil, err
	}
	if app.IconPath == "" {
		return nil, errors.New("focused application has no icon")
	}
	return loadLinuxAppIcon(app.IconPath)
}

func linuxWindowIconByPid(pid int) (image.Image, error) {
	app, ok := linuxAppForPid(pid)
	if !ok || app.IconPath == "" {
		return nil, errors.New("application icon is unavailable")
	}
	return loadLinuxAppIcon(app.IconPath)
}

func linuxActiveWindowName() string {
	app, err := currentLinuxForeground()
	if err != nil {
		return ""
	}
	return app.Name
}

func linuxWindowNameByPid(pid int) string {
	app, ok := linuxAppForPid(pid)
	if !ok {
		return ""
	}
	return app.Name
}

func linuxActiveWindowPid() int {
	app, err := currentLinuxForeground()
	if err != nil || app.Pid <= 0 {
		return -1
	}
	return app.Pid
}

func linuxProcessIdentity(pid int) string {
	app, ok := linuxAppForPid(pid)
	if !ok {
		return ""
	}
	return app.Identity
}

// currentLinuxForeground reads the focused application once and reuses it for
// the pid, name, and icon lookups that follow a single clipboard change.
func currentLinuxForeground() (linuxForegroundApp, error) {
	if app, err, ok := cachedLinuxForeground(); ok {
		return app, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), linuxForegroundTimeout)
	defer cancel()
	app, err := queryLinuxForeground(ctx)
	if err == nil {
		app = finishLinuxForeground(app)
	}
	storeLinuxForeground(app, err)
	return app, err
}

func queryLinuxForeground(ctx context.Context) (linuxForegroundApp, error) {
	switch linuxForegroundBackend(util.IsLinuxWaylandSession(), util.IsHyprlandSession()) {
	case "hyprland":
		return queryHyprlandForeground(ctx)
	case "x11":
		return queryX11Foreground(ctx)
	default:
		return linuxForegroundApp{}, errLinuxForegroundUnavailable
	}
}

// finishLinuxForeground maps the compositor's app id onto a desktop entry.
// StartupWMClass covers ids such as google-chrome whose file is com.google.Chrome.desktop.
func finishLinuxForeground(app linuxForegroundApp) linuxForegroundApp {
	app.AppID = normalizeLinuxAppID(app.AppID)
	app.AltID = normalizeLinuxAppID(app.AltID)
	exeBase, comm := "", ""
	if app.Pid > 0 {
		exeBase, comm = linuxProcCommand(app.Pid)
	}

	apps := linuxDesktopApps()
	matched, ok := matchLinuxDesktopApp(apps, app.AppID, app.AltID)
	if !ok {
		matched, ok = matchLinuxDesktopApp(apps, exeBase, comm)
	}
	if ok {
		app.Identity = matched.ID
		if matched.Name != "" {
			app.Name = matched.Name
		}
		if matched.Icon != "" {
			app.IconPath = resolveLinuxAppIconPath(matched.Icon)
		}
	}
	if app.Identity == "" {
		app.Identity = firstLinuxName(app.AppID, exeBase, comm)
	}
	if app.Name == "" {
		app.Name = firstLinuxName(app.Title, app.AppID, exeBase, comm)
	}
	if app.IconPath == "" {
		app.IconPath = resolveLinuxAppIconPath(firstLinuxName(app.AppID, app.Identity))
	}
	rememberLinuxApp(app.Pid, linuxKnownApp{Name: app.Name, Identity: app.Identity, IconPath: app.IconPath})
	return app
}

func linuxAppForPid(pid int) (linuxKnownApp, bool) {
	if pid <= 0 {
		return linuxKnownApp{}, false
	}
	if app, ok := lookupLinuxApp(pid); ok && (app.Identity != "" || app.Name != "" || app.IconPath != "") {
		return app, true
	}
	exeBase, comm := linuxProcCommand(pid)
	matched, ok := matchLinuxDesktopApp(linuxDesktopApps(), exeBase, comm)
	app := linuxKnownApp{}
	if ok {
		app.Identity = matched.ID
		app.Name = matched.Name
		if matched.Icon != "" {
			app.IconPath = resolveLinuxAppIconPath(matched.Icon)
		}
	}
	if app.Identity == "" {
		app.Identity = firstLinuxName(exeBase, comm)
	}
	if app.Name == "" {
		app.Name = app.Identity
	}
	if app.IconPath == "" {
		app.IconPath = resolveLinuxAppIconPath(app.Identity)
	}
	if app.Identity == "" && app.Name == "" && app.IconPath == "" {
		return linuxKnownApp{}, false
	}
	rememberLinuxApp(pid, app)
	return app, true
}

func cachedLinuxForeground() (linuxForegroundApp, error, bool) {
	linuxForegroundMu.Lock()
	defer linuxForegroundMu.Unlock()
	if !linuxForegroundCache.ok || time.Since(linuxForegroundCache.at) >= linuxForegroundCacheTTL {
		return linuxForegroundApp{}, nil, false
	}
	return linuxForegroundCache.app, linuxForegroundCache.err, true
}

func storeLinuxForeground(app linuxForegroundApp, err error) {
	linuxForegroundMu.Lock()
	linuxForegroundCache = linuxForegroundCacheState{at: time.Now(), app: app, err: err, ok: true}
	linuxForegroundMu.Unlock()
}

func lookupLinuxApp(pid int) (linuxKnownApp, bool) {
	linuxKnownMu.Lock()
	defer linuxKnownMu.Unlock()
	app, ok := linuxKnownApps[pid]
	return app, ok
}

func rememberLinuxApp(pid int, app linuxKnownApp) {
	if pid <= 0 {
		return
	}
	linuxKnownMu.Lock()
	if len(linuxKnownApps) > 256 {
		linuxKnownApps = map[int]linuxKnownApp{}
	}
	linuxKnownApps[pid] = app
	linuxKnownMu.Unlock()
}

func resetLinuxForegroundState() {
	linuxForegroundMu.Lock()
	linuxForegroundCache = linuxForegroundCacheState{}
	linuxForegroundMu.Unlock()
	linuxKnownMu.Lock()
	linuxKnownApps = map[int]linuxKnownApp{}
	linuxKnownMu.Unlock()
	resetLinuxDesktopApps()
}

func firstLinuxName(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
