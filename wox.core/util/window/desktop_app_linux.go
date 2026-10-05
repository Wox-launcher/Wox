package window

import (
	"bufio"
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"wox/util"
	"wox/util/svg"
)

const linuxDesktopCacheTTL = 30 * time.Second

// linuxDesktopApp is the slice of a launcher needed to name a focused window.
// ID matches resolveAppIdentityForPlatform: the desktop file's base name.
type linuxDesktopApp struct {
	ID             string
	Name           string
	StartupWMClass string
	Icon           string
	ExecBase       string
	NoDisplay      bool
}

type linuxDesktopCacheState struct {
	at   time.Time
	key  string
	apps []linuxDesktopApp
}

var (
	linuxDesktopMu    sync.Mutex
	linuxDesktopCache linuxDesktopCacheState
)

func linuxDesktopApps() []linuxDesktopApp {
	key := strings.Join(linuxDesktopSearchRoots(), "\x00")
	linuxDesktopMu.Lock()
	defer linuxDesktopMu.Unlock()
	if linuxDesktopCache.apps != nil && linuxDesktopCache.key == key && time.Since(linuxDesktopCache.at) < linuxDesktopCacheTTL {
		return linuxDesktopCache.apps
	}
	apps := scanLinuxDesktopApps()
	linuxDesktopCache = linuxDesktopCacheState{at: time.Now(), key: key, apps: apps}
	return apps
}

func resetLinuxDesktopApps() {
	linuxDesktopMu.Lock()
	linuxDesktopCache = linuxDesktopCacheState{}
	linuxDesktopMu.Unlock()
}

func scanLinuxDesktopApps() []linuxDesktopApp {
	apps := []linuxDesktopApp{}
	seen := map[string]struct{}{}
	for _, root := range linuxDesktopSearchRoots() {
		walkLinuxDesktopFiles(root, func(path string) {
			if _, ok := seen[path]; ok {
				return
			}
			seen[path] = struct{}{}
			app, ok := parseLinuxDesktopApp(path)
			if !ok {
				return
			}
			apps = append(apps, app)
		})
	}
	return apps
}

// linuxDesktopSearchRoots follows the launcher directories indexed by the app plugin.
// User directories come first so an override wins a tie against the system copy.
func linuxDesktopSearchRoots() []string {
	home, _ := os.UserHomeDir()
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataDirs := strings.Split(os.Getenv("XDG_DATA_DIRS"), ":")
	if len(dataDirs) == 1 && strings.TrimSpace(dataDirs[0]) == "" {
		dataDirs = []string{"/usr/local/share", "/usr/share"}
	}

	roots := []string{}
	if dataHome != "" {
		roots = append(roots, filepath.Join(dataHome, "applications"))
	}
	if home != "" {
		roots = append(roots, filepath.Join(home, ".local", "share", "flatpak", "exports", "share", "applications"))
	}
	roots = append(roots,
		"/var/lib/flatpak/exports/share/applications",
		"/var/lib/snapd/desktop/applications",
		"/snap/applications",
	)
	for _, dir := range dataDirs {
		dir = strings.TrimSpace(dir)
		if dir == "" {
			continue
		}
		roots = append(roots, filepath.Join(dir, "applications"))
	}
	return util.UniqueStrings(roots)
}

func walkLinuxDesktopFiles(root string, visit func(string)) {
	root = filepath.Clean(strings.TrimSpace(root))
	if root == "" || root == "." {
		return
	}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return nil
		}
		if entry.IsDir() {
			if rel != "." && strings.Count(rel, string(os.PathSeparator)) >= 4 {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(entry.Name()), ".desktop") {
			visit(path)
		}
		return nil
	})
}

