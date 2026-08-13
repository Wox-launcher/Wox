package tray

// Linux implements the tray through StatusNotifierItem and dbusmenu over D-Bus.
// Linking libayatana-appindicator3 would make Wox fail at process start on
// distros such as Fedora that do not ship that library, including AppImage.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/godbus/dbus/v5"
	"github.com/godbus/dbus/v5/introspect"
	"github.com/godbus/dbus/v5/prop"

	"wox/util"
)

const (
	sniInterface      = "org.kde.StatusNotifierItem"
	sniItemPath       = "/StatusNotifierItem"
	sniMenuPath       = "/StatusNotifierItem/Menu"
	dbusMenuInterface = "com.canonical.dbusmenu"

	kdeStatusNotifierWatcher  = "org.kde.StatusNotifierWatcher"
	fdoStatusNotifierWatcher  = "org.freedesktop.StatusNotifierWatcher"
	statusNotifierWatcherPath = "/StatusNotifierWatcher"
)

var (
	trayMu    sync.Mutex
	linuxHost *linuxTray
)

type sniIconPixmap struct {
	Width  int32
	Height int32
	Pixels []byte
}

type sniToolTip struct {
	IconName    string
	IconPixmap  []sniIconPixmap
	Title       string
	Description string
}

type menuLayout struct {
	ID         int32
	Properties map[string]dbus.Variant
	Children   []dbus.Variant
}

type menuPropertyGroup struct {
	ID         int32
	Properties map[string]dbus.Variant
}

type linuxTray struct {
	conn      *dbus.Conn
	done      chan struct{}
	closeOnce sync.Once

	mu        sync.Mutex
	leftClick func()
	items     []menuEntry
	revision  uint32
	iconFile  string
}

type menuEntry struct {
	id       int32
	title    string
	callback func()
}

type sniServer struct {
	host *linuxTray
}

type dbusMenuServer struct {
	host *linuxTray
}

func CreateTray(appIcon []byte, onClick func(), items ...MenuItem) {
	RemoveTray()

	host, err := startLinuxTray(appIcon, onClick, items)
	if err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("Linux tray: %s", err.Error()))
		return
	}

	trayMu.Lock()
	linuxHost = host
	trayMu.Unlock()
}

func RemoveTray() {
	trayMu.Lock()
	host := linuxHost
	linuxHost = nil
	trayMu.Unlock()
	if host != nil {
		host.close()
	}
}

func SetQueryIcons(items []QueryIconItem) {
	// Linux query tray icons are not implemented yet.
}

