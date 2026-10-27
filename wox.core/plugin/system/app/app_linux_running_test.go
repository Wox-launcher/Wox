//go:build linux

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLinuxCgroupAppID(t *testing.T) {
	id, leader := linuxCgroupAppID("0::/user.slice/user-1000.slice/user@1000.service/app.slice/app-com.google.Chrome-1539497.scope\n", 1539497)
	if id != "com.google.Chrome" || !leader {
		t.Fatalf("chrome scope = %q leader=%t", id, leader)
	}

	id, leader = linuxCgroupAppID("0::/user.slice/user-1000.slice/app.slice/app-Hyprland-xdg\\x2dterminal\\x2dexec-8a5e96ad.scope", 11257)
	if id != "Hyprland-xdg-terminal-exec" || leader {
		t.Fatalf("terminal scope = %q leader=%t", id, leader)
	}
}

func TestLinuxProcCPUTicks(t *testing.T) {
	_, ticks, ok := linuxProcCPUTicks("1539497 (chrome) S 1156 1 1 0 -1 4194304 1 2 3 4 10 15 0 0")
	if !ok || ticks != 25 {
		t.Fatalf("ticks = %d ok=%t", ticks, ok)
	}
	_, ticks, ok = linuxProcCPUTicks("12 (my app (beta)) R 1 1 1 0 -1 0 0 0 0 0 4 6")
	if !ok || ticks != 10 {
		t.Fatalf("spaced comm ticks = %d ok=%t", ticks, ok)
	}
}

func TestLinuxPrivateBytes(t *testing.T) {
	if got := linuxPrivateBytes("100 10 4 0 0 0 0", 4096); got != 6*4096 {
		t.Fatalf("private = %d", got)
	}
	if got := linuxPrivateBytes("100 3 9 0 0 0 0", 4096); got != 0 {
		t.Fatalf("shared larger than resident = %d", got)
	}
}

func TestLinuxCPUPercent(t *testing.T) {
	if got := linuxCPUPercent(0, 100, 0, 1000, 1); got != 100 {
		t.Fatalf("one core = %v", got)
	}
	if got := linuxCPUPercent(0, 100, 0, 1000, 16); got != 6.25 {
		t.Fatalf("sixteen cores = %v", got)
	}
	if got := linuxCPUPercent(50, 40, 0, 1000, 4); got != 0 {
		t.Fatalf("restarted process = %v", got)
	}
}

func TestMatchLinuxAppProcessesChromeFamily(t *testing.T) {
	catalog := []linuxDesktopRef{
		{ID: "google-chrome", WMClass: "google-chrome", ExecBase: "google-chrome-stable"},
		{ID: "com.google.Chrome", WMClass: "google-chrome", ExecBase: "google-chrome-stable"},
		{ID: "com.microsoft.VSCode", WMClass: "com.microsoft.VSCode", ExecBase: "code"},
	}
	procs := []linuxProc{
		{Pid: 1539497, Exe: "/opt/google/chrome/chrome", ExeBase: "chrome", Comm: "chrome", CgroupID: "com.google.Chrome", Leader: true, Private: 100, CPUTicks: 10},
		{Pid: 200, Exe: "/opt/google/chrome/chrome", ExeBase: "chrome", Comm: "chrome", CgroupID: "com.microsoft.VSCode", Private: 50, CPUTicks: 5},
		{Pid: 9144, Exe: "/usr/share/code/code", ExeBase: "code", Comm: "code", CgroupID: "com.microsoft.VSCode", Leader: true, Private: 80, CPUTicks: 8},
		{Pid: 11257, Exe: "/usr/bin/foot", ExeBase: "foot", Comm: "foot", CgroupID: "Hyprland-xdg-terminal-exec", Private: 20, CPUTicks: 2},
	}

	chrome := matchLinuxAppProcesses(procs, linuxAppKeysFor(catalog[0], catalog), nil)
	if linuxRepresentativePid(chrome, linuxAppKeysFor(catalog[0], catalog)) != 1539497 || len(chrome) != 2 {
		t.Fatalf("chrome = %+v", chrome)
	}
	vscode := matchLinuxAppProcesses(procs, linuxAppKeysFor(catalog[2], catalog), nil)
	if len(vscode) != 1 || vscode[0].Pid != 9144 {
		t.Fatalf("vscode = %+v", vscode)
	}
	foot := matchLinuxAppProcesses(procs, linuxAppKeysFor(linuxDesktopRef{ID: "foot", ExecBase: "foot"}, catalog), nil)
	if len(foot) != 1 || foot[0].Pid != 11257 {
		t.Fatalf("foot = %+v", foot)
	}
}

