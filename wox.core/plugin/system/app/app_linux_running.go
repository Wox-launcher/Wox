package app

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"wox/util"
	"wox/util/window"
)

const (
	// Shorter than the 1s UI refresh. GetPid and GetProcessStat in one tick share
	// this snapshot, and the next tick still reads a new /proc sample for CPU.
	linuxRunningRefresh      = 750 * time.Millisecond
	linuxDesktopCatalogTTL   = 30 * time.Second
	linuxClockTicksPerSecond = 100 // Linux user HZ. /proc stat times use this, not the hardware timer.
)

// linuxCPUSample is the previous summed CPU time for one desktop entry.
type linuxCPUSample struct {
	ticks     int64
	timestamp int64
}

// linuxProc is one process visible in /proc.
type linuxProc struct {
	Pid      int
	Exe      string
	ExeBase  string
	Comm     string
	CgroupID string
	Leader   bool
	CPUTicks int64
	Private  int64
}

// linuxDesktopRef is the identity slice used to attach processes to a launcher.
type linuxDesktopRef struct {
	ID       string
	WMClass  string
	ExecBase string
}

type linuxDesktopCatalog struct {
	at   time.Time
	refs []linuxDesktopRef
}

type linuxRunningSnapshot struct {
	at      time.Time
	procs   []linuxProc
	windows []linuxAppWindow
}

// linuxAppWindow is the process-matcher view of a compositor window.
type linuxAppWindow struct {
	Pid   int
	Class string
	AltID string
}

// linuxAppKeys are the ids that identify one launcher's processes.
// Commands match comm and the executable name. CgroupIDs match the systemd
// app scope, including a NoDisplay sibling such as com.google.Chrome.desktop
// for the visible google-chrome.desktop entry.
type linuxAppKeys struct {
	Commands  []string
	CgroupIDs []string
}

// GetPid returns a process that represents this running desktop entry.
func (a *LinuxRetriever) GetPid(ctx context.Context, app appInfo) int {
	matched := a.matchedProcesses(ctx, app)
	if len(matched.procs) == 0 {
		return 0
	}
	return linuxRepresentativePid(matched.procs, matched.keys)
}

// GetProcessStat sums CPU and private memory across every process that belongs
// to the desktop entry. Chrome's renderers are separate processes that share
// one executable, so a single pid would miss most of the application.
func (a *LinuxRetriever) GetProcessStat(ctx context.Context, app appInfo) (*ProcessStat, error) {
	matched := a.matchedProcesses(ctx, app)
	if len(matched.procs) == 0 {
		return nil, fmt.Errorf("app %s is not running", app.Name)
	}

	var ticks int64
	var private int64
	for _, proc := range matched.procs {
		ticks += proc.CPUTicks
		private += proc.Private
	}

	sampleKey := strings.ToLower(filepath.Clean(app.Path))
	now := util.GetSystemTimestamp()
	cpu := 0.0
	samples := a.cpuSampleMap()
	if previous, ok := samples.Load(sampleKey); ok && now > previous.timestamp && ticks >= previous.ticks {
		cpu = linuxCPUPercent(previous.ticks, ticks, previous.timestamp, now, runtime.NumCPU())
	}
	samples.Store(sampleKey, linuxCPUSample{ticks: ticks, timestamp: now})

	return &ProcessStat{CPU: cpu, Memory: float64(private)}, nil
}

type linuxMatchedProcesses struct {
	procs []linuxProc
	keys  linuxAppKeys
}

func (a *LinuxRetriever) matchedProcesses(ctx context.Context, app appInfo) linuxMatchedProcesses {
	if !strings.HasSuffix(strings.ToLower(strings.TrimSpace(app.Path)), ".desktop") {
		return linuxMatchedProcesses{}
	}
	target, ok := readLinuxDesktopRef(app.Path)
	if !ok {
		return linuxMatchedProcesses{}
	}
	snap, catalog := a.runningState(ctx)
	keys := linuxAppKeysFor(target, catalog)
	return linuxMatchedProcesses{
		procs: matchLinuxAppProcesses(snap.procs, keys, snap.windows),
		keys:  keys,
	}
}

func (a *LinuxRetriever) runningState(ctx context.Context) (linuxRunningSnapshot, []linuxDesktopRef) {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()

	now := time.Now()
	if a.running.procs == nil || now.Sub(a.running.at) >= linuxRunningRefresh {
		a.running = linuxRunningSnapshot{
			at:      now,
			procs:   scanLinuxProcesses(),
			windows: linuxWindowsFromCompositor(window.ListLinuxAppWindows(ctx)),
		}
	}
	if a.runningDesktops.refs == nil || now.Sub(a.runningDesktops.at) >= linuxDesktopCatalogTTL {
		a.runningDesktops = linuxDesktopCatalog{at: now, refs: scanLinuxDesktopCatalog(a.GetAppDirectories(ctx))}
	}
	return a.running, a.runningDesktops.refs
}