func startLinuxTray(appIcon []byte, onClick func(), items []MenuItem) (*linuxTray, error) {
	conn, err := dbus.ConnectSessionBus()
	if err != nil {
		return nil, fmt.Errorf("connect to session bus: %w", err)
	}

	host := &linuxTray{
		conn:      conn,
		done:      make(chan struct{}),
		leftClick: onClick,
		items:     makeMenuEntries(items),
		revision:  1,
	}

	pixmaps, iconName, iconThemePath, iconFile := prepareTrayIcon(appIcon)
	host.iconFile = iconFile

	sniProps := map[string]map[string]*prop.Prop{
		sniInterface: {
			"Category":            {Value: "ApplicationStatus", Writable: false, Emit: prop.EmitTrue},
			"Id":                  {Value: util.LinuxDesktopAppID, Writable: false, Emit: prop.EmitTrue},
			"Title":               {Value: "Wox", Writable: false, Emit: prop.EmitTrue},
			"Status":              {Value: "Active", Writable: false, Emit: prop.EmitTrue},
			"WindowId":            {Value: int32(0), Writable: false, Emit: prop.EmitTrue},
			"IconName":            {Value: iconName, Writable: false, Emit: prop.EmitTrue},
			"IconPixmap":          {Value: pixmaps, Writable: false, Emit: prop.EmitTrue},
			"OverlayIconName":     {Value: "", Writable: false, Emit: prop.EmitTrue},
			"OverlayIconPixmap":   {Value: []sniIconPixmap{}, Writable: false, Emit: prop.EmitTrue},
			"AttentionIconName":   {Value: "", Writable: false, Emit: prop.EmitTrue},
			"AttentionIconPixmap": {Value: []sniIconPixmap{}, Writable: false, Emit: prop.EmitTrue},
			"AttentionMovieName":  {Value: "", Writable: false, Emit: prop.EmitTrue},
			"ToolTip":             {Value: sniToolTip{IconName: iconName, IconPixmap: pixmaps, Title: "Wox"}, Writable: false, Emit: prop.EmitTrue},
			"ItemIsMenu":          {Value: false, Writable: false, Emit: prop.EmitTrue},
			"Menu":                {Value: dbus.ObjectPath(sniMenuPath), Writable: false, Emit: prop.EmitTrue},
			"IconThemePath":       {Value: iconThemePath, Writable: false, Emit: prop.EmitTrue},
		},
	}
	menuProps := map[string]map[string]*prop.Prop{
		dbusMenuInterface: {
			"Version":       {Value: uint32(3), Writable: false, Emit: prop.EmitTrue},
			"TextDirection": {Value: "ltr", Writable: false, Emit: prop.EmitTrue},
			"Status":        {Value: "normal", Writable: false, Emit: prop.EmitTrue},
			"IconThemePath": {Value: []string{}, Writable: false, Emit: prop.EmitTrue},
		},
	}

	sni := &sniServer{host: host}
	menu := &dbusMenuServer{host: host}
	if err := conn.Export(sni, sniItemPath, sniInterface); err != nil {
		host.close()
		return nil, fmt.Errorf("export StatusNotifierItem: %w", err)
	}
	if _, err := prop.Export(conn, sniItemPath, sniProps); err != nil {
		host.close()
		return nil, fmt.Errorf("export StatusNotifierItem properties: %w", err)
	}
	if err := conn.Export(introspect.NewIntrospectable(sniIntrospectNode()), sniItemPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		host.close()
		return nil, fmt.Errorf("export StatusNotifierItem introspect: %w", err)
	}
	if err := conn.Export(menu, sniMenuPath, dbusMenuInterface); err != nil {
		host.close()
		return nil, fmt.Errorf("export tray dbusmenu: %w", err)
	}
	if _, err := prop.Export(conn, sniMenuPath, menuProps); err != nil {
		host.close()
		return nil, fmt.Errorf("export tray dbusmenu properties: %w", err)
	}
	if err := conn.Export(introspect.NewIntrospectable(dbusMenuIntrospectNode()), sniMenuPath, "org.freedesktop.DBus.Introspectable"); err != nil {
		host.close()
		return nil, fmt.Errorf("export tray dbusmenu introspect: %w", err)
	}

	if err := conn.AddMatchSignal(
		dbus.WithMatchSender("org.freedesktop.DBus"),
		dbus.WithMatchInterface("org.freedesktop.DBus"),
		dbus.WithMatchMember("NameOwnerChanged"),
	); err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("Linux tray: watch StatusNotifierWatcher: %s", err.Error()))
	} else {
		signals := make(chan *dbus.Signal, 8)
		conn.Signal(signals)
		go host.watchWatcher(signals)
	}

	if !host.register() {
		util.GetLogger().Warn(util.NewTraceContext(), "Linux tray: StatusNotifierWatcher is not available; the icon will appear once a tray host is running")
	} else {
		util.GetLogger().Info(util.NewTraceContext(), "Linux tray: registered StatusNotifierItem")
	}

	return host, nil
}

func (h *linuxTray) close() {
	h.closeOnce.Do(func() {
		close(h.done)
		if h.conn != nil {
			_ = h.conn.Close()
		}
		if h.iconFile != "" {
			_ = os.Remove(h.iconFile)
		}
	})
}

func (h *linuxTray) register() bool {
	if h.conn == nil {
		return false
	}

	services := []string{sniItemPath}
	if names := h.conn.Names(); len(names) > 0 {
		services = append(services, names[0])
	}

	for _, dest := range []string{kdeStatusNotifierWatcher, fdoStatusNotifierWatcher} {
		for _, iface := range []string{kdeStatusNotifierWatcher, dest} {
			for _, service := range services {
				call := h.conn.Object(dest, statusNotifierWatcherPath).Call(iface+".RegisterStatusNotifierItem", 0, service)
				if call.Err == nil {
					return true
				}
			}
		}
	}
	return false
}

func (h *linuxTray) watchWatcher(signals <-chan *dbus.Signal) {
	for {
		select {
		case <-h.done:
			return
		case signal, ok := <-signals:
			if !ok {
				return
			}
			if signal == nil || signal.Name != "org.freedesktop.DBus.NameOwnerChanged" || len(signal.Body) < 3 {
				continue
			}
			name, _ := signal.Body[0].(string)
			newOwner, _ := signal.Body[2].(string)
			if newOwner == "" {
				continue
			}
			if name != kdeStatusNotifierWatcher && name != fdoStatusNotifierWatcher {
				continue
			}
			if h.register() {
				util.GetLogger().Info(util.NewTraceContext(), fmt.Sprintf("Linux tray: registered StatusNotifierItem with %s", name))
			}
		}
	}
}