func TestMatchLinuxAppProcessesUsesWindowClass(t *testing.T) {
	keys := linuxAppKeysFor(linuxDesktopRef{ID: "google-chrome", WMClass: "google-chrome", ExecBase: "google-chrome-stable"}, nil)
	procs := []linuxProc{
		{Pid: 7, Exe: "/opt/google/chrome/chrome", ExeBase: "chrome", Comm: "chrome", Private: 10},
		{Pid: 8, Exe: "/opt/google/chrome/chrome", ExeBase: "chrome", Comm: "chrome", Private: 4},
		{Pid: 9, Exe: "/usr/bin/foot", ExeBase: "foot", Comm: "foot"},
	}
	matched := matchLinuxAppProcesses(procs, keys, []linuxAppWindow{{Pid: 7, Class: "google-chrome"}})
	if len(matched) != 2 {
		t.Fatalf("matched = %+v", matched)
	}
}

func TestLinuxDesktopCatalogSibling(t *testing.T) {
	root := t.TempDir()
	writeDesktop := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeDesktop("google-chrome.desktop", "[Desktop Entry]\nType=Application\nName=Google Chrome\nExec=/usr/bin/google-chrome-stable %U\nStartupWMClass=google-chrome\n")
	writeDesktop("com.google.Chrome.desktop", "[Desktop Entry]\nType=Application\nNoDisplay=true\nName=Google Chrome\nExec=/usr/bin/google-chrome-stable %U\nStartupWMClass=google-chrome\n")
	writeDesktop("notes.desktop", "[Desktop Entry]\nType=Application\nHidden=true\nName=Notes\nExec=notes\n")

	refs := scanLinuxDesktopCatalog([]appDirectory{{Path: root, Recursive: true}})
	if len(refs) != 2 {
		t.Fatalf("refs = %+v", refs)
	}
	keys := linuxAppKeysFor(refs[0], refs)
	if !linuxKeyMatch("com.google.Chrome", keys.CgroupIDs) || !linuxKeyMatch("google-chrome-stable", keys.Commands) {
		t.Fatalf("keys = %+v", keys)
	}
}

func TestLinuxChromeRunningStat(t *testing.T) {
	desktop := "/usr/share/applications/google-chrome.desktop"
	if _, err := os.Stat(desktop); err != nil {
		t.Skip(err)
	}
	retriever := &LinuxRetriever{}
	info := appInfo{Name: "Google Chrome", Path: desktop, Type: AppTypeDesktop}
	pid := retriever.GetPid(context.Background(), info)
	if pid <= 0 {
		for _, proc := range scanLinuxProcesses() {
			if proc.Exe == "/opt/google/chrome/chrome" || proc.CgroupID == "com.google.Chrome" {
				t.Fatalf("chrome process pid %d exe %s cgroup %s was not matched", proc.Pid, proc.Exe, proc.CgroupID)
			}
		}
		t.Skip("Chrome is not running")
	}
	leader := 0
	for _, proc := range scanLinuxProcesses() {
		if proc.Leader && strings.EqualFold(proc.CgroupID, "com.google.Chrome") {
			leader = proc.Pid
			break
		}
	}
	if leader > 0 && pid != leader {
		t.Fatalf("representative pid %d, chrome scope leader %d", pid, leader)
	}
	stat, err := retriever.GetProcessStat(context.Background(), info)
	if err != nil {
		t.Fatal(err)
	}
	if stat.Memory < 50*1024*1024 || stat.Memory > 64*1024*1024*1024 {
		t.Fatalf("memory = %.1f MiB, pid %d", stat.Memory/1024/1024, pid)
	}
	// The first sample has no previous tick, so CPU stays 0. The percent formula is covered separately.
	if stat.CPU != 0 {
		t.Fatalf("first cpu sample = %.1f%%", stat.CPU)
	}
	t.Logf("chrome pid=%d memory=%.1f MiB", pid, stat.Memory/1024/1024)
}