func (a *LinuxRetriever) cpuSampleMap() *util.HashMap[string, linuxCPUSample] {
	a.runningMu.Lock()
	defer a.runningMu.Unlock()
	if a.cpuSamples == nil {
		a.cpuSamples = util.NewHashMap[string, linuxCPUSample]()
	}
	return a.cpuSamples
}

func linuxWindowsFromCompositor(windows []window.LinuxAppWindow) []linuxAppWindow {
	converted := make([]linuxAppWindow, 0, len(windows))
	for _, item := range windows {
		if item.Pid <= 0 {
			continue
		}
		converted = append(converted, linuxAppWindow{Pid: item.Pid, Class: item.Class, AltID: item.AltID})
	}
	return converted
}

// linuxCPUPercent matches the Windows app tail: percent of the whole machine,
// so one busy core on an 8-core CPU is 12.5 rather than 100.
func linuxCPUPercent(previousTicks, ticks, previousMs, nowMs int64, cores int) float64 {
	if cores < 1 || nowMs <= previousMs || ticks < previousTicks {
		return 0
	}
	elapsedMs := float64(nowMs - previousMs)
	cpuMs := float64(ticks-previousTicks) / linuxClockTicksPerSecond * 1000
	return cpuMs / elapsedMs * 100 / float64(cores)
}

// linuxRepresentativePid prefers the systemd scope leader. That is the process
// the desktop entry started; helper processes share its executable.
func linuxRepresentativePid(procs []linuxProc, keys linuxAppKeys) int {
	leader := 0
	lowest := 0
	for _, proc := range procs {
		if lowest == 0 || proc.Pid < lowest {
			lowest = proc.Pid
		}
		if proc.Leader && linuxKeyMatch(proc.CgroupID, keys.CgroupIDs) && (leader == 0 || proc.Pid < leader) {
			leader = proc.Pid
		}
	}
	if leader > 0 {
		return leader
	}
	return lowest
}

func linuxAppKeysFor(target linuxDesktopRef, catalog []linuxDesktopRef) linuxAppKeys {
	keys := linuxAppKeys{}
	addCommand := func(value string) {
		if linuxGenericCommand(value) {
			return
		}
		keys.Commands = appendLinuxKey(keys.Commands, value)
	}
	addCgroup := func(value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		keys.CgroupIDs = appendLinuxKey(keys.CgroupIDs, value)
	}
	addCommand(target.ID)
	addCommand(target.WMClass)
	addCommand(target.ExecBase)
	addCgroup(target.ID)
	addCgroup(target.WMClass)

	for _, other := range catalog {
		if strings.EqualFold(other.ID, target.ID) {
			continue
		}
		if !linuxDesktopSibling(target, other) {
			continue
		}
		addCgroup(other.ID)
		addCgroup(other.WMClass)
	}
	return keys
}

// linuxDesktopSibling reports a second launcher for the same program.
// com.google.Chrome.desktop is NoDisplay, but systemd still names Chrome's
// scope with that id while the searchable entry is google-chrome.desktop.
func linuxDesktopSibling(target, other linuxDesktopRef) bool {
	if target.ExecBase != "" && !linuxGenericCommand(target.ExecBase) && strings.EqualFold(other.ExecBase, target.ExecBase) {
		return true
	}
	if target.WMClass == "" {
		return false
	}
	return strings.EqualFold(other.WMClass, target.WMClass) || strings.EqualFold(other.ID, target.WMClass) || strings.EqualFold(other.WMClass, target.ID)
}

func appendLinuxKey(keys []string, value string) []string {
	value = strings.TrimSpace(value)
	if value == "" {
		return keys
	}
	for _, existing := range keys {
		if strings.EqualFold(existing, value) {
			return keys
		}
	}
	return append(keys, value)
}

func linuxKeyMatch(value string, keys []string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, key := range keys {
		if strings.EqualFold(value, key) {
			return true
		}
	}
	return false
}

// matchLinuxAppProcesses selects the executable family of this launcher.
// Sharing a cgroup is not enough: Chrome renderers can sit inside another
// app's scope. A confident anchor (command name, scope leader, or window)
// contributes its executable path, and every process using that path counts.
func matchLinuxAppProcesses(procs []linuxProc, keys linuxAppKeys, windows []linuxAppWindow) []linuxProc {
	anchors := make([]linuxProc, 0)
	for _, proc := range procs {
		if linuxProcessAnchor(proc, keys, windows) {
			anchors = append(anchors, proc)
		}
	}
	if len(anchors) == 0 {
		return nil
	}
	exes := map[string]struct{}{}
	for _, proc := range anchors {
		if linuxExpandableExe(proc.Exe, proc.ExeBase) {
			exes[proc.Exe] = struct{}{}
		}
	}
	if len(exes) == 0 {
		return anchors
	}
	matched := make([]linuxProc, 0)
	for _, proc := range procs {
		if _, ok := exes[proc.Exe]; ok {
			matched = append(matched, proc)
		}
	}
	return matched
}