func parseLinuxDesktopApp(path string) (linuxDesktopApp, bool) {
	file, err := os.Open(path)
	if err != nil {
		return linuxDesktopApp{}, false
	}
	defer file.Close()

	values := map[string]string{}
	inEntry := false
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			inEntry = strings.EqualFold(line, "[Desktop Entry]")
			continue
		}
		if !inEntry {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		values[strings.TrimSpace(key)] = unescapeLinuxDesktopValue(strings.TrimSpace(value))
	}

	if linuxDesktopBool(values["Hidden"]) {
		return linuxDesktopApp{}, false
	}
	if entryType := values["Type"]; entryType != "" && !strings.EqualFold(entryType, "Application") {
		return linuxDesktopApp{}, false
	}
	base := filepath.Base(path)
	if !strings.HasSuffix(strings.ToLower(base), ".desktop") {
		return linuxDesktopApp{}, false
	}
	return linuxDesktopApp{
		ID:             base[:len(base)-len(".desktop")],
		Name:           preferredLinuxDesktopName(values),
		StartupWMClass: strings.TrimSpace(values["StartupWMClass"]),
		Icon:           strings.TrimSpace(values["Icon"]),
		ExecBase:       linuxExecBase(values["Exec"]),
		NoDisplay:      linuxDesktopBool(values["NoDisplay"]),
	}, true
}

func preferredLinuxDesktopName(values map[string]string) string {
	lang := strings.TrimSpace(os.Getenv("LANG"))
	if cut := strings.IndexAny(lang, ".@"); cut >= 0 {
		lang = lang[:cut]
	}
	if lang != "" {
		if value := strings.TrimSpace(values["Name["+lang+"]"]); value != "" {
			return value
		}
		if cut := strings.Index(lang, "_"); cut > 0 {
			if value := strings.TrimSpace(values["Name["+lang[:cut]+"]"]); value != "" {
				return value
			}
		}
	}
	return strings.TrimSpace(values["Name"])
}