func (s *sniServer) Activate(x int32, y int32) *dbus.Error {
	s.host.mu.Lock()
	callback := s.host.leftClick
	s.host.mu.Unlock()
	if callback != nil {
		go callback()
	}
	return nil
}

func (s *sniServer) SecondaryActivate(x int32, y int32) *dbus.Error {
	return s.Activate(x, y)
}

func (s *sniServer) ContextMenu(x int32, y int32) *dbus.Error {
	// Hosts that support dbusmenu use the Menu object path instead of this method.
	return nil
}

func (s *sniServer) Scroll(delta int32, orientation string) *dbus.Error {
	return nil
}

func (m *dbusMenuServer) GetLayout(parentId int32, recursionDepth int32, propertyNames []string) (uint32, menuLayout, *dbus.Error) {
	m.host.mu.Lock()
	defer m.host.mu.Unlock()
	layout, ok := buildMenuLayout(m.host.items, parentId, recursionDepth, propertyNames)
	if !ok {
		return m.host.revision, menuLayout{}, dbus.NewError("com.canonical.dbusmenu.Error.UnknownId", []interface{}{parentId})
	}
	return m.host.revision, layout, nil
}

func (m *dbusMenuServer) GetGroupProperties(ids []int32, propertyNames []string) ([]menuPropertyGroup, *dbus.Error) {
	m.host.mu.Lock()
	defer m.host.mu.Unlock()

	groups := make([]menuPropertyGroup, 0, len(ids))
	for _, id := range ids {
		props, ok := menuItemProperties(m.host.items, id)
		if !ok {
			continue
		}
		groups = append(groups, menuPropertyGroup{ID: id, Properties: filterMenuProperties(props, propertyNames)})
	}
	return groups, nil
}

func (m *dbusMenuServer) GetProperty(id int32, name string) (dbus.Variant, *dbus.Error) {
	m.host.mu.Lock()
	defer m.host.mu.Unlock()

	props, ok := menuItemProperties(m.host.items, id)
	if !ok {
		return dbus.Variant{}, dbus.NewError("com.canonical.dbusmenu.Error.UnknownId", []interface{}{id})
	}
	value, exists := props[name]
	if !exists {
		return dbus.Variant{}, dbus.NewError("com.canonical.dbusmenu.Error.UnknownProperty", []interface{}{name})
	}
	return value, nil
}

func (m *dbusMenuServer) Event(id int32, eventId string, data dbus.Variant, timestamp uint32) *dbus.Error {
	if eventId != "clicked" {
		return nil
	}

	m.host.mu.Lock()
	var callback func()
	for _, item := range m.host.items {
		if item.id == id {
			callback = item.callback
			break
		}
	}
	m.host.mu.Unlock()

	util.GetLogger().Info(context.Background(), fmt.Sprintf("Wox tray menu activate: id=%d", id))
	if callback != nil {
		go callback()
	}
	return nil
}

func (m *dbusMenuServer) AboutToShow(id int32) (bool, *dbus.Error) {
	return false, nil
}

func makeMenuEntries(items []MenuItem) []menuEntry {
	entries := make([]menuEntry, 0, len(items))
	for i, item := range items {
		entries = append(entries, menuEntry{
			id:       int32(i + 1),
			title:    item.Title,
			callback: item.Callback,
		})
	}
	return entries
}

func buildMenuLayout(items []menuEntry, parentID int32, recursionDepth int32, propertyNames []string) (menuLayout, bool) {
	if parentID == 0 {
		layout := menuLayout{
			ID: 0,
			Properties: filterMenuProperties(map[string]dbus.Variant{
				"children-display": dbus.MakeVariant("submenu"),
			}, propertyNames),
		}
		if recursionDepth != 0 {
			childDepth := int32(-1)
			if recursionDepth > 0 {
				childDepth = recursionDepth - 1
			}
			for _, item := range items {
				child, _ := buildMenuLayout(items, item.id, childDepth, propertyNames)
				layout.Children = append(layout.Children, dbus.MakeVariant(child))
			}
		}
		return layout, true
	}

	props, ok := menuItemProperties(items, parentID)
	if !ok {
		return menuLayout{}, false
	}
	return menuLayout{
		ID:         parentID,
		Properties: filterMenuProperties(props, propertyNames),
	}, true
}

func menuItemProperties(items []menuEntry, id int32) (map[string]dbus.Variant, bool) {
	if id == 0 {
		return map[string]dbus.Variant{
			"children-display": dbus.MakeVariant("submenu"),
		}, true
	}
	for _, item := range items {
		if item.id == id {
			return map[string]dbus.Variant{
				"label":   dbus.MakeVariant(item.title),
				"enabled": dbus.MakeVariant(true),
				"visible": dbus.MakeVariant(true),
			}, true
		}
	}
	return nil, false
}

