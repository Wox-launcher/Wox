package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"wox/resource/licenses"
)

const isWindows = runtime.GOOS == "windows"

// platformLicenses collects Linux protocol notices and the AppImage's bundled layer-shell library.
func platformLicenses(release bool) ([]licenses.Entry, error) {
	if runtime.GOOS != "linux" {
		return nil, nil
	}
	const native = "ui/runtime/internal/window/"
	var entries []licenses.Entry
	for _, protocol := range []struct{ name, file string }{
		{"ext-background-effect-v1", native + "ext-background-effect-v1-protocol.c"},
		{"relative-pointer-unstable-v1", native + "relative-pointer-unstable-v1-protocol.c"},
		{"ext-data-control-v1", "util/clipboard/clipboard_ext_data_control_v1_protocol.c"},
	} {
		source, err := os.ReadFile(protocol.file)
		if err != nil {
			return nil, err
		}
		_, after, _ := strings.Cut(strings.ReplaceAll(string(source), "\r\n", "\n"), "/*\n")
		notice, _, _ := strings.Cut(after, " */")
		if notice == "" {
			return nil, fmt.Errorf("missing Wayland copyright notice for %s", protocol.name)
		}
		lines := strings.Split(notice, "\n")
		for i, line := range lines {
			lines[i] = strings.TrimPrefix(strings.TrimPrefix(line, " *"), " ")
		}
		entries = append(entries, licenses.Entry{Name: "Wayland " + protocol.name, Text: strings.Join(lines, "\n")})
	}
	// The AppImage copies the system library on the Debian-family release builder.
	version, err := exec.Command("dpkg-query", "-W", "-f=${Version}", "libgtk-layer-shell0").Output()
	if err == nil && strings.TrimSpace(string(version)) != "" {
		notice, err := os.ReadFile("/usr/share/doc/libgtk-layer-shell0/copyright")
		if err != nil {
			return nil, err
		}
		text := string(notice)
		const common = "/usr/share/common-licenses/"
		// The LGPL text supplements GPL v3, so readers need both full texts offline.
		for _, name := range []string{"LGPL-3", "GPL-3"} {
			body, err := os.ReadFile(common + name)
			if err != nil {
				return nil, err
			}
			text += "\n\n" + string(body)
		}
		entries = append(entries, licenses.Entry{Name: "GTK Layer Shell (AppImage)", Version: strings.TrimSpace(string(version)), Text: text})
	} else if release {
		return nil, fmt.Errorf("cannot find installed libgtk-layer-shell0 for the AppImage license catalog")
	}
	return entries, nil
}