func linuxDesktopBool(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

func unescapeLinuxDesktopValue(value string) string {
	return strings.NewReplacer(
		`\\`, `\`,
		`\s`, " ",
		`\n`, "\n",
		`\t`, "\t",
		`\;`, ";",
	).Replace(value)
}

// linuxExecBase returns the launched binary. Flatpak's --command= is the real
// program; wrappers such as flatpak itself are not an application id.
func linuxExecBase(line string) string {
	fields := strings.Fields(line)
	command := ""
	for _, field := range fields {
		if strings.HasPrefix(field, "--command=") {
			command = strings.TrimPrefix(field, "--command=")
			break
		}
	}
	if command == "" {
		for _, field := range fields {
			if field == "env" || strings.HasPrefix(field, "%") || linuxEnvAssignment(field) {
				continue
			}
			command = field
			break
		}
	}
	return usableLinuxCommandName(filepath.Base(command))
}

func linuxEnvAssignment(field string) bool {
	eq := strings.IndexByte(field, '=')
	if eq <= 0 {
		return false
	}
	for index, char := range field[:eq] {
		if char == '_' || (char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') || (index > 0 && char >= '0' && char <= '9') {
			continue
		}
		return false
	}
	return true
}

func usableLinuxCommandName(name string) string {
	name = strings.TrimSpace(strings.TrimSuffix(name, " (deleted)"))
	switch name {
	case "", ".", "/", "flatpak", "snap", "bwrap", "gio", "gtk-launch", "gapplication", "systemd-run", "env":
		return ""
	default:
		return name
	}
}

func normalizeLinuxAppID(id string) string {
	id = strings.TrimSpace(id)
	id = strings.TrimSuffix(id, ".desktop")
	return id
}

// matchLinuxDesktopApp picks the launcher a focused window belongs to.
// An exact desktop id outranks StartupWMClass, which outranks the executable name.
// NoDisplay handlers stay available for icons but lose to a visible launcher.
func matchLinuxDesktopApp(apps []linuxDesktopApp, ids ...string) (linuxDesktopApp, bool) {
	best := linuxDesktopApp{}
	bestScore := 0
	found := false
	for _, app := range apps {
		score := linuxDesktopScore(app, ids...)
		if score > bestScore {
			best = app
			bestScore = score
			found = true
		}
	}
	return best, found
}

func linuxDesktopScore(app linuxDesktopApp, ids ...string) int {
	best := 0
	for _, id := range ids {
		id = normalizeLinuxAppID(id)
		if id == "" {
			continue
		}
		score := 0
		switch {
		case strings.EqualFold(app.ID, id):
			score = 100
		case app.StartupWMClass != "" && strings.EqualFold(app.StartupWMClass, id):
			score = 80
		case app.ExecBase != "" && strings.EqualFold(app.ExecBase, id):
			score = 50
		}
		if score > best {
			best = score
		}
	}
	if best == 0 {
		return 0
	}
	if app.NoDisplay {
		best -= 5
	}
	return best
}

func linuxProcCommand(pid int) (string, string) {
	if pid <= 0 {
		return "", ""
	}
	exeBase := ""
	if exe, err := os.Readlink(linuxProcPath(pid, "exe")); err == nil {
		exeBase = usableLinuxCommandName(filepath.Base(exe))
	}
	comm := ""
	if data, err := os.ReadFile(linuxProcPath(pid, "comm")); err == nil {
		comm = usableLinuxCommandName(strings.TrimSpace(string(data)))
	}
	if comm == exeBase {
		comm = ""
	}
	return exeBase, comm
}

func linuxProcPath(pid int, name string) string {
	return filepath.Join("/proc", strconv.Itoa(pid), name)
}

// resolveLinuxAppIconPath finds a theme icon large enough to stay sharp as a history glyph.
// A 16px file in the active theme does not hide the scalable icon shipped with the application.
func resolveLinuxAppIconPath(iconValue string) string {
	iconValue = strings.TrimSpace(iconValue)
	if iconValue == "" {
		return ""
	}
	if filepath.IsAbs(iconValue) && linuxIsFile(iconValue) {
		return iconValue
	}
	if resolved := resolveLinuxAbsoluteIcon(iconValue); resolved != "" {
		return resolved
	}

	names := linuxIconNames(iconValue)
	fallback := ""
	for _, root := range linuxIconThemeRoots() {
		path, score := bestLinuxThemeIcon(root, names)
		if path == "" {
			continue
		}
		if score >= 50 {
			return path
		}
		if fallback == "" {
			fallback = path
		}
	}
	if fallback != "" {
		return fallback
	}
	for _, root := range linuxIconDataRoots() {
		for _, name := range names {
			if path := existingLinuxIcon(filepath.Join(root, "pixmaps"), name); path != "" {
				return path
			}
			if path := existingLinuxIcon(filepath.Join(root, "icons"), name); path != "" {
				return path
			}
		}
	}
	return ""
}

func resolveLinuxAbsoluteIcon(iconValue string) string {
	if !filepath.IsAbs(iconValue) {
		return ""
	}
	if linuxIsFile(iconValue) {
		return iconValue
	}
	if filepath.Ext(iconValue) != "" {
		return ""
	}
	for _, ext := range []string{".png", ".svg"} {
		if linuxIsFile(iconValue + ext) {
			return iconValue + ext
		}
	}
	return ""
}

func linuxIconNames(iconValue string) []string {
	names := []string{iconValue}
	if ext := filepath.Ext(iconValue); ext != "" {
		names = append(names, strings.TrimSuffix(iconValue, ext))
	}
	return util.UniqueStrings(names)
}

func bestLinuxThemeIcon(root string, names []string) (string, int) {
	sizes := []struct {
		dir   string
		score int
	}{
		{"256x256", 100},
		{"128x128", 95},
		{"512x512", 90},
		{"scalable", 85},
		{"96x96", 70},
		{"64x64", 60},
		{"48x48", 50},
		{"32x32", 30},
		{"24x24", 20},
		{"22x22", 18},
		{"16x16", 10},
	}
	for _, size := range sizes {
		for _, name := range names {
			if path := existingLinuxIcon(filepath.Join(root, size.dir, "apps"), name); path != "" {
				return path, size.score
			}
		}
	}
	return "", 0
}

func existingLinuxIcon(dir string, name string) string {
	if filepath.Ext(name) != "" && linuxIsFile(filepath.Join(dir, name)) {
		return filepath.Join(dir, name)
	}
	for _, ext := range []string{".png", ".svg"} {
		path := filepath.Join(dir, name+ext)
		if linuxIsFile(path) {
			return path
		}
	}
	return ""
}

func linuxIconThemeRoots() []string {
	parents := linuxIconParents()
	roots := []string{}
	seen := map[string]struct{}{}
	add := func(root string) {
		root = filepath.Clean(strings.TrimSpace(root))
		if root == "" || root == "." {
			return
		}
		if _, ok := seen[root]; ok {
			return
		}
		if !linuxIsDir(root) {
			return
		}
		seen[root] = struct{}{}
		roots = append(roots, root)
	}
	for _, theme := range linuxPreferredIconThemes() {
		for _, parent := range parents {
			themeRoot := filepath.Join(parent, theme)
			add(themeRoot)
			for _, inherited := range linuxIconThemeInherits(themeRoot) {
				for _, inheritedParent := range parents {
					add(filepath.Join(inheritedParent, inherited))
				}
			}
		}
	}
	for _, parent := range parents {
		add(filepath.Join(parent, "hicolor"))
	}
	return roots
}

func linuxPreferredIconThemes() []string {
	home, _ := os.UserHomeDir()
	if home == "" {
		return nil
	}
	themes := []string{}
	for _, configPath := range []string{
		filepath.Join(home, ".config", "gtk-4.0", "settings.ini"),
		filepath.Join(home, ".config", "gtk-3.0", "settings.ini"),
	} {
		if value := readLinuxINIValue(configPath, "Settings", "gtk-icon-theme-name"); value != "" {
			themes = append(themes, value)
		}
	}
	if value := readLinuxINIValue(filepath.Join(home, ".config", "kdeglobals"), "Icons", "Theme"); value != "" {
		themes = append(themes, value)
	}
	return util.UniqueStrings(themes)
}

func linuxIconThemeInherits(themeRoot string) []string {
	raw := readLinuxINIValue(filepath.Join(themeRoot, "index.theme"), "Icon Theme", "Inherits")
	if raw == "" {
		return nil
	}
	names := []string{}
	for _, name := range strings.Split(raw, ",") {
		name = strings.TrimSpace(name)
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func readLinuxINIValue(path string, section string, key string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()

	current := ""
	prefix := key + "="
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			current = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		if section != "" && current != section {
			continue
		}
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

func linuxIconParents() []string {
	parents := []string{}
	for _, root := range linuxIconDataRoots() {
		parents = append(parents, filepath.Join(root, "icons"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		parents = append(parents, filepath.Join(home, ".icons"))
	}
	return util.UniqueStrings(parents)
}

func linuxIconDataRoots() []string {
	home, _ := os.UserHomeDir()
	dataHome := strings.TrimSpace(os.Getenv("XDG_DATA_HOME"))
	if dataHome == "" && home != "" {
		dataHome = filepath.Join(home, ".local", "share")
	}
	dataDirs := strings.Split(os.Getenv("XDG_DATA_DIRS"), ":")
	if len(dataDirs) == 1 && strings.TrimSpace(dataDirs[0]) == "" {
		dataDirs = []string{"/usr/local/share", "/usr/share"}
	}
	roots := []string{}
	if dataHome != "" {
		roots = append(roots, dataHome)
	}
	for _, dir := range dataDirs {
		dir = strings.TrimSpace(dir)
		if dir != "" {
			roots = append(roots, dir)
		}
	}
	return util.UniqueStrings(roots)
}

func loadLinuxAppIcon(path string) (image.Image, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(filepath.Ext(path), ".svg") {
		return svg.Render(string(data), 128, 128)
	}
	decoded, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	return decoded, nil
}

func linuxIsFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func linuxIsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