func filterMenuProperties(props map[string]dbus.Variant, names []string) map[string]dbus.Variant {
	if len(names) == 0 {
		return props
	}
	filtered := make(map[string]dbus.Variant, len(names))
	for _, name := range names {
		if value, ok := props[name]; ok {
			filtered[name] = value
		}
	}
	return filtered
}

func prepareTrayIcon(appIcon []byte) (pixmaps []sniIconPixmap, iconName string, iconThemePath string, iconFile string) {
	pixmaps = []sniIconPixmap{}
	if decoded := decodeTrayIcon(appIcon); decoded != nil {
		pixmaps = buildSNIIconPixmaps(decoded)
	}

	if len(appIcon) == 0 {
		return pixmaps, util.LinuxDesktopAppID, "", ""
	}

	iconDir := filepath.Join(util.GetLocation().GetCacheDirectory(), "tray-icons")
	if err := os.MkdirAll(iconDir, 0o700); err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("Linux tray: create icon cache: %s", err.Error()))
		return pixmaps, util.LinuxDesktopAppID, "", ""
	}

	iconFile = filepath.Join(iconDir, "wox-tray.png")
	if err := os.WriteFile(iconFile, appIcon, 0o600); err != nil {
		util.GetLogger().Warn(util.NewTraceContext(), fmt.Sprintf("Linux tray: write icon file: %s", err.Error()))
		return pixmaps, util.LinuxDesktopAppID, "", ""
	}

	return pixmaps, "wox-tray", iconDir, iconFile
}

func sniIntrospectNode() *introspect.Node {
	return &introspect.Node{
		Name: sniItemPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:    sniInterface,
				Methods: introspect.Methods(&sniServer{}),
				Properties: []introspect.Property{
					{Name: "Category", Type: "s", Access: "read"},
					{Name: "Id", Type: "s", Access: "read"},
					{Name: "Title", Type: "s", Access: "read"},
					{Name: "Status", Type: "s", Access: "read"},
					{Name: "WindowId", Type: "i", Access: "read"},
					{Name: "IconName", Type: "s", Access: "read"},
					{Name: "IconPixmap", Type: "a(iiay)", Access: "read"},
					{Name: "OverlayIconName", Type: "s", Access: "read"},
					{Name: "OverlayIconPixmap", Type: "a(iiay)", Access: "read"},
					{Name: "AttentionIconName", Type: "s", Access: "read"},
					{Name: "AttentionIconPixmap", Type: "a(iiay)", Access: "read"},
					{Name: "AttentionMovieName", Type: "s", Access: "read"},
					{Name: "ToolTip", Type: "(sa(iiay)ss)", Access: "read"},
					{Name: "ItemIsMenu", Type: "b", Access: "read"},
					{Name: "Menu", Type: "o", Access: "read"},
					{Name: "IconThemePath", Type: "s", Access: "read"},
				},
				Signals: []introspect.Signal{
					{Name: "NewTitle"},
					{Name: "NewIcon"},
					{Name: "NewAttentionIcon"},
					{Name: "NewOverlayIcon"},
					{Name: "NewToolTip"},
					{Name: "NewStatus", Args: []introspect.Arg{{Name: "status", Type: "s"}}},
				},
			},
		},
	}
}

func dbusMenuIntrospectNode() *introspect.Node {
	return &introspect.Node{
		Name: sniMenuPath,
		Interfaces: []introspect.Interface{
			introspect.IntrospectData,
			prop.IntrospectData,
			{
				Name:    dbusMenuInterface,
				Methods: introspect.Methods(&dbusMenuServer{}),
				Properties: []introspect.Property{
					{Name: "Version", Type: "u", Access: "read"},
					{Name: "TextDirection", Type: "s", Access: "read"},
					{Name: "Status", Type: "s", Access: "read"},
					{Name: "IconThemePath", Type: "as", Access: "read"},
				},
				Signals: []introspect.Signal{
					{
						Name: "ItemsPropertiesUpdated",
						Args: []introspect.Arg{
							{Name: "updatedProps", Type: "a(ia{sv})"},
							{Name: "removedProps", Type: "a(ias)"},
						},
					},
					{
						Name: "LayoutUpdated",
						Args: []introspect.Arg{
							{Name: "revision", Type: "u"},
							{Name: "parentId", Type: "i"},
						},
					},
				},
			},
		},
	}
}