func linuxProcessAnchor(proc linuxProc, keys linuxAppKeys, windows []linuxAppWindow) bool {
	if linuxKeyMatch(proc.Comm, keys.Commands) || linuxKeyMatch(proc.ExeBase, keys.Commands) {
		return true
	}
	if proc.Leader && linuxKeyMatch(proc.CgroupID, keys.CgroupIDs) {
		return true
	}
	for _, item := range windows {
		if item.Pid != proc.Pid {
			continue
		}
		if linuxKeyMatch(item.Class, keys.Commands) || linuxKeyMatch(item.Class, keys.CgroupIDs) || linuxKeyMatch(item.AltID, keys.Commands) || linuxKeyMatch(item.AltID, keys.CgroupIDs) {
			return true
		}
	}
	return false
}

func linuxExpandableExe(exe, base string) bool {
	if exe == "" || !filepath.IsAbs(exe) || linuxGenericCommand(base) {
		return false
	}
	return true
}

func linuxGenericCommand(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "", "sh", "bash", "dash", "zsh", "fish", "env", "gio", "gtk-launch", "gapplication", "bwrap", "flatpak", "snap", "systemd-run", "python", "python2", "python3", "perl", "ruby", "node":
		return true
	default:
		return false
	}
}

func scanLinuxProcesses() []linuxProc {
	entries, err := os.ReadDir("/proc")
	if err != nil {
		return nil
	}
	pageSize := int64(os.Getpagesize())
	procs := make([]linuxProc, 0, len(entries))
	for _, entry := range entries {
		pid, err := strconv.Atoi(entry.Name())
		if err != nil || pid <= 0 {
			continue
		}
		proc, ok := readLinuxProc(pid, pageSize)
		if !ok {
			continue
		}
		procs = append(procs, proc)
	}
	return procs
}

func readLinuxProc(pid int, pageSize int64) (linuxProc, bool) {
	base := filepath.Join("/proc", strconv.Itoa(pid))
	commBytes, err := os.ReadFile(filepath.Join(base, "comm"))
	if err != nil {
		return linuxProc{}, false
	}
	statBytes, err := os.ReadFile(filepath.Join(base, "stat"))
	if err != nil {
		return linuxProc{}, false
	}
	_, ticks, ok := linuxProcCPUTicks(string(statBytes))
	if !ok {
		return linuxProc{}, false
	}
	exe, _ := os.Readlink(filepath.Join(base, "exe"))
	exe = strings.TrimSuffix(exe, " (deleted)")
	cgroupBytes, _ := os.ReadFile(filepath.Join(base, "cgroup"))
	cgroupID, leader := linuxCgroupAppID(string(cgroupBytes), pid)
	private := int64(0)
	if statm, err := os.ReadFile(filepath.Join(base, "statm")); err == nil {
		private = linuxPrivateBytes(string(statm), pageSize)
	}
	return linuxProc{
		Pid:      pid,
		Exe:      exe,
		ExeBase:  linuxProcBase(exe),
		Comm:     strings.TrimSpace(string(commBytes)),
		CgroupID: cgroupID,
		Leader:   leader,
		CPUTicks: ticks,
		Private:  private,
	}, true
}

func linuxProcBase(exe string) string {
	base := filepath.Base(strings.TrimSpace(exe))
	if base == "." || base == string(filepath.Separator) {
		return ""
	}
	return base
}

