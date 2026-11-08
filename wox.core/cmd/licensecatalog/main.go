// Command licensecatalog inventories dependencies shipped by the native Wox build.
package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"wox/resource/licenses"
)

//go:embed notices
var upstream embed.FS

const hosts = "resource/hosts"

func main() {
	tags := flag.String("tags", "sqlite_fts5 wox_ghostty", "Go build tags")
	release := flag.Bool("release", false, "Require all bundled hosts and their license inputs")
	flag.Parse()
	if err := generate(*tags, *release); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func generate(tags string, release bool) error {
	entries, err := goLicenses(tags)
	if err != nil {
		return err
	}
	if slices.Contains(strings.FieldsFunc(tags, func(r rune) bool { return r == ' ' || r == ',' }), "wox_ghostty") {
		output, err := exec.Command("go", "run", "./cmd/ghosttybuild", "-notices-path").Output()
		if err != nil {
			return fmt.Errorf("locate Ghostty notices: %w", err)
		}
		ghostty := strings.TrimSpace(string(output))
		body, err := os.ReadFile(ghostty)
		if err != nil {
			return err
		}
		version := strings.TrimPrefix(filepath.Base(filepath.Dir(ghostty)), "ghostty-")
		version = strings.TrimSuffix(version, "-"+runtime.GOOS+"-"+runtime.GOARCH)
		entries = append(entries, licenses.Entry{Name: "libghostty-vt", Version: version, Text: string(body)})
	}
	for _, collect := range []func(bool) ([]licenses.Entry, error){nodeLicenses, pythonLicenses, raycastLicenses, flowLicenses, platformLicenses} {
		group, err := collect(release)
		if err != nil {
			return err
		}
		entries = append(entries, group...)
	}
	slices.SortFunc(entries, func(a, b licenses.Entry) int {
		if order := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); order != 0 {
			return order
		}
		return strings.Compare(a.Version, b.Version)
	})
	var encoded bytes.Buffer
	encoder := json.NewEncoder(&encoded)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(entries); err != nil {
		return err
	}
	return os.WriteFile("resource/licenses/catalog/"+runtime.GOOS+"-"+runtime.GOARCH+".json", encoded.Bytes(), 0644)
}

type module struct {
	Path    string
	Version string
	Dir     string
	Main    bool
	Replace *module
}

// goLicenses follows the platform's build graph, including the separately built Windows helpers.
func goLicenses(tags string) ([]licenses.Entry, error) {
	modules := make(map[string]module)
	if err := collectModules(modules, "1", runtime.GOARCH, "-tags", tags, "."); err != nil {
		return nil, err
	}
	if runtime.GOOS == "windows" {
		if err := collectModules(modules, "0", runtime.GOARCH, "./cmd/file_index_service"); err != nil {
			return nil, err
		}
		if err := collectModules(modules, "0", "386", "./cmd/window_hook_helper"); err != nil {
			return nil, err
		}
	}
	goRoot := runtime.GOROOT()
	body, err := directoryNotices(goRoot)
	if err != nil && runtime.GOOS == "linux" {
		// Debian-family toolchains keep the Go source notices in the package documentation.
		distroPackage := strings.Replace(filepath.Base(goRoot), "go-", "golang-", 1) + "-src"
		doc := filepath.Join("/usr/share/doc", distroPackage, "copyright")
		if notice, readErr := os.ReadFile(doc); readErr == nil {
			body, err = string(notice), nil
		}
	}
	if err != nil {
		return nil, err
	}
	entries := []licenses.Entry{{Name: "Go", Version: runtime.Version(), Text: body}}
	for _, mod := range modules {
		body, err := directoryNotices(mod.Dir)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", mod.Path, mod.Version, err)
		}
		entries = append(entries, licenses.Entry{Name: mod.Path, Version: mod.Version, Text: body})
		if mod.Path == "github.com/mattn/go-sqlite3" {
			header, err := os.ReadFile(filepath.Join(mod.Dir, "sqlite3-binding.h"))
			if err != nil {
				return nil, err
			}
			_, after, _ := strings.Cut(string(header), "#define SQLITE_VERSION ")
			version, _, _ := strings.Cut(strings.TrimSpace(after), "\n")
			entries = append(entries, licenses.Entry{Name: "SQLite", Version: strings.Trim(strings.TrimSpace(version), "\""), Text: "SQLite is dedicated to the public domain.\n\nhttps://www.sqlite.org/copyright.html"})
		}
	}
	return entries, nil
}

// collectModules deduplicates packages from the same module while honoring replacements.
func collectModules(modules map[string]module, cgo, arch string, args ...string) error {
	cmd := exec.Command("go", append([]string{"list", "-deps", "-json=Module"}, args...)...)
	cmd.Env = append(os.Environ(), "CGO_ENABLED="+cgo, "GOOS="+runtime.GOOS, "GOARCH="+arch)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	decoder := json.NewDecoder(bytes.NewReader(out))
	for {
		var pkg struct{ Module *module }
		if err := decoder.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if pkg.Module == nil || pkg.Module.Main {
			continue
		}
		mod := *pkg.Module
		if mod.Replace != nil {
			mod.Dir = mod.Replace.Dir
			if mod.Replace.Version != "" {
				mod.Path, mod.Version = mod.Replace.Path, mod.Replace.Version
			}
		}
		modules[mod.Path+"@"+mod.Version] = mod
	}
}

// directoryNotices keeps copyright, license, notice and patent files together.
func directoryNotices(dir string) (string, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var parts []string
	for _, file := range files {
		if file.IsDir() || !isNotice(file.Name()) {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, file.Name()))
		if err != nil {
			return "", err
		}
		parts = append(parts, file.Name()+"\n\n"+strings.TrimSpace(string(data)))
	}
	if len(parts) == 0 {
		return "", fmt.Errorf("no license files in %s", dir)
	}
	return strings.Join(parts, "\n\n"), nil
}

func isNotice(name string) bool {
	name, _, _ = strings.Cut(strings.ToUpper(name), ".")
	for _, stem := range []string{"LICENSE", "LICENCE", "COPYING", "COPYRIGHT", "NOTICE", "PATENTS"} {
		if name == stem || strings.HasPrefix(name, stem+"-") || strings.HasPrefix(name, stem+"_") {
			return true
		}
	}
	return false
}
