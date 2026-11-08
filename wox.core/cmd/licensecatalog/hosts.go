package main

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/mail"
	"os"
	"path"
	"path/filepath"
	"strings"

	"wox/resource/licenses"
)

// nodeLicenses uses the report from the bundler, which sees the resolved dependency graph.
func nodeLicenses(release bool) ([]licenses.Entry, error) {
	if !release && !fileExists(filepath.Join(hosts, "node-host.cjs")) {
		return nil, nil
	}
	body, err := os.ReadFile(".build/licenses/node-host.txt")
	if !release && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Node.js host licenses: %w", err)
	}
	return []licenses.Entry{{Name: "Wox Node.js Host", Text: string(body)}}, nil
}

// pythonLicenses reads the exact installed wheel metadata from the embedded zipapp.
func pythonLicenses(release bool) ([]licenses.Entry, error) {
	filename := filepath.Join(hosts, "python-host.pyz")
	bundle, err := zip.OpenReader(filename)
	if !release && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer bundle.Close()

	environment, err := fs.ReadFile(&bundle.Reader, "environment.json")
	if err != nil {
		return nil, err
	}
	var bootstrap struct {
		Version string `json:"shiv_version"`
	}
	if err := json.Unmarshal(environment, &bootstrap); err != nil {
		return nil, err
	}
	shiv, err := upstreamNotice("shiv", bootstrap.Version)
	if err != nil {
		return nil, err
	}
	entries := []licenses.Entry{shiv}
	for _, file := range bundle.File {
		if !strings.HasSuffix(file.Name, ".dist-info/METADATA") {
			continue
		}
		entry, err := pythonWheelLicense(&bundle.Reader, file.Name)
		if err != nil {
			return nil, err
		}
		if entry.Name != "" {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// pythonWheelLicense reads Core Metadata headers, then includes the wheel's license files.
func pythonWheelLicense(bundle *zip.Reader, filename string) (licenses.Entry, error) {
	reader, err := bundle.Open(filename)
	if err != nil {
		return licenses.Entry{}, err
	}
	defer reader.Close()
	metadata, err := mail.ReadMessage(reader)
	if err != nil {
		return licenses.Entry{}, err
	}
	name, version := metadata.Header.Get("Name"), metadata.Header.Get("Version")
	switch strings.ReplaceAll(name, "_", "-") {
	case "wox-plugin", "wox-plugin-host":
		return licenses.Entry{}, nil
	}
	root := path.Dir(filename) + "/"
	var parts []string
	for _, file := range bundle.File {
		relative, ok := strings.CutPrefix(file.Name, root)
		if !ok || file.FileInfo().IsDir() || (!strings.HasPrefix(relative, "licenses/") && !isNotice(path.Base(relative))) {
			continue
		}
		body, err := fs.ReadFile(bundle, file.Name)
		if err != nil {
			return licenses.Entry{}, err
		}
		parts = append(parts, relative+"\n\n"+strings.TrimSpace(string(body)))
	}
	if len(parts) == 0 {
		return upstreamNotice(name, version)
	}
	return licenses.Entry{Name: name, Version: version, Text: strings.Join(parts, "\n\n")}, nil
}

func raycastLicenses(release bool) ([]licenses.Entry, error) {
	if !release && !fileExists(filepath.Join(hosts, "raycast-host.js")) {
		return nil, nil
	}
	data, err := os.ReadFile(".build/licenses/raycast-host.json")
	if !release && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("Raycast host licenses: %w", err)
	}
	var entries []licenses.Entry
	err = json.Unmarshal(data, &entries)
	return entries, err
}

// flowLicenses follows the packaged .NET host's resolved NuGet library list.
func flowLicenses(release bool) ([]licenses.Entry, error) {
	if !isWindows {
		return nil, nil
	}
	data, err := os.ReadFile(filepath.Join(hosts, "flow", "Wox.Flow.DotNetHost.deps.json"))
	if !release && errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var deps struct {
		Libraries map[string]struct{ Type string }
	}
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, err
	}
	var entries []licenses.Entry
	for id, library := range deps.Libraries {
		if library.Type != "package" {
			continue
		}
		name, version, _ := strings.Cut(id, "/")
		entry, err := upstreamNotice(name, version)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// upstreamNotice covers dependencies whose distributable artifacts omit their license files.
func upstreamNotice(name, version string) (licenses.Entry, error) {
	body, err := upstream.ReadFile("notices/" + name + "-" + version + ".txt")
	if err != nil {
		return licenses.Entry{}, fmt.Errorf("missing upstream license for %s %s: %w", name, version, err)
	}
	return licenses.Entry{Name: name, Version: version, Text: string(body)}, nil
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