// linuxProcCPUTicks reads utime+stime from /proc/pid/stat.
// The command field is wrapped in parentheses and may itself contain spaces
// and parentheses, so the numeric fields start after the last ')'.
func linuxProcCPUTicks(stat string) (ppid int, ticks int64, ok bool) {
	end := strings.LastIndex(stat, ")")
	if end < 0 || end+2 >= len(stat) {
		return 0, 0, false
	}
	fields := strings.Fields(stat[end+2:])
	if len(fields) < 13 {
		return 0, 0, false
	}
	ppid, err := strconv.Atoi(fields[1])
	if err != nil {
		return 0, 0, false
	}
	utime, err := strconv.ParseInt(fields[11], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	stime, err := strconv.ParseInt(fields[12], 10, 64)
	if err != nil {
		return 0, 0, false
	}
	return ppid, utime + stime, true
}

// linuxPrivateBytes is resident memory minus shared pages, the same split
// GNOME System Monitor uses for one process. Summing it avoids counting a
// shared library again for every renderer.
func linuxPrivateBytes(statm string, pageSize int64) int64 {
	fields := strings.Fields(statm)
	if len(fields) < 3 || pageSize <= 0 {
		return 0
	}
	resident, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil || resident < 0 {
		return 0
	}
	shared, err := strconv.ParseInt(fields[2], 10, 64)
	if err != nil || shared < 0 {
		return 0
	}
	if shared > resident {
		shared = resident
	}
	return (resident - shared) * pageSize
}

// linuxCgroupAppID reads the desktop id from an application scope such as
// app-com.google.Chrome-1539497.scope. Leader is true when this pid is the
// scope's main process, not a helper that happens to share the cgroup.
func linuxCgroupAppID(cgroup string, pid int) (string, bool) {
	var appID string
	leader := false
	for _, line := range strings.Split(cgroup, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if _, path, ok := strings.Cut(line, ":"); ok {
			line = path
		}
		unit := linuxUnescapeSystemd(filepath.Base(line))
		if !strings.HasPrefix(unit, "app-") || !strings.HasSuffix(unit, ".scope") {
			continue
		}
		body := strings.TrimSuffix(strings.TrimPrefix(unit, "app-"), ".scope")
		// The desktop id can contain dashes, so the pid or token is the last field.
		cut := strings.LastIndex(body, "-")
		if cut <= 0 || cut == len(body)-1 {
			continue
		}
		id := body[:cut]
		suffix := body[cut+1:]
		if id == "" {
			continue
		}
		appID = id
		if suffix == strconv.Itoa(pid) {
			leader = true
		}
	}
	return appID, leader
}

// linuxUnescapeSystemd decodes systemd's \xHH unit escapes, such as \x2d for '-'.
func linuxUnescapeSystemd(value string) string {
	var b strings.Builder
	for index := 0; index < len(value); index++ {
		if value[index] == '\\' && index+3 < len(value) && (value[index+1] == 'x' || value[index+1] == 'X') {
			decoded, err := strconv.ParseUint(value[index+2:index+4], 16, 8)
			if err == nil {
				b.WriteByte(byte(decoded))
				index += 3
				continue
			}
		}
		b.WriteByte(value[index])
	}
	return b.String()
}

func readLinuxDesktopRef(path string) (linuxDesktopRef, bool) {
	refs := parseLinuxDesktopCatalog(path)
	if len(refs) != 1 {
		return linuxDesktopRef{}, false
	}
	return refs[0], true
}

func scanLinuxDesktopCatalog(directories []appDirectory) []linuxDesktopRef {
	refs := []linuxDesktopRef{}
	seen := map[string]struct{}{}
	for _, dir := range directories {
		root := filepath.Clean(strings.TrimSpace(dir.Path))
		if root == "" || root == "." {
			continue
		}
		_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry == nil {
				return nil
			}
			if entry.IsDir() {
				if path != root && !dir.Recursive {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(strings.ToLower(entry.Name()), ".desktop") {
				return nil
			}
			if _, ok := seen[path]; ok {
				return nil
			}
			seen[path] = struct{}{}
			refs = append(refs, parseLinuxDesktopCatalog(path)...)
			return nil
		})
	}
	return refs
}

func parseLinuxDesktopCatalog(path string) []linuxDesktopRef {
	file, err := os.Open(path)
	if err != nil {
		return nil
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
			if inEntry {
				break
			}
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
	if parseLinuxDesktopBool(values["Hidden"]) {
		return nil
	}
	if entryType := strings.TrimSpace(values["Type"]); entryType != "" && !strings.EqualFold(entryType, "Application") {
		return nil
	}
	base := filepath.Base(path)
	if !strings.HasSuffix(strings.ToLower(base), ".desktop") {
		return nil
	}
	return []linuxDesktopRef{{
		ID:       base[:len(base)-len(".desktop")],
		WMClass:  strings.TrimSpace(values["StartupWMClass"]),
		ExecBase: linuxDesktopExecBase(values["Exec"]),
	}}
}

// linuxDesktopExecBase returns the program name from an Exec line.
// Flatpak's --command= is the real program. env assignments and field codes are skipped.
func linuxDesktopExecBase(line string) string {
	fields := strings.Fields(line)
	command := ""
	for _, field := range fields {
		if rest, ok := strings.CutPrefix(field, "--command="); ok {
			command = rest
			break
		}
	}
	if command == "" {
		for _, field := range fields {
			if field == "env" || strings.HasPrefix(field, "%") || linuxDesktopEnvAssignment(field) {
				continue
			}
			command = field
			break
		}
	}
	base := filepath.Base(strings.TrimSpace(command))
	if base == "." || base == string(filepath.Separator) || linuxGenericCommand(base) {
		return ""
	}
	return base
}

func linuxDesktopEnvAssignment(field string) bool {
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
