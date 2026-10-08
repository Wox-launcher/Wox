package script

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	flowDialectV1 = "v1"
	flowDialectV2 = "v2"
)

// detectFlowDialect chooses the process protocol.
//
// Python and Node plugins stay running and speak JSON-RPC 2.0. The host client
// is placed on PYTHONPATH ahead of paths a plugin appends, which is the same
// import order a long-lived host uses. A plugin that reads the request from
// argv, or that ships its own argv client directly in the plugin root, is
// started once per call instead. A client under lib/ does not force that path,
// because an appended lib directory loses to PYTHONPATH.
func detectFlowDialect(directory, language, entry string) string {
	switch strings.ToLower(language) {
	case "executable":
		return flowDialectV1
	case "python":
		if flowFileUses(filepath.Join(directory, filepath.FromSlash(entry)), "sys.argv") {
			return flowDialectV1
		}
		if flowTreeUsesArgv(filepath.Join(directory, "flowlauncher")) || flowFileUses(filepath.Join(directory, "flowlauncher.py"), "sys.argv") {
			return flowDialectV1
		}
		return flowDialectV2
	case "javascript", "typescript":
		text := flowReadSnippet(filepath.Join(directory, filepath.FromSlash(entry)))
		if strings.Contains(text, "process.argv") && !strings.Contains(text, "process.stdin") && !strings.Contains(text, "readline") {
			return flowDialectV1
		}
		return flowDialectV2
	default:
		return flowDialectV1
	}
}

func flowTreeUsesArgv(root string) bool {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return false
	}
	found := false
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || found {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(path), ".py") && flowFileUses(path, "sys.argv") {
			found = true
		}
		return nil
	})
	return found
}

func flowFileUses(path, needle string) bool {
	return strings.Contains(flowReadSnippet(path), needle)
}

func flowReadSnippet(path string) string {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > 512*1024 {
		return ""
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(raw)
}
